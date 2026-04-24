package pipeline

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// fakePlanetilerScript resolves the testdata shell script path.
func fakePlanetilerScript(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(filename), "testdata", "planetiler", "fake-planetiler.sh")
}

// shellCommandFactory returns a CommandFactory that invokes sh with the
// fake script instead of java. The first CLI arg determines behaviour.
func shellCommandFactory(script, mode string) CommandFactory {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", script, mode)
	}
}

// captureReporter is declared in pelias_test.go (package-shared); we
// reuse it so the test suite stays DRY.

func TestNewPlanetilerRunnerValidates(t *testing.T) {
	_, err := NewPlanetilerRunner(PlanetilerConfig{}, zerolog.Nop())
	require.Error(t, err)
	r, err := NewPlanetilerRunner(PlanetilerConfig{JarPath: "/tmp/x.jar"}, zerolog.Nop())
	require.NoError(t, err)
	require.Equal(t, 4096, r.Cfg.MemoryMB)
	require.Equal(t, 10*time.Second, r.Cfg.TerminateGracePeriod)
}

func TestPlanetilerRunSuccess(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no /bin/sh available")
	}
	script := fakePlanetilerScript(t)
	rep := &captureReporter{}
	r, err := NewPlanetilerRunner(PlanetilerConfig{
		JarPath:              "/fake.jar", // unused by fake; required by validator
		MemoryMB:             64,
		TerminateGracePeriod: 2 * time.Second,
	}, zerolog.Nop())
	require.NoError(t, err)
	r.Command = shellCommandFactory(script, "ok")

	tmp := t.TempDir()
	pbf := filepath.Join(tmp, "source.osm.pbf")
	require.NoError(t, os.WriteFile(pbf, []byte("fake"), 0o644))
	out := filepath.Join(tmp, "map.pmtiles")

	require.NoError(t, r.Run(context.Background(), pbf, out, rep))

	_, seen, _ := rep.snapshot()
	require.NotEmpty(t, seen)
	// Monotonic invariant: fractions never decrease.
	last := -1.0
	for _, f := range seen {
		require.GreaterOrEqual(t, f, last, "fractions must be monotonic")
		last = f
	}
	require.Equal(t, 1.0, seen[len(seen)-1])
}

func TestPlanetilerRunNonZeroExit(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no /bin/sh available")
	}
	script := fakePlanetilerScript(t)
	r, err := NewPlanetilerRunner(PlanetilerConfig{JarPath: "/fake.jar"}, zerolog.Nop())
	require.NoError(t, err)
	r.Command = shellCommandFactory(script, "fail")

	err = r.Run(context.Background(), "/does/not/matter", "/tmp/out", DiscardProgress{})
	var pe *PlanetilerError
	require.ErrorAs(t, err, &pe)
	require.Equal(t, 2, pe.ExitCode)
	require.Contains(t, pe.TailStderr, "boom")
}

func TestPlanetilerRunContextCancel(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no /bin/sh available")
	}
	script := fakePlanetilerScript(t)
	r, err := NewPlanetilerRunner(PlanetilerConfig{
		JarPath:              "/fake.jar",
		TerminateGracePeriod: 500 * time.Millisecond,
	}, zerolog.Nop())
	require.NoError(t, err)
	r.Command = shellCommandFactory(script, "hang")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- r.Run(ctx, "/x.pbf", "/x.pmtiles", DiscardProgress{})
	}()
	// Give the child time to spawn, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		// Expect some error (cancellation propagates as PlanetilerError or wrapped ctx).
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not return after ctx cancel")
	}
}

func TestPlanetilerRunStartFails(t *testing.T) {
	// Make a runner whose CommandFactory returns a command pointing at a
	// binary that doesn't exist — exercises the start-error path.
	r, err := NewPlanetilerRunner(PlanetilerConfig{JarPath: "/fake.jar"}, zerolog.Nop())
	require.NoError(t, err)
	r.Command = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/this/command/does/not/exist")
	}
	err = r.Run(context.Background(), "/x", "/y", DiscardProgress{})
	require.Error(t, err)
}

func TestBuildArgsIncludesExtra(t *testing.T) {
	r, err := NewPlanetilerRunner(PlanetilerConfig{
		JarPath:   "/tools/planetiler.jar",
		MemoryMB:  2048,
		ExtraArgs: []string{"--verbose"},
	}, zerolog.Nop())
	require.NoError(t, err)
	a := r.buildArgs("/in.pbf", "/out.pmtiles")
	require.Contains(t, a, "--verbose")
	require.Contains(t, a, "-Xmx2048m")
	require.Contains(t, a, "--osm-path=/in.pbf")
	require.Contains(t, a, "--output=/out.pmtiles")
	require.Contains(t, a, "--download=false")
}

// TestPlanetilerErrorImplementsError is a compile-time check that
// PlanetilerError satisfies `error` (guards against accidental removal
// of the receiver signature).
func TestPlanetilerErrorImplementsError(t *testing.T) {
	var err error = &PlanetilerError{ExitCode: 7, TailStderr: "stuff"}
	require.ErrorContains(t, err, "7")
	var pe *PlanetilerError
	require.True(t, errors.As(err, &pe))
}
