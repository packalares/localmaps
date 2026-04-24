// scheduler.go — the periodic loop. A Scheduler runs one Tick every
// tick-interval (derived from settings key regions.updateCheckCron); each
// Tick walks regions whose next_update_at has elapsed and either bumps
// that column (up-to-date) or enqueues a KindRegionUpdate job.

package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/catalog"
)

// Enqueuer is the narrow view of *asynq.Client used by the scheduler to
// hand off KindRegionUpdate tasks. Defined here so tests can swap in a
// recorder.
type Enqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Scheduler wires the data-layer, catalog, queue, and clock into a
// long-running tick loop. Construct one in the worker main via
// registerAgentEHandlers and call Start in a goroutine.
type Scheduler struct {
	// DB is the SQLite /data/config.db handle. Required.
	DB *sql.DB
	// Queue enqueues the KindRegionUpdate Asynq task. May be nil in
	// tests (Tick logs that it would have enqueued).
	Queue Enqueuer
	// Catalog resolves canonical region keys to their catalog entry.
	Catalog catalog.Reader
	// Fetcher provides the .md5 sidecar. Usually *geofabrik.Client.
	Fetcher ChecksumFetcher
	// TickCron is the 5-field cron spec that governs how often Tick
	// runs. Defaults to "0 3 * * *" (docs/07-config-schema.md).
	TickCron string
	// Now is the clock; nil means time.Now. Injected so Tick is
	// deterministic in tests.
	Now func() time.Time
	// Logger is the zerolog logger. Required for structured logs.
	Logger zerolog.Logger
}

// defaultTickCron is the fallback when settings are missing or invalid.
// Matches regions.updateCheckCron default.
const defaultTickCron = "0 3 * * *"

// now is a small helper to centralise the clock.
func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// TickSummary is the return value of Tick, exposed so callers (tests,
// metrics collectors) can assert on it.
type TickSummary struct {
	// Checked is the number of rows matched by the schedule filter.
	Checked int
	// Due is the number of rows where next_update_at <= now (or null).
	Due int
	// Enqueued is the number of KindRegionUpdate tasks enqueued.
	Enqueued int
	// FirstError is the first non-nil error encountered (nil on clean
	// ticks). Per-row failures do not abort the loop.
	FirstError error
}

// Tick performs one pass over due regions. Safe to call directly from
// tests; Start invokes it on the tick schedule.
//
// The loop never aborts on a per-region failure — a single bad row
// must not stall the rest. The first error is exposed via
// TickSummary.FirstError for the caller to surface via metrics or logs.
func (s *Scheduler) Tick(ctx context.Context) (TickSummary, error) {
	sum := TickSummary{}
	if s.DB == nil {
		return sum, fmt.Errorf("scheduler: DB is nil")
	}
	now := s.now()
	due_, err := s.selectDueRegions(ctx)
	if err != nil {
		return sum, err
	}
	checker := UpdateCheck{DB: s.DB, Catalog: s.Catalog, Fetcher: s.Fetcher}
	for _, d := range due_ {
		sum.Checked++
		if !isDue(d.nextAt, now) {
			continue
		}
		sum.Due++
		s.processOne(ctx, checker, d, now, &sum)
	}
	s.Logger.Info().
		Int("checked", sum.Checked).
		Int("due", sum.Due).
		Int("enqueued", sum.Enqueued).
		Msg("scheduler: tick done")
	return sum, nil
}

// dueRow is a tiny value type carried between selectDueRegions and
// processOne. Unexported.
type dueRow struct{ name, schedule, nextAt string }

// selectDueRegions returns every ready region with a non-never schedule.
func (s *Scheduler) selectDueRegions(ctx context.Context) ([]dueRow, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT name, COALESCE(schedule, ''), COALESCE(next_update_at, '')
		FROM regions
		WHERE state = 'ready'
		  AND schedule IS NOT NULL
		  AND schedule != ''
		  AND schedule != 'never'
	`)
	if err != nil {
		return nil, fmt.Errorf("query regions: %w", err)
	}
	defer rows.Close()
	var out []dueRow
	for rows.Next() {
		var d dueRow
		if err := rows.Scan(&d.name, &d.schedule, &d.nextAt); err != nil {
			return nil, fmt.Errorf("scan region row: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate region rows: %w", err)
	}
	return out, nil
}

// processOne runs the update-check + enqueue + bump for a single due
// region. Per-row errors are logged + recorded into sum but never
// aborted — see the docstring on Tick for the rationale.
func (s *Scheduler) processOne(ctx context.Context, checker UpdateCheck, d dueRow, now time.Time, sum *TickSummary) {
	shouldUpdate, reason, cerr := checker.ShouldUpdate(ctx, d.name)
	if cerr != nil {
		s.Logger.Warn().Str("region", d.name).
			Str("reason", string(reason)).Err(cerr).
			Msg("scheduler: update check failed; advancing next_update_at and retrying later")
		_ = s.bumpNextUpdate(ctx, d.name, d.schedule, now)
		if sum.FirstError == nil {
			sum.FirstError = cerr
		}
		return
	}
	if shouldUpdate {
		if err := s.enqueueUpdate(ctx, d.name); err != nil {
			s.Logger.Error().Str("region", d.name).Err(err).
				Msg("scheduler: enqueue update failed")
			if sum.FirstError == nil {
				sum.FirstError = err
			}
			return
		}
		sum.Enqueued++
		s.Logger.Info().Str("region", d.name).
			Str("reason", string(reason)).Msg("scheduler: enqueued region update")
	} else {
		s.Logger.Debug().Str("region", d.name).
			Str("reason", string(reason)).Msg("scheduler: region up to date")
	}
	if err := s.bumpNextUpdate(ctx, d.name, d.schedule, now); err != nil {
		s.Logger.Warn().Str("region", d.name).Err(err).
			Msg("scheduler: bump next_update_at failed")
	}
}

// Start runs Tick on a cron schedule until ctx is cancelled. TickCron
// is parsed once; if parsing fails we fall back to defaultTickCron and
// log a warning — the loop keeps running so a bad setting never
// silently disables updates.
func (s *Scheduler) Start(ctx context.Context) {
	spec := strings.TrimSpace(s.TickCron)
	if spec == "" {
		spec = defaultTickCron
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(spec)
	if err != nil {
		s.Logger.Warn().Str("spec", spec).Err(err).
			Msgf("scheduler: invalid TickCron; falling back to %q", defaultTickCron)
		sched, _ = parser.Parse(defaultTickCron)
	}
	s.Logger.Info().Str("cron", spec).Msg("scheduler: starting tick loop")
	for {
		next := sched.Next(s.now())
		wait := time.Until(next)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			s.Logger.Info().Msg("scheduler: context cancelled; stopping")
			return
		case <-timer.C:
			if _, err := s.Tick(ctx); err != nil {
				s.Logger.Error().Err(err).Msg("scheduler: tick error")
			}
		}
	}
}
