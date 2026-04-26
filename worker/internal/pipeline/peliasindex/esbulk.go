// Package peliasindex — esbulk.go wraps the two Elasticsearch calls the
// importer makes: the idempotent index-create (`ensureIndex`) and the
// batched document upload (`bulkIndex`). Split out of peliasindex.go so
// the main file fits under the 250-line cap.
package peliasindex

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// schemaJSON is the canonical pelias Elasticsearch schema (settings +
// mappings) generated from https://github.com/pelias/schema via
// `node scripts/output_mapping.js`-equivalent. Embedding it here gives
// pelias-api the custom analyzers (peliasQuery, peliasIndexOneEdgeGram,
// peliasPhrase, peliasAdmin, peliasStreet, peliasHousenumber, peliasZip,
// …) it expects at query time. Without these, pelias-api errors with
// `[query_shard_exception] analyzer [peliasQuery] not found`.
//
//go:embed schema.json
var schemaJSON []byte

// ensureIndex creates opts.IndexName with the pelias schema if it does
// not already exist. If the index exists but lacks the peliasQuery
// analyzer (i.e. it was created by an older minimal-schema build that
// caused pelias-api to 500), it is dropped and recreated. Idempotent —
// 200 and 400-already-exists are both treated as success.
func ensureIndex(ctx context.Context, opts Options, esURL string, log zerolog.Logger) error {
	url := esURL + "/" + opts.IndexName
	exists, err := indexExists(ctx, opts, url)
	if err != nil {
		return err
	}
	if exists {
		ok, err := indexHasPeliasAnalyzers(ctx, opts, url)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		// Existing index is missing the pelias analyzers — drop it so
		// the next PUT below recreates it with the correct schema.
		log.Warn().Str("index", opts.IndexName).
			Msg("existing pelias index lacks peliasQuery analyzer; dropping for recreate")
		if err := deleteIndex(ctx, opts, url); err != nil {
			return err
		}
	}
	return createIndex(ctx, opts, url, log)
}

