// Package geofabrik is a read-only HTTP client for the Geofabrik
// catalog (index-v1.json) and the per-extract pbf metadata (.md5
// sidecar). It does NOT download the pbf blob itself — the download
// streaming lives in the worker's install handler, which uses
// ResolvePbfURL and FetchSHA256 from here to know what to fetch and
// how to verify it.
//
// The client treats every outbound host as untrusted until its
// hostname matches the mirror base configured in settings; see
// docs/08-security.md for the egress allow-list policy.
package geofabrik

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/catalog"
)

// CatalogEntry is an alias for the shared catalog.Entry. Kept so
// older call sites written against this package keep compiling;
// new code should use catalog.Entry directly.
type CatalogEntry = catalog.Entry

// Kind mirrors catalog.Kind.
type Kind = catalog.Kind

// Re-exports for convenience.
const (
	KindContinent = catalog.KindContinent
	KindCountry   = catalog.KindCountry
	KindSubregion = catalog.KindSubregion
)

// Common errors.
var (
	// ErrNotInCatalog is returned by Resolve when the canonical key
	// cannot be matched against any catalog entry.
	ErrNotInCatalog = errors.New("geofabrik: region not in catalog")
	// ErrEgressDenied is returned when ResolvePbfURL would dial a host
	// that is not on the allow-list derived from regions.mirrorBase.
	ErrEgressDenied = errors.New("geofabrik: egress to host not allowed")
	// ErrNoPbfURL is returned when a catalog entry does not expose a pbf url.
	ErrNoPbfURL = errors.New("geofabrik: entry has no pbf url")
)

// SettingsReader is the narrow view of the server's config.Store the
// client needs. We accept an interface so tests can fake it without
// dragging the sqlite package into the worker's test deps.
type SettingsReader interface {
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
}

// Client is a thread-safe catalog client.
type Client struct {
	http       *http.Client
	baseURL    string // e.g. "https://download.geofabrik.de"
	catalogURL string // e.g. "https://download.geofabrik.de/index-v1.json"
	cacheDir   string // e.g. "/data/cache/geofabrik"
	cacheTTL   time.Duration
	allowed    []string // allow-listed hostnames for pbf/md5 egress

	mu       sync.Mutex
	indexMem []CatalogEntry
	indexAt  time.Time
	byKey    map[string]*CatalogEntry // canonical key -> entry
}

// defaultCacheTTL is used when the operator hasn't provided a setting.
// Geofabrik refreshes daily; 24h is a sane baseline.
const defaultCacheTTL = 24 * time.Hour

// NewClient builds a Client. If httpClient is nil, a default with a
// 60-second timeout is used. The mirror base and catalog URL are read
// from regions.mirrorBase and regions.catalogURL in settings
// (see docs/07-config-schema.md); the hostname of mirrorBase is added
// to the egress allow-list. The cache directory is created lazily on
// first successful fetch.
func NewClient(httpClient *http.Client, store SettingsReader, cacheDir string) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	base, err := store.GetString("regions.mirrorBase")
	if err != nil {
		return nil, fmt.Errorf("read regions.mirrorBase: %w", err)
	}
	base = strings.TrimRight(base, "/")
	if base == "" {
		return nil, errors.New("geofabrik: regions.mirrorBase is empty")
	}
	catalog, err := store.GetString("regions.catalogURL")
	if err != nil || catalog == "" {
		catalog = base + "/index-v1.json"
	}
	baseHost, err := hostOf(base)
	if err != nil {
		return nil, fmt.Errorf("parse regions.mirrorBase %q: %w", base, err)
	}
	allowed := []string{baseHost}
	if catHost, err := hostOf(catalog); err == nil && catHost != baseHost {
		allowed = append(allowed, catHost)
	}
	return &Client{
		http:       httpClient,
		baseURL:    base,
		catalogURL: catalog,
		cacheDir:   cacheDir,
		cacheTTL:   defaultCacheTTL,
		allowed:    allowed,
		byKey:      make(map[string]*CatalogEntry),
	}, nil
}

// NewClientWithBase is the explicit-base constructor used by tests
// (and any caller that doesn't go through SettingsReader). It
// mirrors NewClient but takes baseURL directly.
func NewClientWithBase(httpClient *http.Client, baseURL, cacheDir string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	base := strings.TrimRight(baseURL, "/")
	catalog := base + "/index-v1.json"
	host, _ := hostOf(base)
	return &Client{
		http:       httpClient,
		baseURL:    base,
		catalogURL: catalog,
		cacheDir:   cacheDir,
		cacheTTL:   defaultCacheTTL,
		allowed:    []string{host},
		byKey:      make(map[string]*CatalogEntry),
	}
}

// ListRegions fetches (or returns cached) the full catalog tree. Cache
// freshness is governed by regions.catalogCacheTtlHours. The in-memory
// index is rebuilt from disk every call after the TTL expires.
func (c *Client) ListRegions(ctx context.Context) ([]CatalogEntry, error) {
	c.mu.Lock()
	fresh := c.indexMem != nil && time.Since(c.indexAt) < c.cacheTTL
	c.mu.Unlock()
	if fresh {
		return c.copy(), nil
	}
	// Try on-disk cache before hitting the network.
	if bs, at, ok := c.readDiskCache(); ok && time.Since(at) < c.cacheTTL {
		if tree, err := decodeCatalog(bs); err == nil {
			c.setIndex(tree, at)
			return c.copy(), nil
		}
	}
	// Fetch.
	url := c.catalogURL
	log := zerolog.Ctx(ctx)
	log.Info().Str("url", url).Msg("fetching geofabrik index")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build catalog request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog http %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read catalog body: %w", err)
	}
	tree, err := decodeCatalog(body)
	if err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	c.writeDiskCache(body)
	c.setIndex(tree, time.Now().UTC())
	return c.copy(), nil
}

// Resolve, ResolvePbfURL, and FetchSHA256 live in fetch.go.

// BaseURL returns the mirror base URL the client is configured for.
func (c *Client) BaseURL() string { return c.baseURL }

// Cache + allow-list helpers live in cache.go; decoder in decode.go.
