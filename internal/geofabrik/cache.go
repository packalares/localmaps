package geofabrik

// cache.go keeps the disk-cache + in-memory-index helpers apart from
// the main client surface. Split per docs/06-agent-rules.md line cap.

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// hostAllowed checks host against the egress allow-list.
func (c *Client) hostAllowed(host string) bool {
	for _, h := range c.allowed {
		if strings.EqualFold(host, h) {
			return true
		}
	}
	return false
}

// setIndex replaces the in-memory index and rebuilds lookup maps.
func (c *Client) setIndex(tree []CatalogEntry, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.indexMem = tree
	c.indexAt = at
	c.byKey = make(map[string]*CatalogEntry)
	var walk func(es []CatalogEntry)
	walk = func(es []CatalogEntry) {
		for i := range es {
			e := &es[i]
			c.byKey[e.Name] = e
			walk(e.Children)
		}
	}
	walk(c.indexMem)
}

// copy returns a deep-ish copy of the tree so callers can't mutate our
// cache.
func (c *Client) copy() []CatalogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]CatalogEntry, len(c.indexMem))
	copy(out, c.indexMem)
	return out
}

func (c *Client) readDiskCache() ([]byte, time.Time, bool) {
	p := filepath.Join(c.cacheDir, "index-v1.json")
	st, err := os.Stat(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	return b, st.ModTime(), true
}

func (c *Client) writeDiskCache(b []byte) {
	if c.cacheDir == "" {
		return
	}
	_ = os.MkdirAll(c.cacheDir, 0o755)
	tmp := filepath.Join(c.cacheDir, "index-v1.json.tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(c.cacheDir, "index-v1.json"))
}

// hostOf parses a URL and returns the hostname (without port).
func hostOf(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host in %q", rawURL)
	}
	return u.Hostname(), nil
}
