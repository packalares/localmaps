package og

import (
	"context"
	"errors"
	"image"
	"path/filepath"

	"github.com/packalares/localmaps/internal/safepath"
)

// ErrUnavailable is returned by a TileSource when it cannot satisfy a
// raster request — for example, the region isn't installed, the
// pmtiles archive is missing, or the compositor is not implemented
// yet. Render treats this as a silent fallback trigger.
var ErrUnavailable = errors.New("og: tile source unavailable")

// RegionPMTilesPath returns the canonical on-disk location of a
// region's map.pmtiles file:
//
//	<dataDir>/regions/<canonicalKey>/map.pmtiles
//
// Any attempt to escape <dataDir>/regions via a crafted key is rejected
// via internal/safepath — mirrors the guarantees in docs/08-security.md.
// A nil error means the path is safe to pass to os.Open; the file may
// or may not exist.
func RegionPMTilesPath(dataDir, canonicalKey string) (string, error) {
	if dataDir == "" {
		return "", errors.New("og: empty dataDir")
	}
	if canonicalKey == "" {
		return "", errors.New("og: empty region key")
	}
	abs := dataDir
	if !filepath.IsAbs(abs) {
		var err error
		abs, err = filepath.Abs(abs)
		if err != nil {
			return "", err
		}
	}
	return safepath.Join(abs, "regions", canonicalKey, "map.pmtiles")
}

// DiskPMTilesSource is a placeholder implementation of TileSource that
// resolves the region's pmtiles path but never produces an image. It
// exists so callers can wire a TileSource today and swap the guts in a
// future phase without touching the Renderer or the HTTP handler.
//
// DataDir is the gateway boot config LOCALMAPS_DATA_DIR.
type DiskPMTilesSource struct {
	DataDir string
	Region  string
}

// Raster is the stub implementation — always returns ErrUnavailable.
// A follow-up agent swaps this for a real pmtiles compositor.
func (d *DiskPMTilesSource) Raster(ctx context.Context, center LatLon, zoom int, size Size) (*image.RGBA, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	_ = center
	_ = zoom
	_ = size
	if d.DataDir == "" || d.Region == "" {
		return nil, ErrUnavailable
	}
	if _, err := RegionPMTilesPath(d.DataDir, d.Region); err != nil {
		return nil, err
	}
	return nil, ErrUnavailable
}
