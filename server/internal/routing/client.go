// Package routing proxies /api/route*, /api/matrix, /api/isochrone
// requests to the Valhalla HTTP API documented at
// https://valhalla.github.io/valhalla/api/, and shapes the results into
// the LocalMaps contract defined in contracts/openapi.yaml.
//
// The package is intentionally side-effect-free: handlers construct a
// Client at router-boot time from Boot.ValhallaURL and call it
// per-request.
package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LatLon mirrors the OpenAPI LatLon schema. Kept local to avoid a
// server/internal/api ↔ routing import cycle.
type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// RouteMode is the LocalMaps-facing costing mode.
type RouteMode string

const (
	ModeAuto       RouteMode = "auto"
	ModeBicycle    RouteMode = "bicycle"
	ModePedestrian RouteMode = "pedestrian"
	ModeTruck      RouteMode = "truck"
)

// ValidMode reports whether m is one of the four RouteMode values the
// OpenAPI contract allows.
func ValidMode(m string) bool {
	switch RouteMode(m) {
	case ModeAuto, ModeBicycle, ModePedestrian, ModeTruck:
		return true
	}
	return false
}

// --- LocalMaps request/response ------------------------------------

// RouteRequest mirrors the RouteRequest schema. JSON tags align with
// the Fiber body the UI posts.
type RouteRequest struct {
	Locations     []LatLon   `json:"locations"`
	Mode          RouteMode  `json:"mode"`
	AvoidHighways bool       `json:"avoidHighways,omitempty"`
	AvoidTolls    bool       `json:"avoidTolls,omitempty"`
	AvoidFerries  bool       `json:"avoidFerries,omitempty"`
	Alternatives  int        `json:"alternatives,omitempty"`
	Units         string     `json:"units,omitempty"`
	Language      *string    `json:"language,omitempty"`
	Truck         *TruckOpts `json:"truck,omitempty"`
}

// TruckOpts matches the nested `truck` object in the RouteRequest schema.
type TruckOpts struct {
	HeightMeters float64 `json:"heightMeters,omitempty"`
	WidthMeters  float64 `json:"widthMeters,omitempty"`
	WeightTons   float64 `json:"weightTons,omitempty"`
	LengthMeters float64 `json:"lengthMeters,omitempty"`
}

// RouteLegSummary mirrors the nested "summary" in RouteLeg.
type RouteLegSummary struct {
	TimeSeconds    float64 `json:"timeSeconds"`
	DistanceMeters float64 `json:"distanceMeters"`
}

// Maneuver mirrors one entry of RouteLeg.maneuvers.
type Maneuver struct {
	Instruction     string  `json:"instruction"`
	BeginShapeIndex int     `json:"beginShapeIndex"`
	EndShapeIndex   int     `json:"endShapeIndex,omitempty"`
	DistanceMeters  float64 `json:"distanceMeters,omitempty"`
	TimeSeconds     float64 `json:"timeSeconds,omitempty"`
	Type            string  `json:"type,omitempty"`
	StreetName      *string `json:"streetName,omitempty"`
}

// RouteLeg mirrors the RouteLeg OpenAPI schema.
type RouteLeg struct {
	Summary   RouteLegSummary `json:"summary"`
	Maneuvers []Maneuver      `json:"maneuvers"`
	Geometry  struct {
		Polyline string `json:"polyline"`
	} `json:"geometry"`
}

// Route mirrors the Route OpenAPI schema.
type Route struct {
	ID        string          `json:"id"`
	Summary   RouteLegSummary `json:"summary"`
	Legs      []RouteLeg      `json:"legs"`
	Waypoints []LatLon        `json:"waypoints,omitempty"`
	Mode      RouteMode       `json:"mode,omitempty"`
}

// RouteResponse mirrors the RouteResponse OpenAPI schema.
type RouteResponse struct {
	Routes  []Route `json:"routes"`
	TraceID string  `json:"traceId"`
}

// IsochroneRequest mirrors the IsochroneRequest OpenAPI schema.
type IsochroneRequest struct {
	Origin          LatLon    `json:"origin"`
	Mode            RouteMode `json:"mode,omitempty"`
	ContoursSeconds []int     `json:"contoursSeconds"`
}

// MatrixRequest mirrors the MatrixRequest OpenAPI schema.
type MatrixRequest struct {
	Sources []LatLon  `json:"sources"`
	Targets []LatLon  `json:"targets"`
	Mode    RouteMode `json:"mode"`
}

