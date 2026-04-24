// Package pipeline — pelias_util.go holds helpers the pelias runner
// relies on. Kept out of pelias.go so the runner file stays under the
// 250-line cap.
package pipeline

import (
	"os"
	"path/filepath"
	"syscall"
)

// peliasSigterm is the signal CommandContext sends to the importer on
// cancellation. SIGKILL is applied automatically by the Go runtime
// after cmd.WaitDelay. Prefixed so it cannot collide with other
// runners' termination helpers in the same package.
var peliasSigterm = syscall.SIGTERM

// writeFileAtomic writes b to path via a same-dir temp file + rename.
// Keeps partially-written pelias.json out of the importer's view if the
// worker crashes mid-write.
func writeFileAtomic(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".pelias-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// Best-effort cleanup if rename didn't happen.
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
