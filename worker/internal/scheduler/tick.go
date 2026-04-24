// tick.go — helpers for Scheduler.Tick: due-time parsing, per-region
// bump of next_update_at, and KindRegionUpdate enqueue.

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/packalares/localmaps/internal/jobs"
)

// isDue reports whether nextUpdateAt (ISO-8601 or empty) is at or
// before now. Empty / unparseable -> due.
func isDue(nextUpdateAt string, now time.Time) bool {
	s := strings.TrimSpace(nextUpdateAt)
	if s == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, s); err != nil {
			return true
		}
	}
	return !t.After(now)
}

// bumpNextUpdate computes and persists the next firing time for the
// region. When schedule is invalid we surface the error so the caller
// can log; the UI validation gate catches most bad values on write.
func (s *Scheduler) bumpNextUpdate(ctx context.Context, region, schedule string, now time.Time) error {
	next, err := ComputeNextUpdate(now, schedule)
	if err != nil {
		return fmt.Errorf("compute next: %w", err)
	}
	if next.IsZero() {
		return nil
	}
	_, err = s.DB.ExecContext(ctx,
		`UPDATE regions SET next_update_at = ? WHERE name = ?`,
		next.Format(time.RFC3339Nano), region)
	return err
}

// enqueueUpdate inserts a jobs row + pushes the Asynq task. The job id
// is written back to regions.active_job_id so downstream handlers can
// link progress events back.
func (s *Scheduler) enqueueUpdate(ctx context.Context, region string) error {
	jobID := uuid.NewString()
	now := s.now().Format(time.RFC3339Nano)
	if _, err := s.DB.ExecContext(ctx, `
		INSERT INTO jobs(id, kind, region, state, created_by, started_at)
		VALUES (?,?,?,?,?,?)`,
		jobID, jobs.OpenAPIJobKindUpdateRegion, region, "queued", "scheduler", now,
	); err != nil {
		return fmt.Errorf("insert job row: %w", err)
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE regions SET active_job_id = ? WHERE name = ?`,
		jobID, region); err != nil {
		return fmt.Errorf("attach active_job_id: %w", err)
	}
	if s.Queue == nil {
		return nil
	}
	body, err := json.Marshal(jobs.RegionInstallPayload{
		Region: region, JobID: jobID, TriggeredBy: "scheduler",
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if _, err := s.Queue.EnqueueContext(ctx,
		asynq.NewTask(jobs.KindRegionUpdate, body)); err != nil {
		return fmt.Errorf("enqueue %s: %w", jobs.KindRegionUpdate, err)
	}
	return nil
}
