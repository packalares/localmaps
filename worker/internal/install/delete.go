// delete.go — KindRegionDelete orchestrator. Mirror of update.go shape;
// the work itself is purely teardown:
//
//  1. Wipe <DataDir>/regions/<region>/ (live dir).
//  2. Wipe <DataDir>/regions/<region>.new/ (any partial update left
//     behind by an interrupted KindRegionUpdate).
//  3. Issue `_delete_by_query` against pelias-es to drop every doc
//     tagged `addendum.osm.region = <region>` (gated on the
//     `regions.deletePurgesPelias` setting; default true).
//  4. Mark the job row succeeded and clear the region's active_job_id.
//     The region row itself stays in state=archived (soft-delete) per
//     docs/import-cleanup-plan.md so the admin UI can still surface a
//     tombstone.
//
// Failure semantics match update.go: any error flips the region to
// state=failed (preserving the archived row's last_error for the UI)
// and the Asynq retry policy applies.

package install

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/jobs"
	"github.com/packalares/localmaps/worker/internal/pipeline/peliasindex"
)

// PeliasURLReader is the narrow view of the worker's settings reader
// the delete handler needs. Kept tiny + interface-typed so tests can
// inject a fake without dragging in the worker's sqlx shim.
type PeliasURLReader interface {
	GetString(key string) (string, error)
	GetBool(key string) (bool, error)
}

// DeleteDeps extends Deps with the small surface unique to delete:
// a peliasindex purge needs the ES URL out of settings, plus a feature
// flag (`regions.deletePurgesPelias`) so operators can opt out.
type DeleteDeps struct {
	Deps
	Settings PeliasURLReader
}

// NewDeleteHandler returns an Asynq HandlerFunc for KindRegionDelete.
// Same shape as NewUpdateHandler / NewHandler; the orchestration is
// teardown-only (see file header).
func NewDeleteHandler(d DeleteDeps, log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p jobs.RegionDeletePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		if p.Region == "" {
			return errors.New("delete: empty region")
		}
		l := log.With().Str("module", "region.delete").
			Str("region", p.Region).Str("jobId", p.JobID).Logger()
		ctx = l.WithContext(ctx)

		if err := d.Delete(ctx, p, l); err != nil {
			_ = markFailed(ctx, d.DB, p.Region, p.JobID, err.Error())
			return err
		}
		return nil
	}
}

// Delete is the teardown orchestration. Extracted from NewDeleteHandler
// for symmetry with Deps.run / Deps.runUpdate and for direct-callable
// testing.
func (d DeleteDeps) Delete(ctx context.Context, p jobs.RegionDeletePayload, l zerolog.Logger) error {
	regionsDir := filepath.Join(d.DataDir, "regions")
	liveDir := filepath.Join(regionsDir, p.Region)
	newDir := filepath.Join(regionsDir, p.Region+".new")

	// (1) + (2) wipe on-disk artifacts. RemoveAll already swallows
	// "not exist", so the same code handles "no live dir yet" + "no
	// .new left behind" cleanly.
	if err := os.RemoveAll(liveDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove live dir %s: %w", liveDir, err)
	}
	if err := os.RemoveAll(newDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove new dir %s: %w", newDir, err)
	}
	l.Info().Str("liveDir", liveDir).Str("newDir", newDir).
		Msg("delete: filesystem artifacts removed")

	// (3) purge pelias docs tagged with this region. Gated on the
	// `regions.deletePurgesPelias` setting so operators can disable it
	// (e.g. while migrating ES infrastructure).
	if shouldPurgePelias(d.Settings) {
		esURL := peliasESURL(d.Settings)
		deleted, err := peliasindex.PurgeRegion(ctx, esURL, p.Region, l)
		if err != nil {
			return fmt.Errorf("purge pelias: %w", err)
		}
		l.Info().Int64("deleted", deleted).Str("esURL", esURL).
			Msg("delete: pelias docs purged")
	} else {
		l.Info().Msg("delete: pelias purge skipped (regions.deletePurgesPelias=false)")
	}

	// (4) finalise: mark the region row + job row.
	if err := finishDelete(ctx, d.DB, p.Region, p.JobID); err != nil {
		return fmt.Errorf("finalise delete: %w", err)
	}
	l.Info().Msg("delete complete")
	return nil
}

