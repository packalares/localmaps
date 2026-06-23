package main

// region_size.go — disk-usage rollup for a finished region directory.
//
// `regions.disk_bytes` drives the "Size" column in the admin UI. The
// worker writes it once at finishSwap so users see a real footprint
// for each installed region. Filesystem-level rollup (sum of every
// regular file under `<dataDir>/regions/<name>/`) keeps the math
// honest even when Planetiler writes auxiliary sidecars we don't
// track explicitly in code.

import (
	"errors"
	"io/fs"
	"path/filepath"
)

// sumRegionDirBytes walks `root` and returns the sum of every regular
// file's size. Directories, symlinks, devices, and sockets contribute
// 0. Returns 0 + nil when the root is missing (caller's job to treat
// 0 as "no size yet"). Other walk errors propagate so the caller can
// decide whether to log + carry on or fail the job.
//
// Implementation notes:
//   - Uses filepath.Walk (not WalkDir + Type) so we get the FileInfo
//     and `Size()` in one shot — `regions.disk_bytes` is INT64 and
//     pmtiles files routinely exceed 32-bit, so int64 throughout.
//   - Skips broken symlinks rather than aborting the walk. A region's
//     valhalla_tiles dir is a real directory but the planetiler cache
//     dirs sometimes contain dangling symlinks left by mid-build
//     crashes; we'd rather report a partial-but-mostly-correct total
//     than 0 because of one bad link.
func sumRegionDirBytes(root string) (int64, error) {
	var total int64
	walkErr := filepath.Walk(root, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Missing root → return 0, no error. Missing inner
				// file → swallow and continue (broken symlink case).
				if p == root {
					return filepath.SkipAll
				}
				return nil
			}
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if walkErr != nil && errors.Is(walkErr, fs.ErrNotExist) {
		return 0, nil
	}
	return total, walkErr
}
