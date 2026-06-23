package main

// cleanup_inprogress.go — sweeps orphan `*_inprogress` files left by
// crashed Planetiler runs.
//
// Planetiler's Downloader writes aux source archives (water polygons,
// natural earth, lake centerlines) to `<target>_inprogress` first, then
// atomic-renames on success. When the worker container is OOMKilled or
// otherwise terminates mid-download, the `_inprogress` file is left
// behind. On the NEXT run, Planetiler's `Files.move()` rename can fail
// with a confusing NoSuchFileException because two concurrent downloads
// raced on the same temp filename and one cleaned it up first — but the
// underlying root cause is the orphan from the crashed run.
//
// Sweeping on worker boot is safe: when this runs we hold no Planetiler
// children (we haven't started Asynq yet), so every `_inprogress` file
// is by definition orphaned and removable.

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// cleanupInProgressFiles walks the bundled planetiler cache + the
// data/sources/ subtree underneath it and removes any file whose name
// ends in `_inprogress`. Returns the number of files removed (for
// observability) plus any walk error.
//
// The walk root mirrors what handlers_agent_fg.go uses for the
// Planetiler WorkingDir: `<dataDir>/cache/planetiler`. Symlinks are
// followed via filepath.Walk so the live-pod workaround (symlinking
// `/app/data/sources` to `/data/sources`) is covered too.
func cleanupInProgressFiles(dataDir string, log zerolog.Logger) (int, error) {
	root := filepath.Join(dataDir, "cache", "planetiler")
	removed := 0
	walkErr := filepath.Walk(root, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			// Root doesn't exist yet on first boot — that's expected,
			// not a failure. Anything else (permission, broken symlink)
			// is logged and continues; we don't want one bad file to
			// abort the rest of the sweep.
			if errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			log.Warn().Str("path", p).Err(err).
				Msg("inprogress-sweep: walk error, continuing")
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), "_inprogress") {
			return nil
		}
		if rmErr := removeFile(p); rmErr != nil {
			log.Warn().Str("path", p).Err(rmErr).
				Msg("inprogress-sweep: remove failed, continuing")
			return nil
		}
		removed++
		log.Info().Str("path", p).Int64("bytes", info.Size()).
			Msg("inprogress-sweep: removed orphan")
		return nil
	})
	return removed, walkErr
}

// removeFile is split out so tests can inject a fake. Real callers use
// os.Remove; if that ever needs to retry or chmod-first, the change is
// contained.
var removeFile = func(p string) error {
	return osRemove(p)
}
