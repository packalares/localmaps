package geofabrik

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeSettings is a tiny in-memory SettingsReader for tests.
type fakeSettings struct {
	strings map[string]string
	ints    map[string]int
}

func (f *fakeSettings) GetString(key string) (string, error) {
	if v, ok := f.strings[key]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}
func (f *fakeSettings) GetInt(key string) (int, error) {
	if v, ok := f.ints[key]; ok {
		return v, nil
	}
	return 0, errors.New("not found")
}

// serveIndex returns an httptest.Server that serves the fixture at
// /index-v1.json plus a synthetic .md5 sidecar at the given path.
func serveIndex(t *testing.T, md5Path, md5Body string, hits *int32) *httptest.Server {
	t.Helper()
	fixture, err := os.ReadFile("testdata/index-v1.json")
	require.NoError(t, err)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index-v1.json" {
			if hits != nil {
				atomic.AddInt32(hits, 1)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
			return
		}
		if r.URL.Path == md5Path {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(md5Body))
			return
		}
		// Any other request: return a HEAD-friendly 200.
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "12345")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	dir := t.TempDir()
	c := NewClientWithBase(&http.Client{Timeout: 5 * time.Second}, baseURL, dir)
	return c
}

func TestListRegions_ParsesCatalog(t *testing.T) {
	srv := serveIndex(t, "", "", nil)
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	// Fixture uses real download.geofabrik.de pbf urls — override the
	// allow-list so ResolvePbfURL tests can pass.
	c.allowed = append(c.allowed, "download.geofabrik.de")

	tree, err := c.ListRegions(context.Background())
	require.NoError(t, err)
	require.Len(t, tree, 1, "expected 1 root (europe)")
	europe := tree[0]
	require.Equal(t, "europe", europe.Name)
	require.Equal(t, KindContinent, europe.Kind)
	require.Nil(t, europe.Parent)

	// Europe should have two countries.
	var germany, romania *CatalogEntry
	for i := range europe.Children {
		ch := europe.Children[i]
		switch ch.Name {
		case "europe-germany":
			germany = &europe.Children[i]
		case "europe-romania":
			romania = &europe.Children[i]
		}
	}
	require.NotNil(t, germany)
	require.NotNil(t, romania)
	require.Equal(t, KindCountry, germany.Kind)
	require.Equal(t, "europe", *germany.Parent)
	require.NotNil(t, germany.ISO31661)
	require.Equal(t, "DE", *germany.ISO31661)

	// Baden-Württemberg should be a subregion under Germany.
	require.Len(t, germany.Children, 1)
	bw := germany.Children[0]
	require.Equal(t, "europe-germany-baden-wuerttemberg", bw.Name)
	require.Equal(t, KindSubregion, bw.Kind)
	require.Equal(t, "europe-germany", *bw.Parent)
}

func TestListRegions_CachesOnDisk(t *testing.T) {
	var hits int32
	srv := serveIndex(t, "", "", &hits)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	_, err := c.ListRegions(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&hits))

	// Second call — served from in-memory cache.
	_, err = c.ListRegions(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&hits))

	// Simulate a fresh process by constructing a new client over the
	// same cache dir. Disk cache should be honoured.
	c2 := NewClientWithBase(&http.Client{Timeout: 5 * time.Second},
		srv.URL, c.cacheDir)
	_, err = c2.ListRegions(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, atomic.LoadInt32(&hits),
		"disk cache should have been used")
}

func TestListRegions_DiskCacheWritten(t *testing.T) {
	srv := serveIndex(t, "", "", nil)
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, err := c.ListRegions(context.Background())
	require.NoError(t, err)
	p := filepath.Join(c.cacheDir, "index-v1.json")
	_, err = os.Stat(p)
	require.NoError(t, err, "disk cache file should exist at %s", p)
}

func TestResolve_FoundAndNotFound(t *testing.T) {
	srv := serveIndex(t, "", "", nil)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	entry, err := c.Resolve(context.Background(), "europe-romania")
	require.NoError(t, err)
	require.Equal(t, "Romania", entry.DisplayName)

	_, err = c.Resolve(context.Background(), "mars-olympus")
	require.ErrorIs(t, err, ErrNotInCatalog)
}

func TestResolve_RejectsNonCanonical(t *testing.T) {
	srv := serveIndex(t, "", "", nil)
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, err := c.Resolve(context.Background(), "europe/romania")
	require.Error(t, err, "slashes are not canonical")
}

func TestResolvePbfURL_EgressAllowlist(t *testing.T) {
	srv := serveIndex(t, "", "", nil)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	// With only the test server on the allow list, the real Geofabrik
	// URL embedded in the fixture must be rejected.
	entry := CatalogEntry{
		Name:      "europe-romania",
		SourceURL: "https://download.geofabrik.de/europe/romania-latest.osm.pbf",
	}
	_, err := c.ResolvePbfURL(entry)
	require.ErrorIs(t, err, ErrEgressDenied)

	// After whitelisting the host, the same URL passes through.
	c.allowed = append(c.allowed, "download.geofabrik.de")
	url, err := c.ResolvePbfURL(entry)
	require.NoError(t, err)
	require.Equal(t, entry.SourceURL, url)
}

func TestResolvePbfURL_EmptySource(t *testing.T) {
	c := newTestClient(t, "http://example.invalid")
	_, err := c.ResolvePbfURL(CatalogEntry{Name: "x"})
	require.ErrorIs(t, err, ErrNoPbfURL)
}

func TestFetchSHA256_ReadsMd5Sidecar(t *testing.T) {
	var pbfHits int32
	mux := http.NewServeMux()
	mux.HandleFunc("/index-v1.json", func(w http.ResponseWriter, r *http.Request) {
		fixture, _ := os.ReadFile("testdata/index-v1.json")
		_, _ = w.Write(fixture)
	})
	mux.HandleFunc("/europe/romania-latest.osm.pbf.md5",
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("ABCDEF1234567890abcdef1234567890  europe/romania-latest.osm.pbf\n"))
		})
	mux.HandleFunc("/europe/romania-latest.osm.pbf",
		func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&pbfHits, 1)
			w.Header().Set("Content-Length", "55555")
			w.WriteHeader(http.StatusOK)
		})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	// Allow our httptest host.
	h := strings.TrimPrefix(srv.URL, "http://")
	c.allowed = append(c.allowed, strings.Split(h, ":")[0])

	md5, size, err := c.FetchSHA256(context.Background(),
		srv.URL+"/europe/romania-latest.osm.pbf")
	require.NoError(t, err)
	require.Equal(t, "abcdef1234567890abcdef1234567890", md5)
	require.EqualValues(t, 55555, size)
}

func TestNewClient_UsesSettings(t *testing.T) {
	fs := &fakeSettings{
		strings: map[string]string{
			"regions.mirrorBase": "https://download.geofabrik.de",
			"regions.catalogURL": "https://download.geofabrik.de/index-v1.json",
		},
	}
	c, err := NewClient(nil, fs, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, "https://download.geofabrik.de", c.BaseURL())
	require.Contains(t, c.allowed, "download.geofabrik.de")
}

func TestNewClient_RejectsEmptyMirrorBase(t *testing.T) {
	fs := &fakeSettings{strings: map[string]string{"regions.mirrorBase": ""}}
	_, err := NewClient(nil, fs, t.TempDir())
	require.Error(t, err)
}