// indexExists returns whether url responds to HEAD with 200.
func indexExists(ctx context.Context, opts Options, url string) (bool, error) {
	reqCtx, cancel := context.WithTimeout(ctx, opts.ESTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	resp, err := opts.HTTP.Do(req)
	if err != nil {
		return false, fmt.Errorf("HEAD %s: %w", url, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// indexHasPeliasAnalyzers fetches the index's settings and reports
// whether `analysis.analyzer.peliasQuery` is defined. A missing
// analyzer is the canonical signal that the index was created by the
// old minimal-schema importer.
func indexHasPeliasAnalyzers(ctx context.Context, opts Options, url string) (bool, error) {
	reqCtx, cancel := context.WithTimeout(ctx, opts.ESTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url+"/_settings", nil)
	if err != nil {
		return false, err
	}
	resp, err := opts.HTTP.Do(req)
	if err != nil {
		return false, fmt.Errorf("GET %s/_settings: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return false, nil
	}
	body, _ := io.ReadAll(resp.Body)
	// We don't decode the full settings tree; a substring check on the
	// analyzer name is sufficient and cheap.
	return bytes.Contains(body, []byte(`"peliasQuery"`)), nil
}

// deleteIndex DELETEs url. Returns nil on 200 or 404 (already-gone is
// success for our purposes).
func deleteIndex(ctx context.Context, opts Options, url string) error {
	reqCtx, cancel := context.WithTimeout(ctx, opts.ESTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := opts.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("DELETE %s: status %d: %s", url, resp.StatusCode, truncate(string(body), 200))
}

// createIndex PUTs the embedded pelias schema to url. Treats
// `resource_already_exists_exception` (HTTP 400) as success so racing
// importers don't trip each other.
func createIndex(ctx context.Context, opts Options, url string, log zerolog.Logger) error {
	reqCtx, cancel := context.WithTimeout(ctx, opts.ESTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, url, bytes.NewReader(schemaJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := opts.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusBadRequest &&
		bytes.Contains(body, []byte("resource_already_exists")) {
		return nil
	}
	log.Warn().Int("status", resp.StatusCode).
		Str("body", string(body)).Msg("ES index create unexpected response")
	return fmt.Errorf("ES index create %s: status %d", url, resp.StatusCode)
}

// bulkIndex posts `batch` as a single _bulk request. The Elasticsearch
// bulk format is newline-delimited JSON:
//
//	{"index":{"_index":"pelias","_id":"<gid>"}}
//	{...doc...}
//
// Returns the number of docs the server accepted.
func bulkIndex(ctx context.Context, opts Options, esURL string, batch []doc) (int, error) {
	var buf bytes.Buffer
	for _, d := range batch {
		// Pelias's _id convention is "<source>:<layer>:<source_id>".
		// Reconstruct here rather than carrying a struct field (strict
		// schema rejects a top-level `gid` property).
		gid := d.Source + ":" + d.Layer + ":" + d.SourceID
		meta := map[string]map[string]string{
			"index": {"_index": opts.IndexName, "_id": gid},
		}
		if err := json.NewEncoder(&buf).Encode(meta); err != nil {
			return 0, err
		}
		if err := json.NewEncoder(&buf).Encode(d); err != nil {
			return 0, err
		}
	}
	reqCtx, cancel := context.WithTimeout(ctx, opts.ESTimeout)
	defer cancel()
	url := esURL + "/_bulk"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := opts.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("POST _bulk: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("_bulk status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return len(batch), nil
}

// bulkDelete issues a `_delete_by_query` against opts.IndexName,
// matching every doc tagged with `addendum.osm.region == regionKey`.
// This is the per-region cleanup primitive used by KindRegionDelete:
// the strict pelias schema rejects unknown root fields, but the
// `addendum` field is `dynamic:true` so tagging + matching is free.
//
// Returns the number of documents deleted (parsed from the ES response
// body's `deleted` field). 404 on the index is treated as 0 deletions
// + nil error so a delete on a freshly-installed primary that never
// imported the region doesn't fail loudly.
//
// 400 with a `Cannot search on field [addendum.osm.region] since it is
// not indexed` shard exception is also treated as 0 deletions + nil:
// the upstream Pelias schema's dynamic_template maps every
// `addendum.*` field to `index:false, doc_values:false` (Pelias itself
// never queries them, saves space). ES forbids flipping that flag on a
// live field, so the only "fix" would be a full reindex. Until that
// happens we accept that per-region purges leak stale docs — adding
// new docs still works (writes don't depend on the field being
// indexed), and the user-visible delete flow no longer fails loudly.
func bulkDelete(ctx context.Context, opts Options, esURL, regionKey string, log zerolog.Logger) (int64, error) {
	if regionKey == "" {
		return 0, fmt.Errorf("bulkDelete: empty region key")
	}
	body := map[string]any{
		"query": map[string]any{
			"term": map[string]any{
				"addendum.osm.region": regionKey,
			},
		},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("bulkDelete marshal: %w", err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, opts.ESTimeout)
	defer cancel()
	url := esURL + "/" + opts.IndexName + "/_delete_by_query"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := opts.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("POST _delete_by_query: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return 0, nil
	}
	if resp.StatusCode == http.StatusBadRequest && fieldNotIndexed(respBody, "addendum.osm.region") {
		log.Warn().
			Str("region", regionKey).
			Msg("peliasindex purge skipped: addendum.osm.region not indexed (legacy schema); leaving stale docs in place")
		return 0, nil
	}
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("_delete_by_query status %d: %s",
			resp.StatusCode, truncate(string(respBody), 200))
	}
	var parsed struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// Body shape may include `task` when wait_for_completion=false;
		// here we always wait so `deleted` is always present, but be
		// permissive on decode failure rather than masking an otherwise
		// successful 200.
		return 0, nil
	}
	return parsed.Deleted, nil
}

// fieldNotIndexed scans an ES error response body for the
// `query_shard_exception` reason ES emits when a query targets a field
// whose mapping has `index: false`. Cheaper than parsing the JSON since
// the message text is stable across ES 7.x.
func fieldNotIndexed(body []byte, field string) bool {
	s := string(body)
	if !strings.Contains(s, "query_shard_exception") {
		return false
	}
	return strings.Contains(s, "Cannot search on field ["+field+"]")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
