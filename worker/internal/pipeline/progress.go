// Package pipeline contains the per-region build stages described in
// docs/01-architecture.md. This file defines the ProgressReporter
// interface used by every stage (planetiler, valhalla, pelias,
// overture) and a throttled Asynq-backed implementation.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

// ProgressReporter is the cross-stage contract: stage is a short
// identifier (e.g. "planetiler.pass1.nodes"); fraction is monotonic
// 0.0..1.0 overall completion; message is the human string shown in
// the admin UI. Implementations MUST be safe for concurrent Report
// calls and MUST tolerate best-effort delivery (drops are acceptable).
type ProgressReporter interface {
	Report(ctx context.Context, stage string, fraction float64, message string) error
}

// WsPublisher is the minimal surface the reporter needs from the
// gateway WS hub. The worker injects a thin adapter so this package
// never imports server/internal/ws directly (keeps the worker
// self-contained — see docs/01-architecture.md module boundaries).
type WsPublisher interface {
	Publish(eventType, channel string, data any)
}

// DiscardProgress satisfies ProgressReporter but drops every event.
// Used in tests and when the caller does not care about progress.
type DiscardProgress struct{}

// Report implements ProgressReporter by doing nothing.
func (DiscardProgress) Report(_ context.Context, _ string, _ float64, _ string) error {
	return nil
}

// minReportIntervalDefault is how often AsynqProgress will actually
// emit an update when Report is called in a tight loop. Planetiler and
// valhalla both chatter many times per second; throttling here keeps
// Redis + WS fan-out manageable.
const minReportIntervalDefault = 500 * time.Millisecond

// AsynqProgress writes progress updates to three sinks:
//  1. the Asynq task's ResultWriter (so `asynq dash` shows live %),
//  2. the jobs SQLite row (progress + message columns),
//  3. the WS hub (as a job.progress event; payload matches
//     contracts/openapi.yaml components/schemas/Job).
//
// Emissions are throttled to at most one per MinInterval. The terminal
// fractions 0.0 (first call) and 1.0 are always flushed regardless of
// the throttle so the UI sees the final state promptly.
type AsynqProgress struct {
	JobID       string
	TaskWriter  *asynq.ResultWriter // may be nil in tests
	DB          *sqlx.DB            // may be nil in tests
	WS          WsPublisher         // may be nil in tests
	Logger      zerolog.Logger
	MinInterval time.Duration
	Now         func() time.Time

	mu       sync.Mutex
	lastAt   time.Time
	lastFrac float64
}

// NewAsynqProgress wires an AsynqProgress for the given job id. All
// optional sinks may be nil; missing ones are simply skipped at emit
// time. Callers that want zero throttling should pass MinInterval=1
// (not 0; zero uses the default).
func NewAsynqProgress(jobID string, w *asynq.ResultWriter, db *sqlx.DB, ws WsPublisher, log zerolog.Logger) *AsynqProgress {
	return &AsynqProgress{
		JobID:       jobID,
		TaskWriter:  w,
		DB:          db,
		WS:          ws,
		Logger:      log,
		MinInterval: minReportIntervalDefault,
		Now:         time.Now,
	}
}

