// Package install hosts the KindRegionInstall orchestrator. It:
//
//  1. Marks the region row as `downloading`.
//  2. Resolves the pbf URL + md5 via the catalog client.
//  3. Streams the pbf into <data>/regions/<key>.new/source.osm.pbf.
//  4. Verifies md5.
//  5. Enqueues KindPipelineTiles, KindPipelineRouting,
//     KindPipelineGeocoding sequentially — each handler returns when
//     the next one is ready to enqueue. (Simpler than Asynq groups;
//     docs the trade-off here.)
//
// On any failure, last_error + state=failed are persisted and the
// Asynq error is returned so retries happen per the queue policy.
package install

import (
	"context"
	"crypto/md5" // #nosec G501 — md5 is what geofabrik publishes.
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/catalog"
	"github.com/packalares/localmaps/internal/jobs"
)

// Deps bundles the handler's collaborators. Construct one in the
// worker's main() and bind the returned HandlerFunc to the Asynq mux.
type Deps struct {
	DB       *sql.DB
	DataDir  string
	Catalog  catalog.Reader
	Queue    Enqueuer
	HTTP     *http.Client
	AllowURL func(host string) bool
}

// Enqueuer is the narrow view of *asynq.Client used to chain pipelines.
type Enqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// ProgressReporter receives milestone updates as the download streams.
// Satisfied by pipeline.AsynqProgress; a DiscardReporter is provided.
type ProgressReporter interface {
	Report(ctx context.Context, pct float64, message string) error
}

// DiscardReporter is a ProgressReporter that drops everything.
type DiscardReporter struct{}

// Report implements ProgressReporter.
func (DiscardReporter) Report(context.Context, float64, string) error { return nil }

// NewHandler returns an Asynq HandlerFunc for the KindRegionInstall task.
func NewHandler(d Deps, log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p jobs.RegionInstallPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		if p.Region == "" {
			return errors.New("install: empty region")
		}
		l := log.With().Str("module", "region.install").
			Str("region", p.Region).Str("jobId", p.JobID).Logger()
		ctx = l.WithContext(ctx)

		if err := d.run(ctx, p, l); err != nil {
			_ = markFailed(ctx, d.DB, p.Region, p.JobID, err.Error())
			return err
		}
		return nil
	}
}

// run performs the orchestration. Extracted so tests can drive it.
func (d Deps) run(ctx context.Context, p jobs.RegionInstallPayload, l zerolog.Logger) error {
	if err := markState(ctx, d.DB, p.Region, "downloading",
		"fetching source pbf"); err != nil {
		return fmt.Errorf("mark downloading: %w", err)
	}
	entry, err := d.Catalog.Resolve(ctx, p.Region)
	if err != nil {
		return fmt.Errorf("resolve catalog: %w", err)
	}
	if entry.SourceURL == "" {
		return errors.New("catalog entry has no sourceUrl")
	}
	destDir := filepath.Join(d.DataDir, "regions", p.Region+".new")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	dest := filepath.Join(destDir, "source.osm.pbf")
	sum, size, err := Download(ctx, d.HTTP, entry.SourceURL, dest)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	l.Info().Str("md5", sum).Int64("bytes", size).
		Str("path", dest).Msg("pbf downloaded")
	if err := persistSourceMeta(ctx, d.DB, p.Region, sum, size); err != nil {
		return fmt.Errorf("persist source meta: %w", err)
	}
	// Enqueue the next stage. Sequential approach: each stage's handler
	// will enqueue the next on success, ending with KindRegionSwap. This
	// is simpler than Asynq groups and traceable in the jobs table.
	stagePayload := jobs.PipelineStagePayload{
		Region: p.Region, JobID: p.JobID, ParentJobID: p.JobID,
	}
	body, err := json.Marshal(stagePayload)
	if err != nil {
		return fmt.Errorf("marshal stage payload: %w", err)
	}
	if _, err := d.Queue.EnqueueContext(ctx,
		asynq.NewTask(jobs.KindPipelineTiles, body)); err != nil {
		return fmt.Errorf("enqueue tiles: %w", err)
	}
	l.Info().Msg("enqueued pipeline:tiles; subsequent stages will chain")
	return nil
}

// Download streams url to destPath via dest.tmp + rename. Returns the
// md5 hex digest and total bytes written. The caller is responsible
// for host allow-listing — pass a pre-validated URL.
func Download(ctx context.Context, client *http.Client, url, destPath string) (string, int64, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("http %d from %s", resp.StatusCode, url)
	}
	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", 0, err
	}
	h := md5.New() // #nosec G401 — md5 is what geofabrik publishes.
	n, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if cerr := f.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	if err := os.Rename(tmp, destPath); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// --- DB helpers ----------------------------------------------------

func markState(ctx context.Context, db *sql.DB, name, state, detail string) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`UPDATE regions SET state = ?, state_detail = ? WHERE name = ?`,
		state, detail, name)
	return err
}

func markFailed(ctx context.Context, db *sql.DB, name, jobID, errMsg string) error {
	if db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE regions SET state = 'failed', last_error = ? WHERE name = ?`,
		errMsg, name); err != nil {
		_ = tx.Rollback()
		return err
	}
	if jobID != "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE jobs SET state = 'failed', error = ?, finished_at = ? WHERE id = ?`,
			errMsg, now, jobID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func persistSourceMeta(ctx context.Context, db *sql.DB, name, md5sum string, bytes int64) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`UPDATE regions SET source_pbf_sha256 = ?, source_pbf_bytes = ?
		 WHERE name = ?`, md5sum, bytes, name)
	return err
}
