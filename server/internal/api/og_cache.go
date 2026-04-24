package api

// Disk-backed cache for rendered OG preview PNGs. Split out of og.go
// to respect the 250-line-per-file cap. Single-writer semantics are
// fine here because every cache entry is content-addressed — two
// concurrent writers for the same key produce byte-identical output.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/packalares/localmaps/internal/safepath"
	"github.com/packalares/localmaps/server/internal/og"
)

// cacheKey returns a hex-sha256 of the canonicalised params. The order
// here is fixed — changing it invalidates every cached file but that
// is acceptable because the cache is disposable.
func cacheKey(p og.Params) string {
	pin := "0"
	if p.Pin != nil {
		pin = fmt.Sprintf("1,%.6f,%.6f", p.Pin.Lat, p.Pin.Lon)
	}
	canon := fmt.Sprintf("v1|%.6f|%.6f|%d|%s|%dx%d|%s|%s",
		p.Center.Lat, p.Center.Lon, p.Zoom, pin, p.Size.W, p.Size.H, p.Style, p.Region)
	sum := sha256.Sum256([]byte(canon))
	return hex.EncodeToString(sum[:])
}

// cacheFilePath builds the on-disk path for a cache entry, safely
// joined under dataDir so that a crafted key can never escape.
func cacheFilePath(dataDir, key string) (string, error) {
	if dataDir == "" {
		// Fall back to a per-process tmp dir. Cached entries for an
		// ephemeral tmp-backed gateway are still useful within a run.
		dataDir = filepath.Join(os.TempDir(), "localmaps")
	}
	return safepath.Join(dataDir, "cache", "og", key+".png")
}

// readCache returns the file contents plus true on hit, zero bytes
// plus false on miss or any read error (treated as cache-miss — the
// renderer will rebuild).
func readCache(path string) ([]byte, bool) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is safepath-joined
	if err != nil {
		return nil, false
	}
	return data, true
}

// writeCache is best-effort: mkdir-p parent, then same-dir temp file +
// atomic rename so partial writes never leave a bad entry.
func writeCache(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".og-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// cacheCheckOnly serialises the Stat probe in tests. Not used in
// production; exported to the package for og_test.go's parallel
// fixture setup.
var cacheCheckOnly sync.Mutex

// ensureCacheFileExists returns true when the cache entry for key
// already lives on disk. Used by tests to assert the miss→disk-write
// path landed correctly.
func ensureCacheFileExists(dataDir, key string) bool {
	path, err := cacheFilePath(dataDir, key)
	if err != nil {
		return false
	}
	cacheCheckOnly.Lock()
	defer cacheCheckOnly.Unlock()
	_, err = os.Stat(path)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}
