package regions

// tx.go houses the lowest-level DB helpers used inside Service
// transactions. Keeping them in a sibling file keeps service.go under
// the 250-line cap per docs/06-agent-rules.md.

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/packalares/localmaps/internal/catalog"
)

func txGetRegion(ctx context.Context, tx *sqlx.Tx, name string) (Region, error) {
	const q = `SELECT name, display_name, parent, source_url,
		source_pbf_sha256, source_pbf_bytes, bbox, state, state_detail,
		last_error, installed_at, last_updated_at, next_update_at,
		schedule, disk_bytes, active_job_id
		FROM regions WHERE name = ?`
	var r Region
	err := tx.GetContext(ctx, &r, q, name)
	if errors.Is(err, sql.ErrNoRows) {
		return Region{}, ErrNotFound
	}
	return r, err
}

func txInsertRegion(ctx context.Context, tx *sqlx.Tx, r Region) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO regions (name, display_name, parent, source_url,
		 state, state_detail, active_job_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.DisplayName, r.Parent, r.SourceURL,
		r.State, r.StateDetail, r.ActiveJobID)
	return err
}

func txUpdateRegionToDownloading(ctx context.Context, tx *sqlx.Tx, canonical, jobID string, e catalog.Entry) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE regions SET state = ?, state_detail = ?, source_url = ?,
		 display_name = ?, active_job_id = ?, last_error = NULL
		 WHERE name = ?`,
		StateDownloading, "queued for download", e.SourceURL,
		e.DisplayName, jobID, canonical)
	return err
}

func txInsertJob(ctx context.Context, tx *sqlx.Tx, j Job) error {
	started := nowRFC3339()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO jobs (id, kind, region, state, message, started_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Kind, j.Region, j.State, j.Message, started, j.CreatedBy)
	return err
}
