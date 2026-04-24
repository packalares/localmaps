package regions

// mutations.go holds Update / Delete / SetSchedule. The transactional
// pattern matches Install in service.go.

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/packalares/localmaps/internal/jobs"
)

// Update enqueues an update for a region whose state is `ready` or
// `failed` (per docs/04-data-model.md).
func (s *Service) Update(ctx context.Context, input, triggeredBy string) (Job, error) {
	canonical, err := ensureValidRegionName(input)
	if err != nil {
		return Job{}, fmt.Errorf("validate name: %w", err)
	}
	existing, err := s.Get(ctx, canonical)
	if err != nil {
		return Job{}, err
	}
	if existing.State != StateReady && existing.State != StateFailed {
		return Job{}, fmt.Errorf("%w: state=%s", ErrConflict, existing.State)
	}
	job := Job{
		ID:        uuid.NewString(),
		Kind:      jobs.OpenAPIJobKindUpdateRegion,
		Region:    &canonical,
		State:     "queued",
		CreatedBy: strPtr(triggeredBy),
	}
	err = s.runTx(func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE regions SET state = ?, active_job_id = ?,
			 state_detail = ? WHERE name = ?`,
			StateUpdating, job.ID, "queued for update", canonical); err != nil {
			return err
		}
		return txInsertJob(ctx, tx, job)
	})
	if err != nil {
		return Job{}, err
	}
	payload := jobs.RegionInstallPayload{
		Region: canonical, JobID: job.ID, TriggeredBy: triggeredBy,
	}
	if err := s.enqueue(ctx, jobs.KindRegionUpdate, payload); err != nil {
		return Job{}, fmt.Errorf("enqueue update: %w", err)
	}
	return job, nil
}

// Delete enqueues a delete (archive) for a region.
func (s *Service) Delete(ctx context.Context, input, triggeredBy string) (Region, Job, error) {
	canonical, err := ensureValidRegionName(input)
	if err != nil {
		return Region{}, Job{}, fmt.Errorf("validate name: %w", err)
	}
	existing, err := s.Get(ctx, canonical)
	if err != nil {
		return Region{}, Job{}, err
	}
	job := Job{
		ID:        uuid.NewString(),
		Kind:      jobs.OpenAPIJobKindArchiveRegion,
		Region:    &canonical,
		State:     "queued",
		CreatedBy: strPtr(triggeredBy),
	}
	err = s.runTx(func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE regions SET state = ?, active_job_id = ?
			 WHERE name = ?`,
			StateArchived, job.ID, canonical); err != nil {
			return err
		}
		return txInsertJob(ctx, tx, job)
	})
	if err != nil {
		return Region{}, Job{}, err
	}
	payload := jobs.RegionDeletePayload{
		Region: canonical, JobID: job.ID, TriggeredBy: triggeredBy,
	}
	if err := s.enqueue(ctx, jobs.KindRegionDelete, payload); err != nil {
		return Region{}, Job{}, fmt.Errorf("enqueue delete: %w", err)
	}
	existing.State = StateArchived
	existing.ActiveJobID = &job.ID
	return existing, job, nil
}

// SetSchedule validates + persists the update schedule.
func (s *Service) SetSchedule(ctx context.Context, input, schedule string) (Region, error) {
	canonical, err := ensureValidRegionName(input)
	if err != nil {
		return Region{}, fmt.Errorf("validate name: %w", err)
	}
	if !validSchedule(schedule) {
		return Region{}, ErrInvalidSchedule
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE regions SET schedule = ? WHERE name = ?`,
		schedule, canonical)
	if err != nil {
		return Region{}, fmt.Errorf("set schedule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Region{}, ErrNotFound
	}
	return s.Get(ctx, canonical)
}

// cronLikePattern accepts the 5-field cron spec ("m h dom mon dow").
// Each field allows digits, `*`, `/`, `,`, `-`. This is a conservative
// gate — the scheduler (Agent N) will use robfig/cron for full parsing.
var cronLikePattern = regexp.MustCompile(`^[\d\*/,\-]+( [\d\*/,\-]+){4}$`)

func validSchedule(s string) bool {
	switch s {
	case ScheduleNever, ScheduleDaily, ScheduleWeekly, ScheduleMonthly:
		return true
	}
	if strings.Count(s, " ") == 4 && cronLikePattern.MatchString(s) {
		return true
	}
	return false
}
