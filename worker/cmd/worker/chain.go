// chain.go — per-region state advancement + next-stage enqueue helpers
// shared by Agent F/G/H handlers and the swap handler.
//
// The chain is strictly sequential: install → tiles → routing →
// geocoding → swap. Each handler advances regions.state on entry and
// enqueues the next kind on success. Failure drops the region into
// state=failed with last_error; Asynq retries the same stage up to
// the retry policy.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/jobs"
	"github.com/packalares/localmaps/worker/internal/pipeline"
)

// ChainDeps is the shared bundle of collaborators that every pipeline
// stage handler needs to advance state + chain to the next kind.
//
// Settings + WsPub are optional — nil means "no settings available"
// (tests) or "no websocket hub" (worker running without a live gateway
// peer). StageWork implementations MUST tolerate both being nil.
//
// StageSettings lives in stage_progress.go alongside the other progress
// plumbing.
type ChainDeps struct {
	DB       *sql.DB
	Queue    ChainEnqueuer
	DataDir  string
	Settings StageSettings
	WsPub    pipeline.WsPublisher
}

// ChainEnqueuer is the narrow view of *asynq.Client used to enqueue the
// next stage. Exposed as an interface so tests can inject a recorder.
type ChainEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// advanceStage updates regions.state + regions.state_detail for the
// given region. Writes are best-effort when DB is nil (tests that do
// not wire a DB).
func advanceStage(ctx context.Context, db *sql.DB, region, state, detail string) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`UPDATE regions SET state = ?, state_detail = ? WHERE name = ?`,
		state, detail, region)
	return err
}

// markStageFailed writes regions.state=failed + regions.last_error, and
// if a jobID is given also closes the jobs row with state=failed.
func markStageFailed(ctx context.Context, db *sql.DB, region, jobID, errMsg string) {
	if db == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx,
		`UPDATE regions SET state = 'failed', last_error = ? WHERE name = ?`,
		errMsg, region); err != nil {
		// Swallow — caller already returns an error that Asynq will log.
		_ = err
	}
	if jobID != "" {
		_, _ = db.ExecContext(ctx,
			`UPDATE jobs SET state = 'failed', error = ?, finished_at = ? WHERE id = ?`,
			errMsg, now, jobID)
	}
}

// enqueueNextStage enqueues the next pipeline kind with the same
// region + jobID as the current stage. Returns nil if Queue is nil
// (test contexts).
func enqueueNextStage(ctx context.Context, queue ChainEnqueuer, nextKind, region, jobID string) error {
	if queue == nil {
		return nil
	}
	payload, err := json.Marshal(jobs.PipelineStagePayload{
		Region: region, JobID: jobID, ParentJobID: jobID,
	})
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", nextKind, err)
	}
	if _, err := queue.EnqueueContext(ctx, asynq.NewTask(nextKind, payload)); err != nil {
		return fmt.Errorf("enqueue %s: %w", nextKind, err)
	}
	return nil
}

// enqueueSwap enqueues the terminal KindRegionSwap task after the last
// pipeline stage. Uses RegionSwapPayload (no ParentJobID).
func enqueueSwap(ctx context.Context, queue ChainEnqueuer, region, jobID string) error {
	if queue == nil {
		return nil
	}
	payload, err := json.Marshal(jobs.RegionSwapPayload{Region: region, JobID: jobID})
	if err != nil {
		return fmt.Errorf("marshal swap payload: %w", err)
	}
	if _, err := queue.EnqueueContext(ctx, asynq.NewTask(jobs.KindRegionSwap, payload)); err != nil {
		return fmt.Errorf("enqueue %s: %w", jobs.KindRegionSwap, err)
	}
	return nil
}

// decodeStagePayload is a small convenience used by every F/G/H handler.
func decodeStagePayload(t *asynq.Task, log zerolog.Logger) (jobs.PipelineStagePayload, error) {
	var p jobs.PipelineStagePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return p, fmt.Errorf("decode payload: %w", err)
	}
	if p.Region == "" {
		return p, errors.New("empty region")
	}
	return p, nil
}

