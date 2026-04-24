// update_check.go — per-region "is the upstream extract newer than what
// we installed?" decision. Geofabrik publishes an .md5 sidecar next to
// every pbf; we compare that to regions.source_pbf_sha256 (which stores
// the md5 — see internal/geofabrik/fetch.go for the naming note).

package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/packalares/localmaps/internal/catalog"
	"github.com/packalares/localmaps/internal/geofabrik"
)

// UpdateReason is a short tag for logs and, transitively, for the
// scheduler's tick summary.
type UpdateReason string

const (
	// ReasonUpToDate means the region's source_pbf_sha256 matches the
	// current upstream md5. Scheduler will only bump next_update_at.
	ReasonUpToDate UpdateReason = "up to date"
	// ReasonChecksumChanged means the upstream md5 differs from the
	// installed one. Scheduler enqueues a KindRegionUpdate.
	ReasonChecksumChanged UpdateReason = "upstream checksum changed"
	// ReasonMissingChecksum means we've never recorded an md5 for this
	// region (fresh install that didn't run through the normal chain).
	// Treat as "should update" to converge state.
	ReasonMissingChecksum UpdateReason = "no installed checksum"
	// ReasonUnknown is used when any dependency returned an error and
	// the scheduler should retry on the next tick.
	ReasonUnknown UpdateReason = "unknown"
)

// ChecksumFetcher is the narrow view of geofabrik.Client needed for the
// update check. We accept an interface so tests can feed canned md5
// responses without spinning an httptest server around every case.
type ChecksumFetcher interface {
	ResolvePbfURL(entry catalog.Entry) (string, error)
	FetchSHA256(ctx context.Context, pbfURL string) (string, int64, error)
}

// Ensure *geofabrik.Client satisfies the interface at build time.
var _ ChecksumFetcher = (*geofabrik.Client)(nil)

// UpdateCheck compares the installed md5 against the upstream .md5
// sidecar. The zero value is unusable; callers must populate DB,
// Catalog, and Fetcher.
type UpdateCheck struct {
	// DB is the shared SQLite /data/config.db handle.
	DB *sql.DB
	// Catalog resolves a canonical region key to its catalog entry so
	// we can derive the pbf URL.
	Catalog catalog.Reader
	// Fetcher pulls the .md5 sidecar for that URL. Usually a
	// *geofabrik.Client built by the worker main.
	Fetcher ChecksumFetcher
}

// ShouldUpdate reports whether a fresh pipeline should run for the
// given region. The boolean is true only when we have high confidence
// an update is warranted (md5 changed or we never recorded one).
// Any transient error classifies as (false, ReasonUnknown, err) so the
// scheduler tick advances next_update_at and logs the failure.
func (u UpdateCheck) ShouldUpdate(ctx context.Context, region string) (bool, UpdateReason, error) {
	if u.DB == nil || u.Catalog == nil || u.Fetcher == nil {
		return false, ReasonUnknown, errors.New("scheduler: UpdateCheck not fully wired")
	}
	installed, err := readInstalledMD5(ctx, u.DB, region)
	if err != nil {
		return false, ReasonUnknown, fmt.Errorf("read installed md5: %w", err)
	}
	entry, err := u.Catalog.Resolve(ctx, region)
	if err != nil {
		return false, ReasonUnknown, fmt.Errorf("resolve catalog entry: %w", err)
	}
	pbfURL, err := u.Fetcher.ResolvePbfURL(entry)
	if err != nil {
		return false, ReasonUnknown, fmt.Errorf("resolve pbf url: %w", err)
	}
	upstream, _, err := u.Fetcher.FetchSHA256(ctx, pbfURL)
	if err != nil {
		return false, ReasonUnknown, fmt.Errorf("fetch upstream md5: %w", err)
	}
	upstream = strings.ToLower(strings.TrimSpace(upstream))
	if upstream == "" {
		// Server didn't publish a sidecar. Conservative: no update.
		// Scheduler will retry on the next tick — the sidecar is
		// frequently republished after a fresh extract run.
		return false, ReasonUnknown, errors.New("upstream md5 sidecar missing")
	}
	if installed == "" {
		return true, ReasonMissingChecksum, nil
	}
	if !strings.EqualFold(upstream, installed) {
		return true, ReasonChecksumChanged, nil
	}
	return false, ReasonUpToDate, nil
}

// readInstalledMD5 returns the stored md5 (the source_pbf_sha256 column
// despite the name stores md5) for the given canonical region key.
// Returns an empty string — no error — when the row exists but the
// column is NULL.
func readInstalledMD5(ctx context.Context, db *sql.DB, region string) (string, error) {
	var sum sql.NullString
	row := db.QueryRowContext(ctx,
		`SELECT source_pbf_sha256 FROM regions WHERE name = ?`, region)
	if err := row.Scan(&sum); err != nil {
		return "", err
	}
	if !sum.Valid {
		return "", nil
	}
	return strings.ToLower(strings.TrimSpace(sum.String)), nil
}