// MatrixCell is one entry in a MatrixResponse row.
type MatrixCell struct {
	TimeSeconds    *float64 `json:"timeSeconds,omitempty"`
	DistanceMeters *float64 `json:"distanceMeters,omitempty"`
}

// MatrixResponse mirrors the MatrixResponse OpenAPI schema.
type MatrixResponse struct {
	Matrix  [][]MatrixCell `json:"matrix"`
	TraceID string         `json:"traceId"`
}

// --- Valhalla wire types (subset we consume) -----------------------

// vLocation is the `locations[]` element Valhalla expects for /route
// and /isochrone; {lat,lon} is the minimal form.
//
// `radius` (optional, meters) widens Valhalla's edge-snap search around
// the point. Default upstream is ~50m which is far too tight for
// click-on-map UX — clicking just off a road regularly produces
// `error_code 171: No suitable edges near location`. We bump it to
// 500m on every /route location below; users can still target a
// specific edge by clicking precisely.
//
// `search_filter.min_road_class` further constrains which edges the
// snap is allowed to consider. We don't need anything fancier than
// `service` (which still excludes pedestrian / footway), so even with
// the wider radius the snap will pick a real road rather than a
// random alley.
type vLocation struct {
	Lat           float64                `json:"lat"`
	Lon           float64                `json:"lon"`
	Radius        int                    `json:"radius,omitempty"`
	SearchFilter  map[string]interface{} `json:"search_filter,omitempty"`
}

// defaultSnapRadiusMeters is the fallback search radius applied to
// every /route location. Valhalla's docs cap this at ~200m by default;
// 500m is generous enough to absorb the small click-offset users hit
// in practice (snap-to-road from a touch event) without crossing into
// the next street.
const defaultSnapRadiusMeters = 500

// defaultSearchFilter is the snap-time filter we apply to every
// location. min_road_class=service excludes service alleys' children
// (footway/path/pedestrian) without ruling out actual road network.
// Values mirror the documented Valhalla road-class enum at
// https://valhalla.github.io/valhalla/api/turn-by-turn/api-reference/#locations.
var defaultSearchFilter = map[string]interface{}{
	"min_road_class": "service",
}

// vRouteRequest is the Valhalla /route body.
type vRouteRequest struct {
	Locations         []vLocation            `json:"locations"`
	Costing           string                 `json:"costing"`
	CostingOptions    map[string]interface{} `json:"costing_options,omitempty"`
	DirectionsOptions map[string]interface{} `json:"directions_options,omitempty"`
	Alternates        int                    `json:"alternates,omitempty"`
	Language          string                 `json:"language,omitempty"`
}

// vTripSummary matches `trip.summary` / `trip.legs[].summary`.
type vTripSummary struct {
	Time   float64 `json:"time"`
	Length float64 `json:"length"`
}

// vManeuver is one entry of `trip.legs[].maneuvers[]`.
type vManeuver struct {
	Instruction     string  `json:"instruction"`
	BeginShapeIndex int     `json:"begin_shape_index"`
	EndShapeIndex   int     `json:"end_shape_index"`
	Length          float64 `json:"length"`
	Time            float64 `json:"time"`
	Type            int     `json:"type"`
	StreetNames     []string `json:"street_names"`
}

// vLeg is one entry of `trip.legs[]`.
type vLeg struct {
	Maneuvers []vManeuver  `json:"maneuvers"`
	Summary   vTripSummary `json:"summary"`
	Shape     string       `json:"shape"`
}

// vTrip is `trip` inside /route responses.
type vTrip struct {
	Locations []vLocation  `json:"locations"`
	Legs      []vLeg       `json:"legs"`
	Summary   vTripSummary `json:"summary"`
	Units     string       `json:"units"`
	Status    int          `json:"status"`
}

// vRouteResponse is the subset of /route's body we read. Valhalla
// returns either {trip:…} or {trip:…, alternates:[{trip:…}…]}.
type vRouteResponse struct {
	Trip       vTrip `json:"trip"`
	Alternates []struct {
		Trip vTrip `json:"trip"`
	} `json:"alternates"`
}

