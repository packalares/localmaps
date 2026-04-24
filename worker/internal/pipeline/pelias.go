// Package pipeline — pelias.go runs the Pelias openstreetmap importer
// against a freshly-downloaded .osm.pbf, producing ES-indexed data on
// the shared pelias-es Elasticsearch instance.
//
// Strategy: the importer is pre-wired in deploy/docker-compose.yml as a
// sidecar service. This runner shells out via `docker compose exec` so
// the Go binary stays free of Node/Java/ES dependencies. The
// `Executables` map lets tests inject stub scripts (see
// testdata/pelias/fake-importer.sh) without touching docker.
//
// Progress is parsed out of stderr: upstream emits lines like
//
//	[INFO] openstreetmap: imported 123/10000
//
// We convert the N/M ratio to a 0..1 float and forward it to the
// ProgressReporter owned by progress.go (Agent F). No ES client calls
// happen here — Go just orchestrates the process.
package pipeline

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// PeliasRunner orchestrates one Pelias import run. Construct directly;
// the zero value is not usable — Logger, Executables, and WorkDir are
// mandatory.
type PeliasRunner struct {
	// Logger is used for structured diagnostics. Required.
	Logger zerolog.Logger
	// Config is the prepared ImportConfig for this run. Region + ES
	// host/port are authoritative; PbfPath inside the container
	// defaults to "/data/source.osm.pbf" if empty.
	Config ImportConfig
	// Executables maps a logical command name to the argv slice to
	// run. Production uses "importer" → ["docker", "compose", "-f",
	// "/deploy/docker-compose.yml", "exec", "-T", "pelias-importer",
	// "./bin/start"]. Tests inject a local shell script via the same
	// key.
	Executables map[string][]string
	// BuildTimeout caps the import run. Zero means the spec default
	// (120 minutes — `search.peliasBuildTimeoutMinutes`).
	BuildTimeout time.Duration
	// WorkDir is where the generated pelias.json is written. Typically
	// /data/tools/pelias-runs/<jobID>/. Caller creates it.
	WorkDir string
}

// importerKey is the Executables map entry for the importer invocation.
const importerKey = "importer"

// peliasStage is the stage label passed to ProgressReporter.Report so
// the UI can tell pelias progress apart from planetiler/valhalla.
const peliasStage = "pelias.openstreetmap"

// Run executes the full import: write pelias.json to WorkDir, invoke
// the importer, parse N/M progress lines off stderr into the reporter.
// Context cancellation terminates the child via SIGTERM; a 15s grace
// period is followed by SIGKILL.
//
// paths supplies the .osm.pbf absolute host path (logged for debug).
// The container-side path lives on Config.PbfPath. progress may be nil;
// nil is replaced with DiscardProgress.
func (r *PeliasRunner) Run(ctx context.Context, paths RegionPaths, progress ProgressReporter) error {
	if r.WorkDir == "" {
		return errors.New("pelias runner: WorkDir required")
	}
	if progress == nil {
		progress = DiscardProgress{}
	}
	argv, ok := r.Executables[importerKey]
	if !ok || len(argv) == 0 {
		return fmt.Errorf("pelias runner: no executable registered for %q", importerKey)
	}

	cfg := r.Config
	if cfg.PbfPath == "" {
		cfg.PbfPath = "/data/source.osm.pbf"
	}

	body, err := GeneratePeliasJSON(cfg)
	if err != nil {
		return fmt.Errorf("generate pelias.json: %w", err)
	}
	cfgPath := filepath.Join(r.WorkDir, "pelias.json")
	if err := writeFileAtomic(cfgPath, body); err != nil {
		return fmt.Errorf("write pelias.json: %w", err)
	}
	r.Logger.Info().
		Str("region", cfg.Region).
		Str("hostPbf", paths.PbfPath).
		Str("containerPbf", cfg.PbfPath).
		Str("configPath", cfgPath).
		Str("indexName", cfg.IndexName).
		Msg("pelias import starting")

	timeout := r.BuildTimeout
	if timeout <= 0 {
		timeout = 120 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 — argv is config-driven (operator-controlled), never
	// user-supplied. Subprocess rules per docs/08-security.md.
	cmd := exec.CommandContext(runCtx, argv[0], append([]string{}, argv[1:]...)...)
	cmd.Env = append(cmd.Env, "PELIAS_CONFIG="+cfgPath)
	cmd.Cancel = func() error { return cmd.Process.Signal(peliasSigterm) }
	cmd.WaitDelay = 15 * time.Second

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("pelias stderr pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pelias stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pelias start: %w", err)
	}

	_ = progress.Report(ctx, peliasStage, 0.0, "pelias import starting")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		peliasDrainStdout(stdout, r.Logger)
	}()
	go func() {
		defer wg.Done()
		peliasScanProgress(runCtx, stderr, progress, r.Logger)
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	if waitErr != nil {
		r.Logger.Error().Err(waitErr).
			Str("region", cfg.Region).
			Msg("pelias import failed")
		return fmt.Errorf("pelias import: %w", waitErr)
	}
	_ = progress.Report(ctx, peliasStage, 1.0, "pelias import complete")
	r.Logger.Info().Str("region", cfg.Region).Msg("pelias import complete")
	return nil
}

// peliasProgressLine matches upstream's "[<LEVEL>] openstreetmap:
// imported N/M" format. Tolerant to leading timestamps and alternative
// importer names (e.g. "polylines:").
var peliasProgressLine = regexp.MustCompile(`(?i)imported\s+(\d+)\s*/\s*(\d+)`)

// peliasScanProgress reads lines from r, matches the "imported N/M"
// pattern, and pushes fractions to reporter. Non-matching lines are
// logged at debug level. Named with a pelias prefix to avoid any
// collision with other runners' scanners.
func peliasScanProgress(ctx context.Context, r io.Reader, reporter ProgressReporter, log zerolog.Logger) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var lastFrac float64
	for sc.Scan() {
		line := sc.Text()
		m := peliasProgressLine.FindStringSubmatch(line)
		if m == nil {
			log.Debug().Str("line", line).Msg("pelias stderr")
			continue
		}
		n, _ := strconv.ParseFloat(m[1], 64)
		d, _ := strconv.ParseFloat(m[2], 64)
		if d <= 0 {
			continue
		}
		frac := n / d
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		if reporter != nil && frac-lastFrac >= 0.01 {
			_ = reporter.Report(ctx, peliasStage, frac,
				fmt.Sprintf("imported %d/%d", int(n), int(d)))
			lastFrac = frac
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		log.Warn().Err(err).Msg("pelias stderr scan error")
	}
}

func peliasDrainStdout(r io.Reader, log zerolog.Logger) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		log.Debug().Str("line", sc.Text()).Msg("pelias stdout")
	}
}
