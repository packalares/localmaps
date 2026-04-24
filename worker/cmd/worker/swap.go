// swap.go — the terminal Asynq handler that promotes <region>.new →
// <region> atomically, after all pipeline stages have succeeded. The
// implementation is a simple rename dance:
//
//  1. If <dataDir>/regions/<region>/ already exists (an update), rename
//     it to <region>.old-<ts> — kept only until step 3 completes so a
//     crash between 2 and 3 can be recovered.
//  2. Rename <region>.new → <region>.
//  3. Remove <region>.old-<ts>.
//
// All three filesystem operations must live on the same mountpoint for
// the rename to be atomic; that is the case for hostPath-mounted
// deployments (docs/10-deploy.md).
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/jobs"
)

// runSwap is the default SwapWork plugged into swapHandler in
// registerAgentEHandlers. It is a function (not a method) so tests can
// trivially inject a noop equivalent.
func runSwap(_ context.Context, deps ChainDeps, p jobs.RegionSwapPayload, log zerolog.Logger) error {
	regionsDir := filepath.Join(deps.DataDir, "regions")
	newPath := filepath.Join(regionsDir, p.Region+".new")
	finalPath := filepath.Join(regionsDir, p.Region)
	oldPath := filepath.Join(regionsDir,
		p.Region+".old-"+time.Now().UTC().Format("20060102T150405"))

	info, err := os.Stat(newPath)
	switch {
	case os.IsNotExist(err):
		return fmt.Errorf("swap: %q missing; chain must land source.osm.pbf first", newPath)
	case err != nil:
		return fmt.Errorf("swap: stat new: %w", err)
	case !info.IsDir():
		return fmt.Errorf("swap: %q is not a directory", newPath)
	}

	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Rename(finalPath, oldPath); err != nil {
			return fmt.Errorf("swap: move existing: %w", err)
		}
		log.Debug().Str("archived", oldPath).Msg("archived previous region directory")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("swap: stat existing: %w", err)
	}

	if err := os.Rename(newPath, finalPath); err != nil {
		// Best-effort restore if the archive step above moved the old dir.
		_ = os.Rename(oldPath, finalPath)
		return fmt.Errorf("swap: promote: %w", err)
	}
	if _, err := os.Stat(oldPath); err == nil {
		if err := os.RemoveAll(oldPath); err != nil {
			// Non-fatal: region is live, we just leave an archive behind
			// for the next scheduled GC pass.
			log.Warn().Err(err).Str("path", oldPath).
				Msg("could not remove archived region dir; leaving for GC")
		}
	}
	return nil
}
