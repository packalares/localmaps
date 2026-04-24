// update.go — KindRegionUpdate orchestrator. Delta vs install.go:
//
//   - The region row starts in state "ready" and transitions to
//     "updating" (not "downloading").
//   - We still stream into <data>/regions/<key>.new/ alongside the
//     existing <key>/ directory; the pipeline runs to completion and
//     swap.go promotes the new one, archiving the old in the process.
//   - On failure, install.markFailed flips state=failed. Per
//     docs/01-architecture.md the live <key>/ directory is preserved so
//     the region remains queryable during a broken update.
//
// The public entry point is UpdateHandler, mirroring the install
// package's NewHandler shape so registration in handlers_agent_e stays
// symmetric.

package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/jobs"
)

// NewUpdateHandler returns an Asynq HandlerFunc for KindRegionUpdate.
// Same Deps shape as NewHandler; the orchestration differs only in the
// "updating" transition + the fact that an existing <region>/ directory
// may already be live.
func NewUpdateHandler(d Deps, log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p jobs.RegionInstallPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("decode payload: %w", err)
		}
		if p.Region == "" {
			return errors.New("update: empty region")
		}
		l := log.With().Str("module", "region.update").
			Str("region", p.Region).Str("jobId", p.JobID).Logger()
		ctx = l.WithContext(ctx)

		if err := d.runUpdate(ctx, p, l); err != nil {
			_ = markFailed(ctx, d.DB, p.Region, p.JobID, err.Error())
			return err
		}
		return nil
	}
}

// runUpdate is the per-region update orchestration. Extracted from
// NewUpdateHandler for symmetry with Deps.run and direct-callable
// testing.
func (d Deps) runUpdate(ctx context.Context, p jobs.RegionInstallPayload, l zerolog.Logger) error {
	if err := markState(ctx, d.DB, p.Region, "updating",
		"fetching source pbf for update"); err != nil {
		return fmt.Errorf("mark updating: %w", err)
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
		Str("path", dest).Msg("pbf downloaded (update)")
	if err := persistSourceMeta(ctx, d.DB, p.Region, sum, size); err != nil {
		return fmt.Errorf("persist source meta: %w", err)
	}
	// Hand off to the normal pipeline chain; swap.go archives the
	// previous live directory on promotion.
	stagePayload := jobs.PipelineStagePayload{
		Region: p.Region, JobID: p.JobID, ParentJobID: p.JobID,
	}
	body, err := json.Marshal(stagePayload)
	if err != nil {
		return fmt.Errorf("marshal stage payload: %w", err)
	}
	if d.Queue == nil {
		l.Warn().Msg("update: Queue nil; pipeline chain will not advance")
		return nil
	}
	if _, err := d.Queue.EnqueueContext(ctx,
		asynq.NewTask(jobs.KindPipelineTiles, body)); err != nil {
		return fmt.Errorf("enqueue tiles: %w", err)
	}
	l.Info().Msg("update: enqueued pipeline:tiles; chain will swap on success")
	return nil
}
