package geofabrik

// fetch.go holds the per-region lookups (Resolve, ResolvePbfURL,
// FetchSHA256). client.go stays focused on the catalog list + cache.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	sharedregions "github.com/packalares/localmaps/internal/regions"
)

// Resolve returns the catalog entry for a canonical key. It forces a
// catalog refresh if the key is absent from the current in-memory
// index so that freshly-added regions can be found without waiting
// for the cache TTL to expire.
func (c *Client) Resolve(ctx context.Context, canonicalKey string) (CatalogEntry, error) {
	if !sharedregions.IsCanonical(canonicalKey) {
		return CatalogEntry{}, fmt.Errorf("geofabrik: key %q is not canonical", canonicalKey)
	}
	if _, err := c.ListRegions(ctx); err != nil {
		return CatalogEntry{}, err
	}
	c.mu.Lock()
	if e, ok := c.byKey[canonicalKey]; ok {
		out := *e
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()
	return CatalogEntry{}, fmt.Errorf("%w: %s", ErrNotInCatalog, canonicalKey)
}

// ResolvePbfURL derives the -latest.osm.pbf download URL from a catalog
// entry, validates the hostname against the egress allow-list, and
// returns it.
func (c *Client) ResolvePbfURL(entry CatalogEntry) (string, error) {
	if entry.SourceURL == "" {
		return "", ErrNoPbfURL
	}
	host, err := hostOf(entry.SourceURL)
	if err != nil {
		return "", fmt.Errorf("parse sourceUrl %q: %w", entry.SourceURL, err)
	}
	if !c.hostAllowed(host) {
		return "", fmt.Errorf("%w: %s", ErrEgressDenied, host)
	}
	return entry.SourceURL, nil
}

// FetchSHA256 attempts to retrieve the checksum Geofabrik publishes
// next to each pbf. Geofabrik publishes an .md5 sidecar (the .osm.pbf
// suffix is replaced by .osm.pbf.md5); there is no SHA-256 counterpart
// on the mirror. The returned checksum string is the md5 hex digest,
// and the data model's `source_pbf_sha256` column is used to store
// it — the column name predates this finding. See the agent report.
//
// Returns the checksum and the Content-Length of the pbf (obtained
// via HEAD). Either may be empty if the server doesn't publish them.
func (c *Client) FetchSHA256(ctx context.Context, pbfURL string) (string, int64, error) {
	host, err := hostOf(pbfURL)
	if err != nil {
		return "", 0, fmt.Errorf("parse pbf url %q: %w", pbfURL, err)
	}
	if !c.hostAllowed(host) {
		return "", 0, fmt.Errorf("%w: %s", ErrEgressDenied, host)
	}
	md5URL := pbfURL + ".md5"
	log := zerolog.Ctx(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, md5URL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("build md5 request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("fetch md5: %w", err)
	}
	defer resp.Body.Close()
	var checksum string
	if resp.StatusCode == http.StatusOK {
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, 256))
		if rerr != nil {
			return "", 0, fmt.Errorf("read md5 body: %w", rerr)
		}
		parts := strings.Fields(string(body))
		if len(parts) > 0 {
			checksum = strings.ToLower(parts[0])
		}
	} else {
		log.Warn().Int("status", resp.StatusCode).Str("url", md5URL).
			Msg("md5 sidecar not available")
	}

	var size int64
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, pbfURL, nil)
	if err != nil {
		return checksum, 0, fmt.Errorf("build pbf head: %w", err)
	}
	headResp, err := c.http.Do(headReq)
	if err != nil {
		return checksum, 0, fmt.Errorf("head pbf: %w", err)
	}
	defer headResp.Body.Close()
	if headResp.StatusCode == http.StatusOK {
		size = headResp.ContentLength
	}
	return checksum, size, nil
}
