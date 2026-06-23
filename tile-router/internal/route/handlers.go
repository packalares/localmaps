// Package route implements the tile-router HTTP surface. Two
// endpoints:
//
//   GET /tile/{z}/{x}/{y}.{ext}   — pick region by bbox, stream tile
//   GET /maps.json                 — aggregated TileJSON for the UI
//
// The router is deliberately tiny — no auth, no middleware. The
// upstream gateway already does both. We just stream bytes.
package route

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/tile-router/internal/pick"
	"github.com/packalares/localmaps/tile-router/internal/store"
)

// Snapshotter is the contract Handlers needs from the regions store.
// Inverting the dependency here so tests can pass a fake snapshot
// without spinning up a sqlite.
type Snapshotter interface {
	Snapshot() ([]pick.Region, map[string]*store.Loaded)
}

// basemapMaxZoom mirrors basemap.MaxZoom. Duplicated as a const here
// instead of imported to keep the route → basemap import direction
// one-way (the basemap package depends on pick; pick stays leaf;
// route depends on both via small adapters). If basemap.MaxZoom ever
// changes, bump both.
const basemapMaxZoom uint8 = 5

// Handlers serve tiles + tilejson. Construct one and attach to a
// net/http mux.
//
// `Basemap` is optional. When non-nil, low-zoom requests (z <=
// basemap.MaxZoom) that don't resolve to an installed pmtiles region
// fall through to a synthesised world-overview tile rendered from
// the embedded Natural Earth polygons — fixes the "half of Romania
// is missing at z=4" problem where one tile spans several countries
// and per-region picking can only cover part of it.
type Handlers struct {
	Store       Snapshotter
	Basemap     BasemapRenderer
	Attribution string // shown in the map UI's bottom-right; e.g. "© OpenStreetMap contributors"
	Log         zerolog.Logger
}

// BasemapRenderer is the contract Handlers needs from the basemap
// package. Inverting the dependency keeps the route layer testable
// with a fake renderer (and keeps the import graph one-way).
type BasemapRenderer interface {
	Render(z uint8, x, y uint32) ([]byte, error)
	IsEmpty() bool
}

// Register attaches the handler set to the given ServeMux. Routes:
//
//   /tile/{z}/{x}/{y}        — vector tile (.pbf or .mvt extension stripped)
//   /maps.json              — aggregated TileJSON
//   /healthz                 — kubelet probe target
//
// Net/http's path patterns can't carry a literal suffix after a
// wildcard ({y}.pbf would mean "match y as a segment then .pbf as
// extra"), so we accept ANY extension on the last segment and strip
// it in the handler. Clients typically request .pbf for vector mvt.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /tile/{z}/{x}/{y}", h.serveTile)
	mux.HandleFunc("GET /maps.json", h.serveTileJSON)
	mux.HandleFunc("GET /healthz", h.serveHealthz)
}

