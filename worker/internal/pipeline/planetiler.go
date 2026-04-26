package pipeline

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

// PlanetilerError wraps a non-zero planetiler exit; TailStderr holds
// the last ~4 KiB of stderr (operator log has the full stream).
type PlanetilerError struct {
	ExitCode   int
	TailStderr string
}

// Error implements the error interface.
func (e *PlanetilerError) Error() string {
	return fmt.Sprintf("planetiler exited %d: %s", e.ExitCode, e.TailStderr)
}

// PlanetilerConfig is the subset of tiles.* settings this runner reads.
// Per docs/07-config-schema.md — the planetiler* keys are NEEDED
// (agent-F report).
type PlanetilerConfig struct {
	JarPath              string
	MemoryMB             int
	ExtraArgs            []string
	MaxDuration          time.Duration // 0 = no timeout
	StderrTailBytes      int           // 0 → 4096
	TerminateGracePeriod time.Duration // 0 → 10s
}

// PlanetilerRunner spawns `java -jar planetiler.jar …` and streams its
// output, translating progress lines into ProgressReporter calls.
type PlanetilerRunner struct {
	Cfg     PlanetilerConfig
	Logger  zerolog.Logger
	Command CommandFactory
}

// CommandFactory builds the subprocess. Tests inject a factory that
// returns a tiny shell script so no JVM is required in CI.
type CommandFactory func(ctx context.Context, name string, args ...string) *exec.Cmd

// NewPlanetilerRunner validates cfg and returns a runner. JarPath is
// required; MemoryMB defaults to 4096 (planetiler's country-scale min).
func NewPlanetilerRunner(cfg PlanetilerConfig, log zerolog.Logger) (*PlanetilerRunner, error) {
	if strings.TrimSpace(cfg.JarPath) == "" {
		return nil, errors.New("planetiler: empty JarPath")
	}
	if cfg.MemoryMB <= 0 {
		cfg.MemoryMB = 4096
	}
	if cfg.StderrTailBytes <= 0 {
		cfg.StderrTailBytes = 4096
	}
	if cfg.TerminateGracePeriod <= 0 {
		cfg.TerminateGracePeriod = 10 * time.Second
	}
	return &PlanetilerRunner{Cfg: cfg, Logger: log}, nil
}

// Run executes planetiler on pbfPath → outPmtilesPath. ctx cancellation
// kills the child (SIGTERM, then SIGKILL after TerminateGracePeriod).
func (r *PlanetilerRunner) Run(ctx context.Context, pbfPath, outPmtilesPath string, progress ProgressReporter) error {
	if progress == nil {
		progress = DiscardProgress{}
	}
	if r.Cfg.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Cfg.MaxDuration)
		defer cancel()
	}
	factory := r.Command
	if factory == nil {
		factory = realCommandFactory
	}
	cmd := factory(ctx, "java", r.buildArgs(pbfPath, outPmtilesPath)...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("planetiler stdout: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("planetiler stderr: %w", err)
	}
	r.Logger.Info().Str("jar", r.Cfg.JarPath).Int("memMB", r.Cfg.MemoryMB).
		Str("pbf", pbfPath).Str("out", outPmtilesPath).Msg("planetiler starting")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("planetiler start: %w", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); r.streamStdout(stdoutPipe) }()
	tail := newPlanetilerTail(r.Cfg.StderrTailBytes)
	parser := newProgressParser()
	go func() { defer wg.Done(); r.streamStderr(ctx, stderrPipe, tail, parser, progress) }()
	waitErr := r.waitWithCancel(ctx, cmd)
	wg.Wait()
	if waitErr != nil {
		code := -1
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			code = ee.ExitCode()
		}
		return &PlanetilerError{ExitCode: code, TailStderr: tail.String()}
	}
	_ = progress.Report(ctx, "planetiler.done", 1.0, "planetiler complete")
	return nil
}

func (r *PlanetilerRunner) buildArgs(pbfPath, out string) []string {
	a := []string{
		fmt.Sprintf("-Xmx%dm", r.Cfg.MemoryMB),
		"-jar", r.Cfg.JarPath,
		"--osm-path=" + pbfPath, "--output=" + out,
		// Planetiler needs aux sources (lake_centerline, natural_earth,
		// water_polygons). --download=true fetches them on first run and
		// reuses the cache under $PWD/data/sources afterwards.
		"--download=true", "--force",
	}
	return append(a, r.Cfg.ExtraArgs...)
}

func (r *PlanetilerRunner) streamStdout(rd io.Reader) {
	s := bufio.NewScanner(rd)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	for s.Scan() {
		r.Logger.Info().Str("src", "planetiler.stdout").Msg(s.Text())
	}
}

func (r *PlanetilerRunner) streamStderr(ctx context.Context, rd io.Reader, tail *planetilerTail, parser *progressParser, reporter ProgressReporter) {
	s := bufio.NewScanner(rd)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	for s.Scan() {
		line := s.Text()
		_, _ = tail.Write([]byte(line + "\n"))
		r.Logger.Info().Str("src", "planetiler.stderr").Msg(line)
		if stage, frac, ok := parser.Parse(line); ok {
			_ = reporter.Report(ctx, stage, frac, line)
		}
	}
}

func (r *PlanetilerRunner) waitWithCancel(ctx context.Context, cmd *exec.Cmd) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Signal(planetilerSigterm)
		}
		select {
		case err := <-done:
			return err
		case <-time.After(r.Cfg.TerminateGracePeriod):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return <-done
		}
	}
}

func realCommandFactory(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...) // #nosec G204 -- argv is config-driven
}

var planetilerSigterm = syscall.SIGTERM

type planetilerTail struct {
	mu   sync.Mutex
	buf  []byte
	size int
}

func newPlanetilerTail(n int) *planetilerTail {
	if n <= 0 {
		n = 4096
	}
	return &planetilerTail{size: n}
}

func (t *planetilerTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.size {
		t.buf = t.buf[len(t.buf)-t.size:]
	}
	return len(p), nil
}

func (t *planetilerTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