// StageWork is the unit of work a pipeline stage executes. It receives
// the per-region ".new" directory and the decoded payload. Returning
// nil signals success; any error flips the region to failed and
// propagates to Asynq for retry.
//
// The ctx carries the *asynq.Task under stageTaskCtxKey (stageHandler
// sets it) so StageWork can reach `t.ResultWriter()` for live Asynq
// progress without a signature change. See StageTask / newStageProgress.
type StageWork func(ctx context.Context, regionDir string, p jobs.PipelineStagePayload, log zerolog.Logger) error

// SwapWork mirrors StageWork for the terminal swap stage (different
// payload shape).
type SwapWork func(ctx context.Context, deps ChainDeps, p jobs.RegionSwapPayload, log zerolog.Logger) error

// stageHandler is the canonical pipeline:* Asynq handler. It:
//
//  1. decodes PipelineStagePayload
//  2. advances regions.state to processingState
//  3. runs `work`
//  4. on success, enqueues nextKind (or RegionSwap as a terminal)
//  5. on failure, marks the region + job failed and returns the error
//
// Every stage flows through here so the state machine + chain-forward
// semantics are identical across agents F / G / H.
func stageHandler(deps ChainDeps, stageName, processingState, nextKind string, work StageWork, log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		p, err := decodeStagePayload(t, log)
		if err != nil {
			return fmt.Errorf("pipeline:%s: %w", stageName, err)
		}
		l := log.With().Str("module", "pipeline."+stageName).
			Str("region", p.Region).Str("jobId", p.JobID).Logger()

		if err := advanceStage(ctx, deps.DB, p.Region, processingState, stageName+" in progress"); err != nil {
			return fmt.Errorf("advance %s: %w", stageName, err)
		}
		regionDir := fmt.Sprintf("%s/regions/%s.new", deps.DataDir, p.Region)
		stageCtx := context.WithValue(ctx, stageTaskCtxKey{}, t)

		if err := work(stageCtx, regionDir, p, l); err != nil {
			markStageFailed(ctx, deps.DB, p.Region, p.JobID, err.Error())
			l.Error().Err(err).Str("stage", stageName).Msg("pipeline stage failed")
			return err
		}
		l.Info().Str("stage", stageName).Str("next", nextKind).Msg("stage succeeded; chaining")
		if nextKind == jobs.KindRegionSwap {
			return enqueueSwap(ctx, deps.Queue, p.Region, p.JobID)
		}
		return enqueueNextStage(ctx, deps.Queue, nextKind, p.Region, p.JobID)
	}
}

// swapHandler is the terminal KindRegionSwap Asynq handler. It
// decodes RegionSwapPayload and runs `work`; success flips the region
// to state=ready.
func swapHandler(deps ChainDeps, work SwapWork, log zerolog.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var p jobs.RegionSwapPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("swap: decode payload: %w", err)
		}
		if p.Region == "" {
			return errors.New("swap: empty region")
		}
		l := log.With().Str("module", "region.swap").
			Str("region", p.Region).Str("jobId", p.JobID).Logger()

		if err := advanceStage(ctx, deps.DB, p.Region, "updating", "swapping region directory"); err != nil {
			return fmt.Errorf("swap: advance state: %w", err)
		}
		if err := work(ctx, deps, p, l); err != nil {
			markStageFailed(ctx, deps.DB, p.Region, p.JobID, err.Error())
			return err
		}
		if err := finishSwap(ctx, deps.DB, p.Region, p.JobID); err != nil {
			return fmt.Errorf("swap: finalise: %w", err)
		}
		l.Info().Msg("region ready")
		return nil
	}
}

// finishSwap writes the terminal success state: region.state=ready,
// region.installed_at + last_updated_at = now, jobs.state=succeeded.
func finishSwap(ctx context.Context, db *sql.DB, region, jobID string) error {
	if db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE regions SET state = 'ready',
		 installed_at = COALESCE(installed_at, ?),
		 last_updated_at = ?,
		 last_error = NULL,
		 active_job_id = NULL,
		 state_detail = 'ready'
		 WHERE name = ?`, now, now, region); err != nil {
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