// vMatrixResponse is the /sources_to_targets body subset.
type vMatrixResponse struct {
	SourcesToTargets [][]struct {
		Distance *float64 `json:"distance"`
		Time     *float64 `json:"time"`
	} `json:"sources_to_targets"`
	Units string `json:"units"`
}

// --- Client --------------------------------------------------------

// Client talks HTTP to a Valhalla server and owns the route-id cache
// used by the GPX/KML exporters.
type Client struct {
	baseURL string
	http    *http.Client
	// Timeouts may vary by endpoint — /isochrone is heavier than /route.
	routeTimeout     time.Duration
	isochroneTimeout time.Duration
	matrixTimeout    time.Duration
	cache            *routeCache
}

// NewClient builds a Client pointing at baseURL (trailing slash stripped).
// Pass an empty string to fall back to http://valhalla:8002.
func NewClient(baseURL string) *Client {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if u == "" {
		u = "http://valhalla:8002"
	}
	return &Client{
		baseURL:          u,
		http:             &http.Client{Timeout: 60 * time.Second},
		routeTimeout:     30 * time.Second,
		isochroneTimeout: 60 * time.Second,
		matrixTimeout:    30 * time.Second,
		cache:            newRouteCache(256),
	}
}

// BaseURL returns the normalized Valhalla URL (useful for tests + logs).
func (c *Client) BaseURL() string { return c.baseURL }

// LookupRoute returns the cached shape / metadata for id, if any.
func (c *Client) LookupRoute(id string) (CachedRoute, bool) {
	return c.cache.Get(id)
}

// Errors returned by the Client are wrapped so handlers can
// distinguish bad-request (4xx) from upstream-unavailable (5xx /
// network) conditions.
var (
	// ErrUpstreamUnavailable is returned for network errors, timeouts,
	// or non-2xx/non-4xx responses from Valhalla.
	ErrUpstreamUnavailable = errors.New("valhalla upstream unavailable")
	// ErrUpstreamBadRequest is returned when Valhalla rejects our body
	// (4xx). Usually means the request is malformed (out of coverage,
	// invalid locations, …).
	ErrUpstreamBadRequest = errors.New("valhalla rejected the request")
)

// Route translates req into the Valhalla /route payload, posts it, and
// maps the result to RouteResponse. The caller gets back an opaque
// id that can be exchanged for GPX/KML via LookupRoute.
func (c *Client) Route(ctx context.Context, req RouteRequest, traceID string) (*RouteResponse, error) {
	if len(req.Locations) < 2 {
		return nil, fmt.Errorf("%w: need at least 2 locations", ErrUpstreamBadRequest)
	}
	if !ValidMode(string(req.Mode)) {
		return nil, fmt.Errorf("%w: invalid mode %q", ErrUpstreamBadRequest, req.Mode)
	}

	units := "kilometers"
	if req.Units == "imperial" {
		units = "miles"
	}

	vreq := vRouteRequest{
		Locations:         make([]vLocation, len(req.Locations)),
		Costing:           string(req.Mode),
		DirectionsOptions: map[string]interface{}{"units": units},
		Alternates:        req.Alternatives,
	}
	for i, l := range req.Locations {
		vreq.Locations[i] = vLocation{
			Lat:          l.Lat,
			Lon:          l.Lon,
			Radius:       defaultSnapRadiusMeters,
			SearchFilter: defaultSearchFilter,
		}
	}
	if req.Language != nil && *req.Language != "" {
		vreq.Language = *req.Language
	}
	if opts := buildCostingOptions(req); opts != nil {
		vreq.CostingOptions = map[string]interface{}{string(req.Mode): opts}
	}

	var vresp vRouteResponse
	if err := c.postJSON(ctx, "/route", c.routeTimeout, vreq, &vresp); err != nil {
		return nil, err
	}

	trips := append([]vTrip{vresp.Trip}, func() []vTrip {
		out := make([]vTrip, 0, len(vresp.Alternates))
		for _, a := range vresp.Alternates {
			out = append(out, a.Trip)
		}
		return out
	}()...)

	out := &RouteResponse{TraceID: traceID}
	for _, t := range trips {
		if len(t.Legs) == 0 {
			continue
		}
		id := uuid.NewString()
		r := convertTrip(id, req.Mode, req.Locations, t)
		out.Routes = append(out.Routes, r)

		// Cache the combined shape for GPX/KML export.
		var shape []LatLon
		for _, leg := range t.Legs {
			if pts, ok := DecodePolyline6(leg.Shape); ok {
				shape = append(shape, pts...)
			}
		}
		c.cache.Put(id, CachedRoute{
			ID:             id,
			Mode:           string(req.Mode),
			Shape:          shape,
			TimeSeconds:    t.Summary.Time,
			DistanceMeters: summaryMeters(t.Summary.Length, units),
		})
	}
	if len(out.Routes) == 0 {
		return nil, fmt.Errorf("%w: valhalla returned no route", ErrUpstreamUnavailable)
	}
	return out, nil
}

