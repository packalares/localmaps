package route

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/tile-router/internal/pick"
	"github.com/packalares/localmaps/tile-router/internal/pmtiles"
	"github.com/packalares/localmaps/tile-router/internal/store"
)

const realFile = "/tmp/romania.pmtiles"

// fakeStore implements Snapshotter without needing a sqlite. Useful
// to test handler logic in isolation from the regions package.
type fakeStore struct {
	regions []pick.Region
	loaded  map[string]*store.Loaded
}

func (f *fakeStore) Snapshot() ([]pick.Region, map[string]*store.Loaded) {
	return f.regions, f.loaded
}

func newRomaniaStore(t *testing.T) *fakeStore {
	t.Helper()
	if _, err := os.Stat(realFile); err != nil {
		t.Skipf("integration fixture missing (%s); skip", realFile)
	}
	tmp := t.TempDir()
	if err := os.Symlink(realFile, filepath.Join(tmp, "map.pmtiles")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	r, err := pmtiles.Open(filepath.Join(tmp, "map.pmtiles"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	minLon, minLat, maxLon, maxLat := r.BBox()
	reg := pick.Region{
		Name: "europe-romania",
		BBox: pick.BBox{MinLon: minLon, MinLat: minLat, MaxLon: maxLon, MaxLat: maxLat},
	}
	l := &store.Loaded{Region: reg, Reader: r}
	return &fakeStore{
		regions: []pick.Region{reg},
		loaded:  map[string]*store.Loaded{"europe-romania": l},
	}
}

func mux(h *Handlers) *http.ServeMux {
	m := http.NewServeMux()
	h.Register(m)
	return m
}

func TestServeTile_Bucharest(t *testing.T) {
	h := &Handlers{Store: newRomaniaStore(t), Log: zerolog.Nop()}
	w := httptest.NewRecorder()
	// Bucharest z=10 (from the pmtiles test): tile (586, 370).
	req := httptest.NewRequest(http.MethodGet, "/tile/10/586/370.pbf", nil)
	mux(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/x-protobuf" {
		t.Errorf("content-type: got %q", got)
	}
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("content-encoding: got %q", got)
	}
	if got := w.Header().Get("X-Region"); got != "europe-romania" {
		t.Errorf("x-region: got %q", got)
	}
	body := w.Body.Bytes()
	if len(body) == 0 {
		t.Fatal("response body empty")
	}
	if body[0] != 0x1f || body[1] != 0x8b {
		t.Errorf("body doesn't start with gzip magic: %x %x", body[0], body[1])
	}
}

func TestServeTile_OceanReturns404(t *testing.T) {
	h := &Handlers{Store: newRomaniaStore(t), Log: zerolog.Nop()}
	w := httptest.NewRecorder()
	// Random ocean tile — Atlantic at z=8.
	req := httptest.NewRequest(http.MethodGet, "/tile/8/70/100.pbf", nil)
	mux(h).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for ocean tile; got %d", w.Code)
	}
}

func TestServeTile_BadCoords(t *testing.T) {
	h := &Handlers{Store: newRomaniaStore(t), Log: zerolog.Nop()}
	cases := []string{
		"/tile/abc/0/0.pbf",
		"/tile/10/abc/0.pbf",
		"/tile/10/0/-1.pbf", // negative parses as bad uint
		"/tile/99/0/0.pbf",  // z out of range
	}
	for _, path := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		mux(h).ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("path %s: expected 400; got %d", path, w.Code)
		}
	}
}

func TestServeTile_EmptyStoreReturns404(t *testing.T) {
	empty := &fakeStore{regions: nil, loaded: map[string]*store.Loaded{}}
	h := &Handlers{Store: empty, Log: zerolog.Nop()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tile/10/586/370.pbf", nil)
	mux(h).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when no regions installed; got %d", w.Code)
	}
}

func TestServeTileJSON_RealRomania(t *testing.T) {
	h := &Handlers{
		Store:       newRomaniaStore(t),
		Attribution: "© OpenStreetMap contributors",
		Log:         zerolog.Nop(),
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/maps.json", nil)
	req.Host = "tile.example.test"
	mux(h).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", w.Code)
	}

	var tj struct {
		TileJSON    string     `json:"tilejson"`
		Tiles       []string   `json:"tiles"`
		Bounds      [4]float64 `json:"bounds"`
		MinZoom     uint8      `json:"minzoom"`
		MaxZoom     uint8      `json:"maxzoom"`
		Attribution string     `json:"attribution"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tj); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if tj.TileJSON != "3.0.0" {
		t.Errorf("tilejson version: got %q", tj.TileJSON)
	}
	if len(tj.Tiles) != 1 || !strings.Contains(tj.Tiles[0], "tile.example.test") {
		t.Errorf("tile URL template wrong: %v", tj.Tiles)
	}
	if !strings.Contains(tj.Tiles[0], "{z}/{x}/{y}") {
		t.Errorf("tile URL missing {z}/{x}/{y} placeholders: %s", tj.Tiles[0])
	}
	if tj.Bounds[0] < 18 || tj.Bounds[2] > 32 {
		t.Errorf("bounds suspicious for Romania-only: %+v", tj.Bounds)
	}
	if tj.MinZoom > 0 || tj.MaxZoom < 10 {
		t.Errorf("zoom range looks wrong: %d..%d", tj.MinZoom, tj.MaxZoom)
	}
	if tj.Attribution == "" {
		t.Errorf("attribution should be set")
	}
}

func TestServeTileJSON_NoRegionsReturnsEmpty(t *testing.T) {
	empty := &fakeStore{regions: nil, loaded: map[string]*store.Loaded{}}
	h := &Handlers{Store: empty, Log: zerolog.Nop()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/maps.json", nil)
	mux(h).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even with no regions; got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"tiles":[]`) {
		t.Errorf("empty store should serve empty tiles array; got %s", w.Body.String())
	}
}

func TestServeHealthz(t *testing.T) {
	empty := &fakeStore{regions: nil, loaded: map[string]*store.Loaded{}}
	h := &Handlers{Store: empty, Log: zerolog.Nop()}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	mux(h).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("healthz should return 200 even when empty; got %d", w.Code)
	}
}
