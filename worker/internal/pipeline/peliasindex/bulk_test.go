package peliasindex

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestBulkIndex_SendsNDJSONBody mounts a fake ES, sends two docs and
// verifies the ndjson wire format (alternating meta + source lines).
func TestBulkIndex_SendsNDJSONBody(t *testing.T) {
	var got bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/_bulk", r.URL.Path)
		require.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))
		_, _ = io.Copy(&got, r.Body)
		_, _ = w.Write([]byte(`{"took":1,"errors":false,"items":[]}`))
	}))
	defer srv.Close()

	opts := Options{}.defaulted()
	docs := []doc{
		docBuilder("node/1", "venue", "Cafe", 44.4, 26.1, "ro", []string{"amenity:cafe"}, nil),
		docBuilder("node/2", "locality", "Bucharest", 44.43, 26.1, "ro", nil, nil),
	}
	n, err := bulkIndex(context.Background(), opts, srv.URL, docs)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	lines := strings.Split(strings.TrimRight(got.String(), "\n"), "\n")
	// 2 docs → 4 lines: meta, source, meta, source.
	require.Len(t, lines, 4)
	require.Contains(t, lines[0], `"_index":"pelias"`)
	require.Contains(t, lines[0], `"_id":"openstreetmap:venue:node/1"`)
	require.Contains(t, lines[1], `"name":{"default":"Cafe"}`)
	require.Contains(t, lines[1], `"layer":"venue"`)
	require.Contains(t, lines[2], `"_id":"openstreetmap:locality:node/2"`)
}

// TestEnsureIndex_CreateWhenMissing covers the 404-HEAD → PUT branch.
func TestEnsureIndex_CreateWhenMissing(t *testing.T) {
	putCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		}
	}))
	defer srv.Close()

	opts := Options{}.defaulted()
	err := ensureIndex(context.Background(), opts, srv.URL, zerolog.Nop())
	require.NoError(t, err)
	require.True(t, putCalled)
}

// TestEnsureIndex_AlreadyExistsBranch treats the HEAD-200 + settings
// containing peliasQuery as a no-op (PUT/DELETE must not be called).
func TestEnsureIndex_AlreadyExistsBranch(t *testing.T) {
	putCalled := false
	deleteCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			// /_settings response — analyzer present, so ensureIndex
			// should treat the index as good and short-circuit.
			_, _ = w.Write([]byte(`{"pelias":{"settings":{"index":{"analysis":{"analyzer":{"peliasQuery":{}}}}}}}`))
		case http.MethodPut:
			putCalled = true
		case http.MethodDelete:
			deleteCalled = true
		}
	}))
	defer srv.Close()

	err := ensureIndex(context.Background(), Options{}.defaulted(), srv.URL, zerolog.Nop())
	require.NoError(t, err)
	require.False(t, putCalled, "PUT must not run when index already has pelias analyzers")
	require.False(t, deleteCalled, "DELETE must not run when index already has pelias analyzers")
}

// TestEnsureIndex_RecreatesStaleIndex covers the case where the index
// exists but was created by the old minimal-schema importer (no
// peliasQuery analyzer). Expectation: DELETE then PUT.
func TestEnsureIndex_RecreatesStaleIndex(t *testing.T) {
	deleteCalled := false
	putCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			// Settings without any pelias analyzer block — looks like
			// the old hardcoded minimal mapping from before this fix.
			_, _ = w.Write([]byte(`{"pelias":{"settings":{"index":{"refresh_interval":"10s"}}}}`))
		case http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			putCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		}
	}))
	defer srv.Close()

	err := ensureIndex(context.Background(), Options{}.defaulted(), srv.URL, zerolog.Nop())
	require.NoError(t, err)
	require.True(t, deleteCalled, "stale index must be deleted before recreate")
	require.True(t, putCalled, "stale index must be recreated with the embedded schema")
}