// Isochrone proxies req to Valhalla's /isochrone endpoint and returns
// the raw GeoJSON FeatureCollection body. Valhalla already returns
// GeoJSON shaped to our contract, so we forward it verbatim.
func (c *Client) Isochrone(ctx context.Context, req IsochroneRequest) (json.RawMessage, error) {
	if len(req.ContoursSeconds) == 0 {
		return nil, fmt.Errorf("%w: contoursSeconds must not be empty", ErrUpstreamBadRequest)
	}
	mode := string(req.Mode)
	if mode == "" {
		mode = string(ModePedestrian)
	}
	if !ValidMode(mode) {
		return nil, fmt.Errorf("%w: invalid mode %q", ErrUpstreamBadRequest, mode)
	}
	contours := make([]map[string]interface{}, 0, len(req.ContoursSeconds))
	for _, s := range req.ContoursSeconds {
		// Valhalla's /isochrone takes `time` in minutes; we accept seconds
		// from our UI so convert (round down, min 1 to avoid 0-minute request).
		m := s / 60
		if m < 1 {
			m = 1
		}
		contours = append(contours, map[string]interface{}{"time": m})
	}
	body := map[string]interface{}{
		"locations":    []vLocation{{Lat: req.Origin.Lat, Lon: req.Origin.Lon}},
		"costing":      mode,
		"contours":     contours,
		"polygons":     true,
		"denoise":      0.5,
		"generalize":   150,
		"show_locations": false,
	}
	raw, err := c.postRaw(ctx, "/isochrone", c.isochroneTimeout, body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// Matrix proxies req to Valhalla's /sources_to_targets endpoint and
// maps the cells back to our MatrixResponse.
func (c *Client) Matrix(ctx context.Context, req MatrixRequest, traceID string) (*MatrixResponse, error) {
	if len(req.Sources) == 0 || len(req.Targets) == 0 {
		return nil, fmt.Errorf("%w: sources and targets must not be empty", ErrUpstreamBadRequest)
	}
	if !ValidMode(string(req.Mode)) {
		return nil, fmt.Errorf("%w: invalid mode %q", ErrUpstreamBadRequest, req.Mode)
	}
	sources := make([]vLocation, len(req.Sources))
	for i, s := range req.Sources {
		sources[i] = vLocation{Lat: s.Lat, Lon: s.Lon}
	}
	targets := make([]vLocation, len(req.Targets))
	for i, t := range req.Targets {
		targets[i] = vLocation{Lat: t.Lat, Lon: t.Lon}
	}
	body := map[string]interface{}{
		"sources": sources,
		"targets": targets,
		"costing": string(req.Mode),
		"units":   "kilometers",
	}
	var vresp vMatrixResponse
	if err := c.postJSON(ctx, "/sources_to_targets", c.matrixTimeout, body, &vresp); err != nil {
		return nil, err
	}
	out := &MatrixResponse{TraceID: traceID}
	out.Matrix = make([][]MatrixCell, len(vresp.SourcesToTargets))
	kmFactor := 1000.0
	if strings.EqualFold(vresp.Units, "miles") {
		kmFactor = 1609.344
	}
	for i, row := range vresp.SourcesToTargets {
		cells := make([]MatrixCell, len(row))
		for j, cell := range row {
			c := MatrixCell{}
			if cell.Time != nil {
				t := *cell.Time
				c.TimeSeconds = &t
			}
			if cell.Distance != nil {
				d := *cell.Distance * kmFactor
				c.DistanceMeters = &d
			}
			cells[j] = c
		}
		out.Matrix[i] = cells
	}
	return out, nil
}

// --- helpers -------------------------------------------------------

// convertTrip maps a Valhalla trip → LocalMaps Route.
func convertTrip(id string, mode RouteMode, waypoints []LatLon, t vTrip) Route {
	units := t.Units
	legs := make([]RouteLeg, 0, len(t.Legs))
	for _, leg := range t.Legs {
		out := RouteLeg{
			Summary: RouteLegSummary{
				TimeSeconds:    leg.Summary.Time,
				DistanceMeters: summaryMeters(leg.Summary.Length, units),
			},
			Maneuvers: make([]Maneuver, 0, len(leg.Maneuvers)),
		}
		out.Geometry.Polyline = leg.Shape
		for _, m := range leg.Maneuvers {
			mn := Maneuver{
				Instruction:     m.Instruction,
				BeginShapeIndex: m.BeginShapeIndex,
				EndShapeIndex:   m.EndShapeIndex,
				DistanceMeters:  summaryMeters(m.Length, units),
				TimeSeconds:     m.Time,
				Type:            maneuverType(m.Type),
			}
			if len(m.StreetNames) > 0 {
				s := m.StreetNames[0]
				mn.StreetName = &s
			}
			out.Maneuvers = append(out.Maneuvers, mn)
		}
		legs = append(legs, out)
	}
	return Route{
		ID: id,
		Summary: RouteLegSummary{
			TimeSeconds:    t.Summary.Time,
			DistanceMeters: summaryMeters(t.Summary.Length, units),
		},
		Legs:      legs,
		Waypoints: waypoints,
		Mode:      mode,
	}
}

// summaryMeters converts Valhalla's `length` (km or miles) to meters.
func summaryMeters(length float64, units string) float64 {
	if strings.EqualFold(units, "miles") {
		return length * 1609.344
	}
	return length * 1000.0
}

// maneuverType maps Valhalla's integer maneuver enum to a short string
// label. The full enum has ~40 entries; we only surface the common ones
// so the UI can key off a stable string (everything else becomes "").
func maneuverType(t int) string {
	switch t {
	case 1, 2:
		return "start"
	case 4, 5, 6:
		return "destination"
	case 8:
		return "continue"
	case 10, 11, 12, 13, 14, 15:
		return "turn"
	case 16, 17:
		return "uturn"
	case 21, 22, 23:
		return "merge"
	case 24, 25, 26, 27, 28, 29:
		return "roundabout"
	case 34, 35, 36, 37:
		return "ramp"
	default:
		return ""
	}
}

// buildCostingOptions turns the avoid* + truck fields into the
// `costing_options.<mode>` object Valhalla understands.
func buildCostingOptions(r RouteRequest) map[string]interface{} {
	opts := map[string]interface{}{}
	if r.AvoidHighways {
		opts["use_highways"] = 0.0
	}
	if r.AvoidTolls {
		opts["use_tolls"] = 0.0
	}
	if r.AvoidFerries {
		opts["use_ferry"] = 0.0
	}
	if r.Truck != nil && r.Mode == ModeTruck {
		if r.Truck.HeightMeters > 0 {
			opts["height"] = r.Truck.HeightMeters
		}
		if r.Truck.WidthMeters > 0 {
			opts["width"] = r.Truck.WidthMeters
		}
		if r.Truck.WeightTons > 0 {
			opts["weight"] = r.Truck.WeightTons
		}
		if r.Truck.LengthMeters > 0 {
			opts["length"] = r.Truck.LengthMeters
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

// postJSON marshals body, posts to baseURL+path, and decodes the response
// into out. It classifies errors as ErrUpstreamBadRequest (4xx) or
// ErrUpstreamUnavailable (anything else, including timeouts).
func (c *Client) postJSON(ctx context.Context, path string, timeout time.Duration, body, out interface{}) error {
	raw, err := c.postRaw(ctx, path, timeout, body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decoding valhalla response: %v", ErrUpstreamUnavailable, err)
	}
	return nil
}

// postRaw returns the raw response body. Used by the isochrone handler
// which forwards the GeoJSON verbatim.
func (c *Client) postRaw(ctx context.Context, path string, timeout time.Duration, body interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%w: encoding valhalla request: %v", ErrUpstreamUnavailable, err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: building request: %v", ErrUpstreamUnavailable, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading response: %v", ErrUpstreamUnavailable, err)
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrUpstreamBadRequest, resp.StatusCode, truncate(string(respBody), 200))
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrUpstreamUnavailable, resp.StatusCode, truncate(string(respBody), 200))
	}
	return respBody, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