// shouldPurgePelias reads the `regions.deletePurgesPelias` flag,
// defaulting to true on missing rows / nil settings (matches the
// seeded default in server/internal/config/defaults.go).
func shouldPurgePelias(s PeliasURLReader) bool {
	if s == nil {
		return true
	}
	v, err := s.GetBool("regions.deletePurgesPelias")
	if err != nil {
		return true
	}
	return v
}

// peliasESURL mirrors the worker's stage-side helper (handlers_agent_h.go);
// duplicated here because handlers_agent_h is in package main and we
// cannot reach across the package boundary. Resolution order:
// settings.search.peliasElasticUrl → LOCALMAPS_PELIAS_ES_URL env →
// http://pelias-es:9200.
func peliasESURL(s PeliasURLReader) string {
	if s != nil {
		if raw, err := s.GetString("search.peliasElasticUrl"); err == nil && raw != "" {
			if normalised, ok := normalisePeliasURL(raw); ok {
				return normalised
			}
		}
	}
	if raw := os.Getenv("LOCALMAPS_PELIAS_ES_URL"); raw != "" {
		if normalised, ok := normalisePeliasURL(raw); ok {
			return normalised
		}
	}
	return "http://pelias-es:9200"
}

func normalisePeliasURL(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", false
	}
	host := u.Hostname()
	port := 9200
	if p := u.Port(); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			port = v
		}
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port), true
}

// finishDelete writes the terminal state: jobs.state=succeeded,
// regions.active_job_id cleared. The region row itself stays in
// state=archived (soft delete) so the admin UI can still surface a
// tombstone — matches the recommendation in
// docs/import-cleanup-plan.md.
func finishDelete(ctx context.Context, db *sql.DB, region, jobID string) error {
	if db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE regions SET active_job_id = NULL,
		 state_detail = 'archived',
		 last_error = NULL
		 WHERE name = ?`, region); err != nil {
		_ = tx.Rollback()
		return err
	}
	if jobID != "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE jobs SET state = 'succeeded', finished_at = ?,
			 error = NULL WHERE id = ?`, now, jobID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// peliasSettingsAdapter wraps any *sqlx.DB-backed settings reader the
// caller already has into the narrow PeliasURLReader the handler
// expects. Kept here so worker/cmd/worker can adapt its sqlxSettings
// without exposing it as part of the public Deps shape.
type peliasSettingsAdapter struct {
	GetStringFn func(key string) (string, error)
	GetBoolFn   func(key string) (bool, error)
}

// GetString implements PeliasURLReader.
func (a peliasSettingsAdapter) GetString(key string) (string, error) {
	if a.GetStringFn == nil {
		return "", sql.ErrNoRows
	}
	return a.GetStringFn(key)
}

// GetBool implements PeliasURLReader.
func (a peliasSettingsAdapter) GetBool(key string) (bool, error) {
	if a.GetBoolFn == nil {
		return false, sql.ErrNoRows
	}
	return a.GetBoolFn(key)
}

// AdaptSettings is a small constructor that lets the worker's main
// pass its own sqlx-backed settings into the delete handler without
// exporting that type. Intended call site: handlers_agent_e.go.
func AdaptSettings(getString func(key string) (string, error),
	getBool func(key string) (bool, error)) PeliasURLReader {
	return peliasSettingsAdapter{GetStringFn: getString, GetBoolFn: getBool}
}

// Compile-time check that we satisfy the interface.
var _ PeliasURLReader = peliasSettingsAdapter{}
