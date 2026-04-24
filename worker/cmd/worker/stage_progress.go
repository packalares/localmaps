// stage_progress.go — helpers that bridge the Asynq stage handler to
// the pipeline.AsynqProgress reporter (Agent F) so every StageWork gets
// live progress emission into the jobs row + Asynq ResultWriter + WS
// hub without widening the StageWork signature.
//
// Split out of chain.go to respect the 250-line cap; every name here is
// referenced by chain.go and by the three pipeline StageWork funcs.
package main

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/worker/internal/pipeline"
)

// StageSettings is the narrow slice of the settings tree each StageWork
// reads. The concrete implementation is `sqlxSettings` in
// handlers_agent_e.go; tests may stub it out. Missing keys return an
// error — callers fall back to the schema defaults in
// docs/07-config-schema.md (R3 — no hardcoded values).
type StageSettings interface {
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
	GetStringSlice(key string) ([]string, error)
}

// stageTaskCtxKey is the private context key used to pipe the raw
// *asynq.Task into StageWork without widening its signature.
type stageTaskCtxKey struct{}

// StageTask retrieves the Asynq task bound to the current stage's ctx
// (set by stageHandler). Returns (nil, false) in test harnesses that
// call StageWork directly without going through stageHandler.
func StageTask(ctx context.Context) (*asynq.Task, bool) {
	t, ok := ctx.Value(stageTaskCtxKey{}).(*asynq.Task)
	return t, ok && t != nil
}

// newStageProgress returns a pipeline.ProgressReporter for the current
// stage. If the Asynq task is absent (tests) or DB unavailable, emits
// a Discard reporter — the StageWork still runs, just without live
// progress.
//
// jobID is pulled from the stage payload and passed explicitly so the
// row update in pipeline.AsynqProgress targets the right jobs row.
func newStageProgress(ctx context.Context, deps ChainDeps, jobID string, log zerolog.Logger) pipeline.ProgressReporter {
	t, ok := StageTask(ctx)
	if !ok {
		return pipeline.DiscardProgress{}
	}
	var writer *asynq.ResultWriter
	if t != nil {
		writer = t.ResultWriter()
	}
	var db *sqlx.DB
	if deps.DB != nil {
		db = sqlx.NewDb(deps.DB, "sqlite")
	}
	return pipeline.NewAsynqProgress(jobID, writer, db, deps.WsPub, log)
}
