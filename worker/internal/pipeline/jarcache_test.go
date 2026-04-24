package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// tinyJar is a deterministic payload used to stand in for a real
// planetiler jar. Content doesn't matter — only the SHA match does.
var tinyJar = []byte("PLANETILER-FAKE-JAR\x00\x01\x02")

func tinyJarSHA() string {
	h := sha256.Sum256(tinyJar)
	return hex.EncodeToString(h[:])
}

// jarServer returns an httptest.Server serving tinyJar and the host
// name (no port) for allow-listing.
func jarServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tinyJar)
	}))
	u, _ := url.Parse(s.URL)
	host := u.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return s, host
}

func TestDownloadJarHappyPath(t *testing.T) {
	srv, host := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	path, err := DownloadJar(context.Background(), nil, srv.URL, tinyJarSHA(), dir, []string{host})
	require.NoError(t, err)
	require.FileExists(t, path)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, tinyJar, b)
}

func TestDownloadJarSHAMismatch(t *testing.T) {
	srv, host := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	bad := strings.Repeat("0", 64)
	_, err := DownloadJar(context.Background(), nil, srv.URL, bad, dir, []string{host})
	require.ErrorIs(t, err, ErrJarSHAMismatch)
	// Temp file must not be left behind on mismatch.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestDownloadJarEgressDenied(t *testing.T) {
	srv, _ := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	_, err := DownloadJar(context.Background(), nil, srv.URL, tinyJarSHA(), dir, []string{"not-the-host.invalid"})
	require.ErrorIs(t, err, ErrEgressDenied)
}

func TestDownloadJarAllowListMissing(t *testing.T) {
	srv, _ := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	_, err := DownloadJar(context.Background(), nil, srv.URL, tinyJarSHA(), dir, nil)
	require.ErrorIs(t, err, ErrEgressAllowListMissing)
}

func TestJarCacheHitSkipsDownload(t *testing.T) {
	srv, host := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	ctx := context.Background()
	sha := tinyJarSHA()

	path1, err := DownloadJar(ctx, nil, srv.URL, sha, dir, []string{host})
	require.NoError(t, err)

	// Swap the server out — a second Ensure must not hit the network.
	srv.Close()

	// Use a client that always fails — proves cache hit.
	failClient := &failingClient{}
	c := &JarCache{DestDir: dir, Client: failClient, Allowed: []string{host}}
	path2, err := c.Ensure(ctx, srv.URL, sha)
	require.NoError(t, err)
	require.Equal(t, path1, path2)
	require.Zero(t, failClient.calls, "expected no HTTP call on cache hit")
}

func TestDownloadJarBadSHAHex(t *testing.T) {
	dir := t.TempDir()
	_, err := DownloadJar(context.Background(), nil, "https://example.com/jar", "nothex", dir, []string{"example.com"})
	require.Error(t, err)
}

func TestDownloadJar404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	dir := t.TempDir()
	_, err := DownloadJar(context.Background(), nil, srv.URL, tinyJarSHA(), dir, []string{host})
	require.Error(t, err)
}

func TestDownloadJarMaxBytes(t *testing.T) {
	srv, host := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	cache := &JarCache{DestDir: dir, Allowed: []string{host}, MaxBytes: 5}
	_, err := cache.Ensure(context.Background(), srv.URL, tinyJarSHA())
	require.Error(t, err)
	require.Contains(t, err.Error(), "MaxBytes")
}

func TestDownloadJarHostAllowListMatchesPort(t *testing.T) {
	srv, host := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	// Allow-list entry is the bare host; the server URL has :port.
	_, err := DownloadJar(context.Background(), nil, srv.URL, tinyJarSHA(), dir, []string{host})
	require.NoError(t, err)
}

// Final sanity: cached file that's been tampered with triggers a fresh
// download + re-verify.
func TestJarCacheTamperedHit(t *testing.T) {
	srv, host := jarServer(t)
	defer srv.Close()
	dir := t.TempDir()
	ctx := context.Background()
	sha := tinyJarSHA()
	path, err := DownloadJar(ctx, nil, srv.URL, sha, dir, []string{host})
	require.NoError(t, err)

	// Corrupt the cached jar.
	require.NoError(t, os.WriteFile(path, []byte("tampered"), 0o644))

	path2, err := DownloadJar(ctx, nil, srv.URL, sha, dir, []string{host})
	require.NoError(t, err)
	require.Equal(t, path, path2)
	got, err := os.ReadFile(path2)
	require.NoError(t, err)
	require.Equal(t, tinyJar, got, "cache should have been re-downloaded")
}

type failingClient struct{ calls int }

func (f *failingClient) Do(*http.Request) (*http.Response, error) {
	f.calls++
	return nil, errors.New("test: client should not be called")
}