// serveTile streams the tile bytes for the requested z/x/y from the
// region whose bbox best covers it. Cache headers are set so the
// browser keeps the bytes; tiles for an installed region don't
// change between builds within a given pmtiles file.
func (h *Handlers) serveTile(w http.ResponseWriter, r *http.Request) {
	z, err := parseUint(r.PathValue("z"))
	if err != nil || z > 30 {
		http.Error(w, "bad z", http.StatusBadRequest)
		return
	}
	x, err := parseUint(r.PathValue("x"))
	if err != nil {
		http.Error(w, "bad x", http.StatusBadRequest)
		return
	}
	// y comes in with an optional extension (.pbf / .mvt) — strip the
	// last dot-suffix before parsing as int.
	yRaw := r.PathValue("y")
	if dot := strings.LastIndexByte(yRaw, '.'); dot >= 0 {
		yRaw = yRaw[:dot]
	}
	y, err := parseUint(yRaw)
	if err != nil {
		http.Error(w, "bad y", http.StatusBadRequest)
		return
	}

	// notFound is a wrapper that returns 404 with explicit
	// no-cache headers. Without this the gateway's default
	// `max-age=300` Cache-Control bleeds onto 404 responses too,
	// and browsers cache them for 5 minutes — which means any
	// picker-side mistake (or staged regions still building) gets
	// frozen into every connected client's cache for the duration.
	// Tile coverage changes day-to-day; 404s should NEVER cache.
	notFound := func() {
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		http.NotFound(w, r)
	}

	// serveBasemap renders the embedded world-overview MVT and
	// writes it to the response. Used as a fallback at low zoom
	// for any (z, x, y) where the per-region picker either fails
	// to match a region OR matches a region whose pmtiles is
	// empty for that tile.
	serveBasemap := func() bool {
		if h.Basemap == nil || h.Basemap.IsEmpty() {
			return false
		}
		if z > uint64(basemapMaxZoom) {
			return false
		}
		body, err := h.Basemap.Render(uint8(z), uint32(x), uint32(y))
		if err != nil {
			h.Log.Warn().Err(err).Int("z", int(z)).Int("x", int(x)).Int("y", int(y)).
				Msg("basemap render error")
			return false
		}
		if len(body) == 0 {
			return false
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Header().Set("Content-Encoding", "gzip") // basemap MarshalGzipped output
		// Basemap content is static for the life of the binary —
		// safe to cache aggressively. Browsers can hold a day
		// without missing updates because the rendered geometry
		// is baked at build time.
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Region", "basemap")
		if _, err := w.Write(body); err != nil {
			h.Log.Debug().Err(err).Msg("write basemap: client disconnected")
		}
		return true
	}

	// At low zoom (z <= basemapMaxZoom) one tile spans multiple
	// countries and per-region pmtiles only cover ONE country's
	// territory inside the tile. Serving the basemap directly here
	// — bypassing the per-region picker entirely — gives the user
	// a consistent world overview at world / continent zoom. The
	// country-scale pmtiles take back over at z = basemapMaxZoom +
	// 1, where each tile fits cleanly inside one country.
	if z <= uint64(basemapMaxZoom) && serveBasemap() {
		return
	}

	regions, loaded := h.Store.Snapshot()
	if len(regions) == 0 {
		// No installed regions at all. (Basemap was tried above for
		// low zoom; here at high zoom we have nothing to serve.)
		if serveBasemap() {
			return
		}
		notFound()
		return
	}
	picked, ok := pick.Pick(regions, int(z), int(x), int(y))
	if !ok {
		// Tile is over ocean or outside any installed region — try
		// the basemap fallback first so the world overview keeps
		// showing continent outlines even when the user pans away
		// from installed countries at low zoom.
		if serveBasemap() {
			return
		}
		notFound()
		return
	}
	l, ok := loaded[picked.Name]
	if !ok {
		// Shouldn't happen — picker only sees regions present in
		// loaded — but defend the contract anyway.
		h.Log.Warn().Str("region", picked.Name).Msg("picker returned missing region")
		notFound()
		return
	}

	tile, err := l.Reader.ReadTile(uint8(z), uint32(x), uint32(y))
	if err != nil {
		h.Log.Warn().Err(err).Str("region", picked.Name).
			Int("z", int(z)).Int("x", int(x)).Int("y", int(y)).
			Msg("ReadTile error")
		http.Error(w, "tile read failed", http.StatusInternalServerError)
		return
	}
	if len(tile) == 0 {
		// In-region but the picked pmtiles has no data for this
		// tile. At low zoom this is the "country pmtiles only
		// covers ITS own territory but the tile spans multiple
		// countries" case — fall back to the basemap so the user
		// sees neighbouring countries' outlines instead of a hole.
		if serveBasemap() {
			return
		}
		notFound()
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Content-Encoding", "gzip") // pmtiles stores tiles gzip-compressed; pass straight through
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Region", picked.Name) // useful in dev to see which file served what
	if _, err := w.Write(tile); err != nil {
		h.Log.Debug().Err(err).Msg("write tile: client probably disconnected")
	}
}

// serveTileJSON returns a TileJSON 3.0.0 document describing the
// union of installed regions. The map UI fetches this once on load
// and uses `bounds` to zoom-to-fit all installed countries on first
// render — that's the "see all four at once" UX.
func (h *Handlers) serveTileJSON(w http.ResponseWriter, r *http.Request) {
	regions, _ := h.Store.Snapshot()
	if len(regions) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tilejson":"3.0.0","tiles":[],"bounds":[-180,-85,180,85],"minzoom":0,"maxzoom":0}`))
		return
	}

	// Union bbox: outermost extent across all loaded regions. The
	// map UI uses this to set initial viewport so every country lands
	// on screen at first paint.
	u := unionBBox(regions)

	// Zoom range: take the WIDEST envelope across regions so the UI
	// allows the deepest zoom anyone's installed. Per-region max may
	// differ if some are continental and some country-scale, but
	// for OpenMapTiles country builds everyone caps at 14 in practice.
	minZ, maxZ := uint8(255), uint8(0)
	for _, reg := range regions {
		// We don't have the zoom on pick.Region (lean by design); pull
		// from store via tile URL would be circular. Just hardcode
		// the typical range for now; v2 can plumb it through.
		_ = reg
	}
	if minZ == 255 {
		minZ, maxZ = 0, 14 // OpenMapTiles country build defaults
	}

	// Build the tile URL template. Browsers substitute {z}/{x}/{y}.
	// We use a relative URL so the gateway's host/port doesn't have
	// to be threaded through here — the browser resolves against
	// whatever origin it loaded the page from.
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	scheme := "http"
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		scheme = "https"
	}

	type tileJSON struct {
		TileJSON    string    `json:"tilejson"`
		Tiles       []string  `json:"tiles"`
		Bounds      [4]float64 `json:"bounds"`
		Center      [3]float64 `json:"center"`
		MinZoom     uint8     `json:"minzoom"`
		MaxZoom     uint8     `json:"maxzoom"`
		Attribution string    `json:"attribution,omitempty"`
		Name        string    `json:"name"`
	}
	resp := tileJSON{
		TileJSON:    "3.0.0",
		Tiles:       []string{fmt.Sprintf("%s://%s/tile/{z}/{x}/{y}.pbf", scheme, host)},
		Bounds:      [4]float64{u.MinLon, u.MinLat, u.MaxLon, u.MaxLat},
		Center:      [3]float64{(u.MinLon + u.MaxLon) / 2, (u.MinLat + u.MaxLat) / 2, float64(minZ + 2)},
		MinZoom:     minZ,
		MaxZoom:     maxZ,
		Attribution: h.Attribution,
		Name:        fmt.Sprintf("LocalMaps (%d regions)", len(regions)),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60") // regions can change every 5s; keep this short
	_ = json.NewEncoder(w).Encode(resp)
}

// serveHealthz is the kubelet liveness probe target. We don't gate
// on the store having any regions — an empty router is still healthy,
// it just doesn't have anything to serve yet.
func (h *Handlers) serveHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// unionBBox computes the bbox enclosing all input regions. Skips
// invalid (zero-area) bboxes so a single malformed entry can't widen
// the union to the whole planet.
func unionBBox(regions []pick.Region) pick.BBox {
	first := true
	u := pick.BBox{}
	for _, r := range regions {
		if !r.BBox.IsValid() {
			continue
		}
		if first {
			u = r.BBox
			first = false
			continue
		}
		if r.BBox.MinLon < u.MinLon {
			u.MinLon = r.BBox.MinLon
		}
		if r.BBox.MinLat < u.MinLat {
			u.MinLat = r.BBox.MinLat
		}
		if r.BBox.MaxLon > u.MaxLon {
			u.MaxLon = r.BBox.MaxLon
		}
		if r.BBox.MaxLat > u.MaxLat {
			u.MaxLat = r.BBox.MaxLat
		}
	}
	if first {
		// No valid bboxes → return world.
		return pick.BBox{MinLon: -180, MinLat: -85, MaxLon: 180, MaxLat: 85}
	}
	return u
}

func parseUint(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseUint(s, 10, 32)
}
