package pipeline

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// captureReporter is a thread-safe ProgressReporter for tests. Signature
// matches progress.go's ProgressReporter (owned by Agent F).
type captureReporter struct {
	mu     sync.Mutex
	stages []string
	seen   []float64
	msgs   []string
}

func (c *captureReporter) Report(_ context.Context, stage string, frac float64, msg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stages = append(c.stages, stage)
	c.seen = append(c.seen, frac)
	c.msgs = append(c.msgs, msg)
	return nil
}

func (c *captureReporter) snapshot() ([]string, []float64, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.stages...),
		append([]float64(nil), c.seen...),
		append([]string(nil), c.msgs...)
}

func skipIfNoBash(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-importer.sh requires a POSIX shell")
	}
}

func fakeImporterPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "pelias", "fake-importer.sh"))
	require.NoError(t, err)
	require.FileExists(t, abs)
	return abs
}

func newRunner(t *testing.T, workDir string, argv []string) *PeliasRunner {
	t.Helper()
	return &PeliasRunner{
		Logger: zerolog.New(io.Discard),
		Config: ImportConfig{
			Region:    "europe-romania",
			PbfPath:   "/data/source.osm.pbf",
			ESHost:    "pelias-es",
			ESPort:    9200,
			IndexName: "pelias-europe-romania-20260424",
			Languages: []string{"en"},
		},
		Executables:  map[string][]string{importerKey: argv},
		BuildTimeout: 30 * time.Second,
		WorkDir:      workDir,
	}
}

func TestPeliasRunner_ProgressFractions(t *testing.T) {
	skipIfNoBash(t)
	t.Parallel()
	dir := t.TempDir()
	runner := newRunner(t, dir, []string{"bash", fakeImporterPath(t)})
	rep := &captureReporter{}

	err := runner.Run(
		context.Background(),
		RegionPaths{PbfPath: "/data/source.osm.pbf"},
		rep,
	)
	require.NoError(t, err)

	stages, fracs, msgs := rep.snapshot()
	require.NotEmpty(t, fracs, "reporter must receive at least one progress update")
	for _, s := range stages {
		require.Equal(t, peliasStage, s, "every event must be tagged with the pelias stage")
	}

	// Fractions monotonically non-decreasing, final == 1.0 (completion
	// message emitted on success).
	for i := 1; i < len(fracs); i++ {
		require.GreaterOrEqual(t, fracs[i], fracs[i-1],
			"fractions must be monotonic; got %v", fracs)
	}
	require.InDelta(t, 1.0, fracs[len(fracs)-1], 1e-9)
	require.Contains(t, msgs[len(msgs)-1], "complete")

	// pelias.json actually written
	cfgPath := filepath.Join(dir, "pelias.json")
	info, err := os.Stat(cfgPath)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}

func TestPeliasRunner_PropagatesImporterFailure(t *testing.T) {
	skipIfNoBash(t)
	t.Parallel()
	dir := t.TempDir()
	runner := newRunner(t, dir, []string{
		"bash", "-c",
		"PELIAS_FAIL=1 " + fakeImporterPath(t),
	})
	err := runner.Run(
		context.Background(),
		RegionPaths{PbfPath: "/data/source.osm.pbf"},
		&captureReporter{},
	)
	require.Error(t, err)
	require.True(t,
		strings.Contains(err.Error(), "pelias import"),
		"error must be wrapped with pelias prefix, got %v", err)
}

func TestPeliasRunner_RejectsMissingExecutable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &PeliasRunner{
		Logger: zerolog.New(io.Discard),
		Config: ImportConfig{
			Region: "europe-romania", PbfPath: "/data/source.osm.pbf",
			ESHost: "pelias-es", ESPort: 9200,
			IndexName: "pelias-europe-romania-20260424",
			Languages: []string{"en"},
		},
		Executables: map[string][]string{},
		WorkDir:     dir,
	}
	err := r.Run(context.Background(), RegionPaths{}, &captureReporter{})
	require.Error(t, err)
	require.Contains(t, err.Error(), importerKey)
}

func TestPeliasRunner_RejectsMissingWorkDir(t *testing.T) {
	t.Parallel()
	r := &PeliasRunner{
		Logger:      zerolog.New(io.Discard),
		Config:      ImportConfig{Region: "r", PbfPath: "/p", ESHost: "h", ESPort: 9200, IndexName: "i"},
		Executables: map[string][]string{importerKey: {"/bin/true"}},
	}
	err := r.Run(context.Background(), RegionPaths{}, &captureReporter{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "WorkDir")
}

func TestPeliasScanProgress_ParsesNM(t *testing.T) {
	t.Parallel()
	input := strings.NewReader(
		"junk line\n" +
			"[INFO] openstreetmap: imported 10/100\n" +
			"noise\n" +
			"[INFO] openstreetmap: imported 55/100\n" +
			"[INFO] openstreetmap: imported 100/100\n",
	)
	rep := &captureReporter{}
	peliasScanProgress(context.Background(), input, rep, zerolog.New(io.Discard))
	_, fracs, _ := rep.snapshot()
	require.Equal(t, []float64{0.1, 0.55, 1.0}, fracs)
}

func TestPeliasScanProgress_IgnoresDivByZero(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("[INFO] openstreetmap: imported 5/0\n")
	rep := &captureReporter{}
	peliasScanProgress(context.Background(), input, rep, zerolog.New(io.Discard))
	_, fracs, _ := rep.snapshot()
	require.Empty(t, fracs)
}

func TestWriteFileAtomic_CreatesDirAndFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "pelias.json")
	require.NoError(t, writeFileAtomic(target, []byte(`{"ok":true}`)))
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, string(got))
}
