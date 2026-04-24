package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ErrEgressDenied is returned when DownloadJar is asked to fetch from a
// host that is not in the configured egress allow-list. Mirrors
// apierr.CodeEgressDenied on the server side — see docs/08-security.md.
var ErrEgressDenied = errors.New("pipeline: egress denied")

// ErrEgressAllowListMissing indicates the operator has not configured
// the allow-list; we fail closed per docs/08-security.md ("No
// dependency version ranges", "secure by default").
var ErrEgressAllowListMissing = errors.New("pipeline: egress allow-list missing")

// ErrJarSHAMismatch is returned when a downloaded jar does not match
// the expected SHA-256 hex digest supplied by the caller.
var ErrJarSHAMismatch = errors.New("pipeline: jar sha256 mismatch")

// HTTPDoer is the tiny subset of *http.Client that DownloadJar needs.
// Callers may inject a client with custom timeouts / transports.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// JarCache fetches planetiler (or other) jars into a local cache dir,
// verifying SHA-256. Cache filenames embed the first 8 hex digits of
// the expected digest so different pinned versions coexist safely.
type JarCache struct {
	DestDir  string
	Client   HTTPDoer
	Allowed  []string // host names (lowercase, exact match) permitted for egress
	MaxBytes int64    // optional cap on download size; 0 = unlimited
}

// Ensure returns the on-disk path to a jar matching the expected SHA,
// downloading it if the cache is cold. The download is atomic: data
// goes to a temp file beside the final destination and is renamed only
// after SHA verification passes.
func (c *JarCache) Ensure(ctx context.Context, rawURL, sha256hex string) (string, error) {
	if c == nil {
		return "", errors.New("pipeline: nil jar cache")
	}
	if len(c.Allowed) == 0 {
		return "", ErrEgressAllowListMissing
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	if !hostAllowed(u.Host, c.Allowed) {
		return "", fmt.Errorf("%w: %s", ErrEgressDenied, u.Host)
	}
	if err := validateSHA(sha256hex); err != nil {
		return "", err
	}
	if err := os.MkdirAll(c.DestDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir cache: %w", err)
	}
	finalName := fmt.Sprintf("planetiler-%s.jar", strings.ToLower(sha256hex)[:8])
	finalPath := filepath.Join(c.DestDir, finalName)
	// Cache hit: verify on-disk still matches. If the operator swapped
	// files under us we want to redownload, so we always hash.
	if ok, _ := fileMatchesSHA(finalPath, sha256hex); ok {
		return finalPath, nil
	}
	return c.download(ctx, rawURL, sha256hex, finalPath)
}

// DownloadJar is a stand-alone convenience entry point for callers
// that don't want to build a JarCache struct. It is the function the
// agent-F spec names explicitly.
func DownloadJar(ctx context.Context, client HTTPDoer, rawURL, sha256hex, destDir string, allowedHosts []string) (string, error) {
	c := &JarCache{DestDir: destDir, Client: client, Allowed: allowedHosts}
	return c.Ensure(ctx, rawURL, sha256hex)
}

func (c *JarCache) download(ctx context.Context, rawURL, sha256hex, finalPath string) (string, error) {
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get jar: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get jar: status %d", resp.StatusCode)
	}
	// Same directory as final so rename is atomic.
	tmp, err := os.CreateTemp(c.DestDir, ".planetiler.*.jar.tmp")
	if err != nil {
		return "", fmt.Errorf("temp jar: %w", err)
	}
	tmpPath := tmp.Name()
	hasher := sha256.New()
	var reader io.Reader = resp.Body
	if c.MaxBytes > 0 {
		reader = io.LimitReader(resp.Body, c.MaxBytes+1)
	}
	n, copyErr := io.Copy(io.MultiWriter(tmp, hasher), reader)
	_ = tmp.Sync()
	_ = tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("stream jar: %w", copyErr)
	}
	if c.MaxBytes > 0 && n > c.MaxBytes {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("jar exceeds MaxBytes (%d)", c.MaxBytes)
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	want := strings.ToLower(strings.TrimSpace(sha256hex))
	if got != want {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("%w: got %s want %s", ErrJarSHAMismatch, got, want)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("rename jar: %w", err)
	}
	return finalPath, nil
}

// hostAllowed returns true when h (with or without :port) matches any
// entry in the allow list (case-insensitive, exact match on host).
func hostAllowed(h string, allowed []string) bool {
	host := strings.ToLower(h)
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return false
	}
	for _, a := range allowed {
		if strings.EqualFold(strings.TrimSpace(a), host) {
			return true
		}
	}
	return false
}

// validateSHA ensures the hex digest is well-formed (64 hex chars).
func validateSHA(s string) error {
	s = strings.TrimSpace(s)
	if len(s) != 64 {
		return fmt.Errorf("sha256 must be 64 hex chars, got %d", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		return fmt.Errorf("sha256 not hex: %w", err)
	}
	return nil
}

// fileMatchesSHA returns true iff path exists and its content hashes
// to sha256hex. Missing files return (false, nil); I/O errors return
// (false, err).
func fileMatchesSHA(path, sha256hex string) (bool, error) {
	f, err := os.Open(path) // #nosec G304 -- path is cache-internal
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	got := hex.EncodeToString(h.Sum(nil))
	return strings.EqualFold(got, strings.TrimSpace(sha256hex)), nil
}
