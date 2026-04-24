// Package pipeline — valhalla-specific subprocess helpers.
//
// Splits three concerns out of valhalla.go to keep that file under the
// 250-line cap:
//
//   - tailBuffer: ring buffer that retains the last N bytes of a
//     child's stderr for inclusion in error messages.
//   - sigterm: the OS signal used to gracefully terminate children on
//     ctx cancellation before SIGKILL fires via exec.Cmd.WaitDelay.
//     (SIGTERM; the pelias / planetiler runners use their own prefixed
//     copies so names never collide.)
//   - valhallaScanProgress: stderr line-parser specific to the
//     valhalla tool family.
package pipeline

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

// sigterm is the signal exec.Cmd.Cancel sends to children on ctx
// cancellation. Declared as os.Signal so tests (or future Windows
// ports) can swap it. Kept here because planetiler.go and pelias.go
// both reference the symbol.
var sigterm os.Signal = syscall.SIGTERM

// tailBuffer is a fixed-size ring buffer of stderr bytes. The last
// `cap` bytes written are retained for inclusion in error messages.
// Supports both io.Writer (Write) and a convenience Append for
// string input — planetiler/pelias use Append, valhalla uses Write.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	cap int
}

func newTailBuffer(capacity int) *tailBuffer {
	if capacity <= 0 {
		capacity = 4096
	}
	return &tailBuffer{cap: capacity}
}

// Write appends bytes; oldest bytes are dropped when capacity is
// exceeded. Always returns len(p), nil.
func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.cap {
		t.buf = t.buf[len(t.buf)-t.cap:]
	}
	return len(p), nil
}

// Append is a string-friendly wrapper over Write.
func (t *tailBuffer) Append(s string) { _, _ = t.Write([]byte(s)) }

// String returns the retained tail as a string.
func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// valhallaPercentRE matches common valhalla progress patterns such as
// `[INFO] Processing foo 42%` or `tiles: 42 %`.
var valhallaPercentRE = regexp.MustCompile(`(\d{1,3})\s*%`)

// valhallaScanProgress reads stderr line-by-line, parses percentages
// where possible, and forwards events to reporter. A 5 s heartbeat
// forces a report even when tools stay silent — the UI needs motion.
// Uses F's ProgressReporter signature: Report(ctx, stage, frac, msg).
func valhallaScanProgress(ctx context.Context, r io.Reader, tail *tailBuffer, s step, reporter ProgressReporter, logger zerolog.Logger) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lastReport := time.Now()
	lastPct := -1.0
	for sc.Scan() {
		line := sc.Text()
		tail.Append(line + "\n")
		logger.Debug().Str("step", s.name).Msg(line)

		pct := -1.0
		if m := valhallaPercentRE.FindStringSubmatch(line); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil && v >= 0 && v <= 100 {
				pct = float64(v) / 100.0
			}
		}
		now := time.Now()
		if pct >= 0 && pct != lastPct {
			frac := s.fracStart + (s.fracEnd-s.fracStart)*pct
			_ = reporter.Report(ctx, "valhalla."+s.name, frac, line)
			lastReport, lastPct = now, pct
			continue
		}
		if now.Sub(lastReport) >= 5*time.Second {
			frac := s.fracStart + (s.fracEnd-s.fracStart)*lastPctOrZero(lastPct)
			_ = reporter.Report(ctx, "valhalla."+s.name, frac, line)
			lastReport = now
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		logger.Warn().Err(err).Str("step", s.name).Msg("valhalla stderr scanner")
	}
}

func lastPctOrZero(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
