// Package pipeline — valhalla build runner.
//
// ValhallaRunner orchestrates the canonical four-step valhalla tile
// build as documented by upstream Valhalla / docs/02-stack.md:
//
//  1. valhalla_build_admins     → admin.sqlite
//  2. valhalla_build_timezones  → timezones.sqlite
//  3. valhalla_build_tiles      → tile directory
//  4. valhalla_build_extract -v → tar archive for mmap'd serving
//
// Each step is an exec'd child process. stderr is line-parsed for
// progress; ctx cancellation SIGTERMs children with a 10s grace
// before SIGKILL (see docs/08-security.md — no shell injection, no
// shell invocation).
//
// The ProgressReporter interface lives in progress.go (Agent F). We
// pass the runner's context into reporter.Report so F's AsynqProgress
// can write to the jobs row with the right cancellation semantics.
package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ValhallaRunner builds routing tiles for one region. Instances are
// cheap; construct per-job.
type ValhallaRunner struct {
	logger zerolog.Logger
	cfg    ValhallaRuntimeConfig
	// execPath maps logical tool name → absolute binary path. Empty
	// means "look up on PATH". Tests populate this to inject fakes.
	execPath map[string]string
}

// ValhallaOption configures a ValhallaRunner at construction.
type ValhallaOption func(*ValhallaRunner)

// WithValhallaExecutables overrides the binaries used for each build
// step. Missing keys fall back to PATH lookup. Primary use: tests
// inject a fake `valhalla_build_tiles` shell script.
func WithValhallaExecutables(m map[string]string) ValhallaOption {
	return func(r *ValhallaRunner) {
		if r.execPath == nil {
			r.execPath = map[string]string{}
		}
		for k, v := range m {
			r.execPath[k] = v
		}
	}
}

// NewValhallaRunner builds a runner. logger may be zerolog.Nop(); cfg
// supplies runtime knobs (concurrency, timeout, extra args). If
// concurrency is 0 the runner defaults to 2 (see NEEDED: these
// settings are not yet in docs/07-config-schema.md).
func NewValhallaRunner(logger zerolog.Logger, cfg ValhallaRuntimeConfig, opts ...ValhallaOption) *ValhallaRunner {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 2
	}
	if cfg.BuildTimeoutMin <= 0 {
		cfg.BuildTimeoutMin = 60
	}
	r := &ValhallaRunner{logger: logger, cfg: cfg}
	for _, o := range opts {
		o(r)
	}
	return r
}

// step defines a single child-process invocation in the four-step
// build chain. fracStart/fracEnd map step-local 0..1 progress onto
// the overall job progress range.
type step struct {
	name      string
	tool      string
	args      []string
	fracStart float64
	fracEnd   float64
}

// Run executes the four build steps in sequence. On any failure it
// returns a wrapped error whose tail includes the last ~32 KiB of
// stderr from the failed child. ctx cancellation terminates the
// current child and aborts the chain.
func (r *ValhallaRunner) Run(ctx context.Context, region string, paths RegionPaths, reporter ProgressReporter) error {
	if reporter == nil {
		reporter = DiscardProgress{}
	}
	if err := validateValhallaPaths(paths); err != nil {
		return err
	}
	cfgBytes, err := GenerateConfig(region, paths, r.cfg)
	if err != nil {
		return fmt.Errorf("valhalla: generate config: %w", err)
	}
	cfgPath, cleanup, err := writeTempValhallaConfig(paths.TileDir, cfgBytes)
	if err != nil {
		return fmt.Errorf("valhalla: write config: %w", err)
	}
	defer cleanup()

	// Slices: admins 10 %, timezones 5 %, tiles 70 %, extract 15 %.
	steps := []step{
		{name: "admins", tool: "valhalla_build_admins",
			args: []string{"--config", cfgPath, paths.PbfPath},
			fracStart: 0.00, fracEnd: 0.10},
		{name: "timezones", tool: "valhalla_build_timezones",
			args: []string{"--config", cfgPath},
			fracStart: 0.10, fracEnd: 0.15},
		{name: "tiles", tool: "valhalla_build_tiles",
			args: []string{"--config", cfgPath, paths.PbfPath},
			fracStart: 0.15, fracEnd: 0.85},
		{name: "extract", tool: "valhalla_build_extract",
			// --overwrite: a retry run hits an existing .tar from the
			// previous build; extract refuses without this flag.
			args: []string{"--config", cfgPath, "--overwrite", "-v"},
			fracStart: 0.85, fracEnd: 1.00},
	}

	overall, cancel := context.WithTimeout(ctx,
		time.Duration(r.cfg.BuildTimeoutMin)*time.Minute)
	defer cancel()

	for _, s := range steps {
		if err := r.runStep(overall, s, reporter); err != nil {
			return err
		}
		_ = reporter.Report(overall, "valhalla."+s.name, s.fracEnd, s.name+" done")
	}
	return nil
}

// runStep execs one build tool. stderr is scanned line-by-line for
// progress percentages; unrecognised lines tick the heartbeat so the
// UI still sees motion on quiet tools.
func (r *ValhallaRunner) runStep(ctx context.Context, s step, reporter ProgressReporter) error {
	bin := r.binary(s.tool)
	args := append([]string{}, s.args...)
	args = append(args, r.cfg.ExtraArgs...)

	cmd := exec.CommandContext(ctx, bin, args...) // #nosec G204 — argv is config-driven
	// SIGTERM first with a 10s grace before Go's CommandContext sends
	// SIGKILL (Go 1.22+).
	cmd.Cancel = func() error { return cmd.Process.Signal(sigterm) }
	cmd.WaitDelay = 10 * time.Second

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("valhalla: %s stderr pipe: %w", s.name, err)
	}
	cmd.Stdout = io.Discard

	r.logger.Info().Str("step", s.name).Str("bin", bin).
		Strs("args", args).Msg("valhalla step starting")
	_ = reporter.Report(ctx, "valhalla."+s.name, s.fracStart, s.name+" starting")

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("valhalla: %s start: %w", s.name, err)
	}

	tail := newTailBuffer(32 * 1024)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		valhallaScanProgress(ctx, stderr, tail, s, reporter, r.logger)
	}()
	waitErr := cmd.Wait()
	wg.Wait()

	if waitErr != nil {
		return fmt.Errorf("valhalla: %s failed: %w; stderr tail: %s",
			s.name, waitErr, tail.String())
	}
	return nil
}

// binary resolves a tool name to a command. Overrides from
// WithValhallaExecutables win; otherwise PATH lookup is left to exec.
func (r *ValhallaRunner) binary(tool string) string {
	if r.execPath != nil {
		if p, ok := r.execPath[tool]; ok && p != "" {
			return p
		}
	}
	return tool
}

// writeTempValhallaConfig drops the valhalla.json bytes next to the
// tile dir under a random name and returns a cleanup func. The
// per-region dir is pre-created by the caller.
func writeTempValhallaConfig(tileDir string, data []byte) (string, func(), error) {
	parent := filepath.Dir(tileDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", func() {}, err
	}
	f, err := os.CreateTemp(parent, "valhalla-*.json")
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", func() {}, err
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}
