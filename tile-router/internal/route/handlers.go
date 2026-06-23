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

// Handlers serve tiles + tilejson. Construct one and attach to a
// net/http mux.
type Handlers struct {
	Store       Snapshotter
	Attribution string  // shown in the map UI's bottom-right; e.g. "© OpenStreetMap contributors"
	Log         zerolog.Logger
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

	regions, loaded := h.Store.Snapshot()
	if len(regions) == 0 {
		// No installed regions at all — empty 404 is the right answer.
		// The map UI will show a blank canvas; the operator should
		// install a region.
		http.NotFound(w, r)
		return
	}
	picked, ok := pick.Pick(regions, int(z), int(x), int(y))
	if !ok {
		// Tile is over ocean or outside any installed bbox. Standard
		// slippy-map behaviour: 404; the client just doesn't render.
		http.NotFound(w, r)
		return
	}
	l, ok := loaded[picked.Name]
	if !ok {
		// Shouldn't happen — picker only sees regions present in
		// loaded — but defend the contract anyway.
		h.Log.Warn().Str("region", picked.Name).Msg("picker returned missing region")
		http.NotFound(w, r)
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
		// In-region but no data for this tile (sparse). 204 vs 404
		// is a holy war; we pick 404 because the protomaps client
		// library treats both as "empty" and 404 keeps Cloudflare /
		// nginx access logs simpler.
		http.NotFound(w, r)
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
