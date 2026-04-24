// Package regions owns the region state machine (see
// docs/04-data-model.md), the HTTP handlers for the `regions` tag in
// contracts/openapi.yaml, and the orchestration that enqueues Asynq
// jobs into the worker.
//
// Rule of the road: validate input through the shared normaliser in
// internal/regions before any DB or filesystem touch. Every fallible
// call writes a zerolog line with traceId.
package regions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"

	"github.com/packalares/localmaps/internal/catalog"
	"github.com/packalares/localmaps/internal/jobs"
)

// ErrNotFound is returned when the region row does not exist.
var ErrNotFound = errors.New("regions: not found")

// ErrConflict is returned when a state transition is disallowed
// (e.g. installing a region that's already downloading).
var ErrConflict = errors.New("regions: state transition not allowed")

// ErrInvalidSchedule is returned when SetSchedule receives a value
// that's neither a preset enum nor a valid cron string.
var ErrInvalidSchedule = errors.New("regions: invalid schedule")

// Enqueuer is the small subset of *asynq.Client we depend on. The
// indirection keeps tests trivial and isolates us from the asynq
// package API churn.
type Enqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Catalog is the narrow subset of the shared catalog.Reader we depend on.
type Catalog interface {
	ListRegions(ctx context.Context) ([]catalog.Entry, error)
	Resolve(ctx context.Context, canonicalKey string) (catalog.Entry, error)
}

// Service bundles the DB + catalog + queue client. Construct one per
// process via NewService; share via the router Deps.
type Service struct {
	db      *sqlx.DB
	catalog Catalog
	queue   Enqueuer
}

// NewService wires a Service from its collaborators. All three are
// required; nil will panic at first use.
func NewService(db *sqlx.DB, catalog Catalog, queue Enqueuer) *Service {
	return &Service{db: db, catalog: catalog, queue: queue}
}

// ListCatalog proxies the Geofabrik catalog. It returns a slice of
// CatalogEntry whose JSON representation matches
// components/schemas/RegionCatalogEntry in openapi.yaml.
func (s *Service) ListCatalog(ctx context.Context) ([]catalog.Entry, error) {
	return s.catalog.ListRegions(ctx)
}

// ListInstalled returns every row in the regions table.
func (s *Service) ListInstalled(ctx context.Context) ([]Region, error) {
	const q = `SELECT name, display_name, parent, source_url,
		source_pbf_sha256, source_pbf_bytes, bbox, state, state_detail,
		last_error, installed_at, last_updated_at, next_update_at,
		schedule, disk_bytes, active_job_id
		FROM regions`
	var rows []Region
	if err := s.db.SelectContext(ctx, &rows, q); err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	for i := range rows {
		_ = rows[i].hydrateBBox()
	}
	return rows, nil
}

// Get returns a single region by canonical key. Returns ErrNotFound
// when the row is absent.
func (s *Service) Get(ctx context.Context, canonical string) (Region, error) {
	const q = `SELECT name, display_name, parent, source_url,
		source_pbf_sha256, source_pbf_bytes, bbox, state, state_detail,
		last_error, installed_at, last_updated_at, next_update_at,
		schedule, disk_bytes, active_job_id
		FROM regions WHERE name = ?`
	var r Region
	err := s.db.GetContext(ctx, &r, q, canonical)
	if errors.Is(err, sql.ErrNoRows) {
		return Region{}, ErrNotFound
	}
	if err != nil {
		return Region{}, fmt.Errorf("get region: %w", err)
	}
	_ = r.hydrateBBox()
	return r, nil
}

// Install creates (or restarts) the install pipeline for a canonical
// region. Returns the Job row that will carry progress.
func (s *Service) Install(ctx context.Context, input, triggeredBy string) (Region, Job, error) {
	canonical, err := ensureValidRegionName(input)
	if err != nil {
		return Region{}, Job{}, fmt.Errorf("validate name: %w", err)
	}
	entry, err := s.catalog.Resolve(ctx, canonical)
	if err != nil {
		return Region{}, Job{}, fmt.Errorf("resolve catalog: %w", err)
	}

	job := Job{
		ID:        uuid.NewString(),
		Kind:      jobs.OpenAPIJobKindDownloadPBF,
		Region:    &canonical,
		State:     "queued",
		CreatedBy: strPtr(triggeredBy),
	}
	region := newRegionFromCatalog(entry)
	region.State = StateDownloading
	region.ActiveJobID = &job.ID
	region.StateDetail = strPtr("queued for download")

	err = s.runTx(func(tx *sqlx.Tx) error {
		// Lock + check existing row.
		existing, eerr := txGetRegion(ctx, tx, canonical)
		if eerr != nil && !errors.Is(eerr, ErrNotFound) {
			return eerr
		}
		if eerr == nil {
			switch existing.State {
			case StateNotInstalled, StateArchived, StateFailed:
				// allowed — proceed to re-install
			default:
				return fmt.Errorf("%w: state=%s", ErrConflict, existing.State)
			}
			if err := txUpdateRegionToDownloading(ctx, tx, canonical, job.ID, entry); err != nil {
				return err
			}
		} else {
			if err := txInsertRegion(ctx, tx, region); err != nil {
				return err
			}
		}
		if err := txInsertJob(ctx, tx, job); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Region{}, Job{}, err
	}

	payload := jobs.RegionInstallPayload{
		Region:      canonical,
		JobID:       job.ID,
		TriggeredBy: triggeredBy,
	}
	if err := s.enqueue(ctx, jobs.KindRegionInstall, payload); err != nil {
		return Region{}, Job{}, fmt.Errorf("enqueue install: %w", err)
	}
	// Re-read so we return the fresh row.
	saved, err := s.Get(ctx, canonical)
	if err != nil {
		return Region{}, Job{}, err
	}
	return saved, job, nil
}

// enqueue JSON-encodes the payload and submits the Asynq task.
func (s *Service) enqueue(ctx context.Context, kind string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", kind, err)
	}
	_, err = s.queue.EnqueueContext(ctx, asynq.NewTask(kind, body))
	return err
}

// runTx is a tiny wrapper around sqlx.DB.Beginx/Commit.
func (s *Service) runTx(fn func(tx *sqlx.Tx) error) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Low-level tx helpers live in tx.go; mutations in mutations.go.