// TestEnsureIndex_PutsEmbeddedSchema verifies the PUT body is the
// full pelias schema (not the old minimal mapping). Without these
// custom analyzers pelias-api 500s with `analyzer [peliasQuery] not
// found` at query time — that is the bug this fix targets.
func TestEnsureIndex_PutsEmbeddedSchema(t *testing.T) {
	var putBody bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPut:
			_, _ = io.Copy(&putBody, r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		}
	}))
	defer srv.Close()

	err := ensureIndex(context.Background(), Options{}.defaulted(), srv.URL, zerolog.Nop())
	require.NoError(t, err)
	body := putBody.String()
	require.Contains(t, body, `"peliasQuery"`, "embedded schema must define peliasQuery analyzer")
	require.Contains(t, body, `"peliasIndexOneEdgeGram"`)
	require.Contains(t, body, `"peliasPhrase"`)
	require.Contains(t, body, `"peliasHousenumber"`)
	require.Contains(t, body, `"peliasStreet"`)
}

// TestBuildWithOptions_MissingPBF surfaces the pbf-open error through
// the public Build entry (guards against a regression that'd silently
// swallow the missing file).
func TestBuildWithOptions_MissingPBF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // HEAD success short-circuits ensureIndex
	}))
	defer srv.Close()
	_, err := BuildWithOptions(context.Background(), "/tmp/does-not-exist.pbf",
		srv.URL, "test", Options{}, zerolog.Nop())
	require.Error(t, err)
	require.Contains(t, err.Error(), "open pbf")
}

// TestBuildWithOptions_GuardEmptyInputs covers the required-arg checks.
func TestBuildWithOptions_GuardEmptyInputs(t *testing.T) {
	cases := []struct {
		pbf, esURL, region string
	}{
		{"", "http://x", "ro"},
		{"/x", "", "ro"},
		{"/x", "http://x", ""},
	}
	for _, c := range cases {
		_, err := BuildWithOptions(context.Background(), c.pbf, c.esURL, c.region,
			Options{}, zerolog.Nop())
		require.Error(t, err)
	}
}

// TestBulkDelete_PostsDeleteByQuery asserts the wire shape of the
// region-purge call: POST /pelias/_delete_by_query with a term filter
// on `addendum.osm.region`.
func TestBulkDelete_PostsDeleteByQuery(t *testing.T) {
	var seenPath, seenMethod string
	var seenBody bytes.Buffer
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenMethod = r.Method
		_, _ = io.Copy(&seenBody, r.Body)
		_, _ = w.Write([]byte(`{"deleted":42,"failures":[]}`))
	}))
	defer srv.Close()

	deleted, err := bulkDelete(context.Background(),
		Options{}.defaulted(), srv.URL, "europe-romania")
	require.NoError(t, err)
	require.Equal(t, int64(42), deleted)
	require.Equal(t, http.MethodPost, seenMethod)
	require.Equal(t, "/pelias/_delete_by_query", seenPath)
	require.Contains(t, seenBody.String(), `"addendum.osm.region"`)
	require.Contains(t, seenBody.String(), `"europe-romania"`)
}

// TestPurgeRegion_Wraps404AsZero covers the "delete on a region whose
// docs were never indexed (or whose pelias index doesn't exist yet)"
// edge case — must surface 0 deletions, not an error.
func TestPurgeRegion_Wraps404AsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	deleted, err := PurgeRegion(context.Background(), srv.URL, "europe-romania", zerolog.Nop())
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted)
}

// TestPurgeRegion_GuardsEmptyArgs covers the required-arg checks.
func TestPurgeRegion_GuardsEmptyArgs(t *testing.T) {
	_, err := PurgeRegion(context.Background(), "", "europe-romania", zerolog.Nop())
	require.Error(t, err)
	_, err = PurgeRegion(context.Background(), "http://x", "", zerolog.Nop())
	require.Error(t, err)
}
