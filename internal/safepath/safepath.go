// Package safepath joins path components under a root, rejecting any
// inputs that would escape the root (via "..", absolute paths, or NUL
// bytes). Used whenever a user-supplied string feeds into a filesystem
// operation — see docs/06-agent-rules.md R8.
package safepath

import (
	"errors"
	"path/filepath"
	"strings"
)

// Errors returned by Join.
var (
	ErrEmptyRoot     = errors.New("safepath: root must be a non-empty absolute path")
	ErrAbsolutePart  = errors.New("safepath: absolute path components are forbidden")
	ErrNullByte      = errors.New("safepath: NUL byte in path component")
	ErrEscapesRoot   = errors.New("safepath: path escapes root")
	ErrEmptyPart     = errors.New("safepath: empty path component")
)

// Join combines root with the given parts and returns an absolute path
// that is guaranteed to live under root. It rejects:
//
//   - absolute components (e.g. "/etc/passwd")
//   - NUL bytes in any component
//   - ".." sequences that would climb past the root
//
// The returned path is always cleaned.
func Join(root string, parts ...string) (string, error) {
	if root == "" || !filepath.IsAbs(root) {
		return "", ErrEmptyRoot
	}
	cleanedRoot := filepath.Clean(root)

	for _, p := range parts {
		if p == "" {
			return "", ErrEmptyPart
		}
		if strings.ContainsRune(p, 0) {
			return "", ErrNullByte
		}
		if filepath.IsAbs(p) {
			return "", ErrAbsolutePart
		}
	}

	joined := filepath.Join(append([]string{cleanedRoot}, parts...)...)
	cleaned := filepath.Clean(joined)

	// Must remain within cleanedRoot. filepath.Rel tells us if it escapes.
	rel, err := filepath.Rel(cleanedRoot, cleaned)
	if err != nil {
		return "", ErrEscapesRoot
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrEscapesRoot
	}
	return cleaned, nil
}
