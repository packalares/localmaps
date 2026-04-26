package api

// Handlers for the `jobs` tag in contracts/openapi.yaml. The WS
// handler is wired from router.go to the ws.Hub directly.

import (
	"database/sql"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/jmoiron/sqlx"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/config"
)

var jobsDB *sqlx.DB

// setJobsStore wires the SQLite handle used by the jobs handler.
// Called from router.go after Deps is assembled.
func setJobsStore(s *config.Store) {
	if s == nil {
		jobsDB = nil
		return
	}
	jobsDB = s.DB()
}

// GET /api/jobs/{jobId}
func jobsGetHandler(c fiber.Ctx) error {
	if jobsDB == nil {
		return notImplemented(c)
	}
	id := strings.TrimSpace(c.Params("jobId"))
	if id == "" {
		return apierr.Write(c, apierr.CodeBadRequest, "missing jobId", false)
	}

	row := jobsDB.QueryRowContext(c.Context(), `
		SELECT id, kind, region, state, progress, message,
		       started_at, finished_at, error, parent_job_id
		FROM jobs WHERE id = ?`, id)

	var (
		jobID, kind, state                                          string
		region, message, startedAt, finishedAt, errStr, parentJobID sql.NullString
		progress                                                    sql.NullFloat64
	)
	if err := row.Scan(&jobID, &kind, &region, &state,
		&progress, &message, &startedAt, &finishedAt, &errStr, &parentJobID); err != nil {
		if err == sql.ErrNoRows {
			return apierr.Write(c, apierr.CodeJobNotFound, "job not found", false)
		}
		return apierr.Write(c, apierr.CodeInternal, "db error", true)
	}

	body := map[string]any{
		"id":    jobID,
		"kind":  kind,
		"state": state,
	}
	if region.Valid {
		body["region"] = region.String
	}
	if progress.Valid {
		body["progress"] = progress.Float64
	}
	if message.Valid {
		body["message"] = message.String
	}
	if startedAt.Valid {
		body["startedAt"] = startedAt.String
	}
	if finishedAt.Valid {
		body["finishedAt"] = finishedAt.String
	}
	if errStr.Valid {
		body["error"] = errStr.String
	}
	if parentJobID.Valid {
		body["parentJobId"] = parentJobID.String
	}
	return c.JSON(body)
}
