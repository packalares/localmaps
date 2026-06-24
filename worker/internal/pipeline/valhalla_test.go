// Package pipeline — valhalla_test exercises ValhallaRunner against a
// small shell-script fake that mimics the stderr progress pattern of
// `valhalla_build_*`. No real valhalla binary is involved.
package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturingReporter collects every Report call for test assertions.
// Implements the ProgressReporter interface from progress.go.
type capturingReporter struct {
	mu      sync.Mutex
	records []progressEvent
}

type progressEvent struct {
	Stage    string
	Fraction float64
	Message  string
}

func (c *capturingReporter) Report(_ context.Context, stage string, fraction float64, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, progressEvent{Stage: stage, Fraction: fraction, Message: message})
	return nil
}

func (c *capturingReporter) events() []progressEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]progressEvent, len(c.records))
	copy(out, c.records)
	return out
}

func fakeScript(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	return filepath.Join(dir, "testdata", "valhalla", "fake-valhalla-build.sh")
}

// buildValhallaPaths returns a set of paths under a per-test temp dir
// so the runner's config-file write does not touch the real tree.
func buildValhallaPaths(t *testing.T) RegionPaths {
	t.Helper()
	dir := t.TempDir()
	return RegionPaths{
		PbfPath:    filepath.Join(dir, "source.osm.pbf"),
		TileDir:    filepath.Join(dir, "valhalla_tiles"),
		TarPath:    filepath.Join(dir, "valhalla_tiles.tar"),
		AdminDB:    filepath.Join(dir, "valhalla_admin.sqlite"),
		TimezoneDB: filepath.Join(dir, "valhalla_timezones.sqlite"),
		Root:       dir,
		RegionKey:  "test-region",
	}
}

func TestValhallaRunnerSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell script is POSIX-only")
	}
	paths := buildValhallaPaths(t)
	script := fakeScript(t)
	execs := map[string]string{
		"valhalla_build_admins":    script,
		"valhalla_build_timezones": script,
		"valhalla_build_tiles":     script,
		"valhalla_build_extract":   script,
	}
	r := NewValhallaRunner(zerolog.Nop(),
		ValhallaRuntimeConfig{Concurrency: 1, BuildTimeoutMin: 1},
		WithValhallaExecutables(execs))

	rep := &capturingReporter{}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := r.Run(ctx, "test-region", paths, rep)
	require.NoError(t, err)

	// At minimum we should see the four step "starting" events plus the
	// four "done" events — and some progress updates from the fake.
	stages := map[string]int{}
	for _, e := range rep.events() {
		stages[e.Stage]++
	}
	for _, s := range []string{"valhalla.admins", "valhalla.timezones",
		"valhalla.tiles", "valhalla.extract"} {
		assert.Greaterf(t, stages[s], 1,
			"expected multiple progress events for %s, got %d", s, stages[s])
	}
}

func TestValhallaRunnerFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell script is POSIX-only")
	}
	paths := buildValhallaPaths(t)
	script := fakeScript(t)
	execs := map[string]string{
		"valhalla_build_admins":    script,
		"valhalla_build_timezones": script,
		"valhalla_build_tiles":     script,
		"valhalla_build_extract":   script,
	}
	r := NewValhallaRunner(zerolog.Nop(),
		ValhallaRuntimeConfig{Concurrency: 1, BuildTimeoutMin: 1},
		WithValhallaExecutables(execs))

	// The fake script respects FAKE_VALHALLA_FAIL=1 via its env. We can't
	// set an arbitrary env here without adding extra Option surface; use
	// a failing wrapper script instead.
	failScript := writeFailScript(t)
	execs["valhalla_build_admins"] = failScript
	r = NewValhallaRunner(zerolog.Nop(),
		ValhallaRuntimeConfig{Concurrency: 1, BuildTimeoutMin: 1},
		WithValhallaExecutables(execs))

	err := r.Run(context.Background(), "test-region", paths, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valhalla: admins failed")
	assert.Contains(t, err.Error(), "stderr tail:")
}

func TestValhallaRunnerContextCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell script is POSIX-only")
	}
	paths := buildValhallaPaths(t)
	// Use a long-running script so we can cancel mid-run.
	slow := writeSlowScript(t)
	execs := map[string]string{
		"valhalla_build_admins":    slow,
		"valhalla_build_timezones": slow,
		"valhalla_build_tiles":     slow,
		"valhalla_build_extract":   slow,
	}
	r := NewValhallaRunner(zerolog.Nop(),
		ValhallaRuntimeConfig{Concurrency: 1, BuildTimeoutMin: 1},
		WithValhallaExecutables(execs))

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	err := r.Run(ctx, "test-region", paths, nil)
	require.Error(t, err)
}

// writeFailScript drops a tiny stub that prints a stderr line and
// exits 1. Kept inline so tests don't need a second testdata file.
func writeFailScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "fail.sh")
	writeExecutable(t, p,
		"#!/usr/bin/env bash\necho 'fake failure' 1>&2\nexit 1\n")
	return p
}

// writeSlowScript drops a stub that sleeps for 10 s so the test can
// cancel it after a short delay.
func writeSlowScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "slow.sh")
	writeExecutable(t, p,
		"#!/usr/bin/env bash\nfor i in 1 2 3 4 5 6 7 8 9 10; do echo tick 1>&2; sleep 1; done\n")
	return p
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
}