// progressPayload mirrors contracts/openapi.yaml Job (subset). Only the
// fields the progress stream mutates are set; consumers merge into
// whatever they already have.
type progressPayload struct {
	ID       string  `json:"id"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
	Stage    string  `json:"stage,omitempty"`
}

// Report pushes an update to all configured sinks, subject to the
// MinInterval throttle. Fractions are clamped to [0,1] and the
// monotonic invariant is enforced: if the caller reports a smaller
// fraction than the previous one the call is dropped (and a warn log
// emitted) — this keeps the UI from jittering backwards.
func (p *AsynqProgress) Report(ctx context.Context, stage string, fraction float64, message string) error {
	if p == nil {
		return errors.New("progress: nil reporter")
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	now := p.Now()

	p.mu.Lock()
	// Monotonic: never step backwards.
	if fraction < p.lastFrac-1e-9 {
		p.Logger.Warn().
			Float64("prev", p.lastFrac).
			Float64("new", fraction).
			Str("stage", stage).
			Msg("progress: non-monotonic report dropped")
		p.mu.Unlock()
		return nil
	}
	interval := p.MinInterval
	if interval <= 0 {
		interval = minReportIntervalDefault
	}
	// Always flush first (lastAt zero), terminal (1.0), and non-throttled
	// intervals. Intermediate rapid updates are coalesced.
	forceFlush := p.lastAt.IsZero() || fraction >= 1.0-1e-9
	if !forceFlush && now.Sub(p.lastAt) < interval {
		p.lastFrac = fraction // remember latest, will flush on next tick
		p.mu.Unlock()
		return nil
	}
	p.lastAt = now
	p.lastFrac = fraction
	p.mu.Unlock()

	payload := progressPayload{
		ID:       p.JobID,
		State:    "running",
		Progress: fraction,
		Message:  message,
		Stage:    stage,
	}
	if fraction >= 1.0-1e-9 {
		payload.State = "succeeded"
	}

	// Sink 1: Asynq ResultWriter (JSON line per update).
	if p.TaskWriter != nil {
		if b, err := json.Marshal(payload); err == nil {
			if _, werr := p.TaskWriter.Write(b); werr != nil {
				p.Logger.Warn().Err(werr).Msg("progress: asynq writer")
			}
		}
	}
	// Sink 2: jobs row (progress + message). Per docs/04-data-model.md
	// the jobs table has no state_detail column; we write to `message`.
	if p.DB != nil && p.JobID != "" {
		if _, err := p.DB.ExecContext(ctx,
			`UPDATE jobs SET progress = ?, message = ? WHERE id = ?`,
			fraction, message, p.JobID); err != nil {
			p.Logger.Warn().Err(err).Str("jobId", p.JobID).Msg("progress: db update")
		}
	}
	// Sink 3: WS hub.
	if p.WS != nil && p.JobID != "" {
		p.WS.Publish("job.progress", fmt.Sprintf("job.%s", p.JobID), payload)
	}
	return nil
}

// --- planetiler-specific helpers (parser + tail buffer) ---------------
// Kept in progress.go as shared plumbing rather than a new file (spec
// constrains agent F to the file list in the preamble). Private names
// prevent collisions with valhalla/pelias helpers until Primary
// reconciles at the Phase 2 gate.

type stageWeight struct {
	key    string
	weight float64
}

// planetilerStages weight: pass1 20 %, pass2_nodes 10 % (reserved),
// pass2 30 %, sort 15 %, mbtiles 25 %.
var planetilerStages = []stageWeight{
	{"osm_pass1", 0.20}, {"osm_pass2_nodes", 0.10},
	{"osm_pass2", 0.30}, {"sort", 0.15}, {"mbtiles", 0.25},
}

var progressPattern = regexp.MustCompile(`\[([a-z0-9_]+)\][^0-9%]*(\d{1,3})%`)

type progressParser struct{ cumul map[string]float64 }

func newProgressParser() *progressParser {
	return &progressParser{cumul: map[string]float64{}}
}

func (p *progressParser) Parse(line string) (string, float64, bool) {
	m := progressPattern.FindStringSubmatch(line)
	if m == nil {
		return "", 0, false
	}
	stage, pct := m[1], 0
	for _, d := range m[2] {
		pct = pct*10 + int(d-'0')
	}
	if pct > 100 {
		pct = 100
	}
	stageFrac, overall := float64(pct)/100.0, 0.0
	for _, sw := range planetilerStages {
		if sw.key == stage {
			overall += sw.weight * stageFrac
			break
		}
		overall += sw.weight * p.cumul[sw.key]
	}
	if prev := p.cumul[stage]; stageFrac > prev {
		p.cumul[stage] = stageFrac
	}
	if overall > 1 {
		overall = 1
	}
	return "planetiler." + stage, overall, true
}

