package regions

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/packalares/localmaps/internal/catalog"
	sharedregions "github.com/packalares/localmaps/internal/regions"
)

// Region states — values MUST match the RegionState enum in
// contracts/openapi.yaml.
const (
	StateNotInstalled       = "not_installed"
	StateDownloading        = "downloading"
	StateProcessingTiles    = "processing_tiles"
	StateProcessingRouting  = "processing_routing"
	StateProcessingGeocoding = "processing_geocoding"
	StateProcessingPOI      = "processing_poi"
	StateReady              = "ready"
	StateUpdating           = "updating"
	StateFailed             = "failed"
	StateArchived           = "archived"
)

// Valid schedule enum values for the REST API input.
const (
	ScheduleNever   = "never"
	ScheduleDaily   = "daily"
	ScheduleWeekly  = "weekly"
	ScheduleMonthly = "monthly"
)

// Region mirrors the Region schema from contracts/openapi.yaml. JSON
// tags match field-for-field — do not rename.
type Region struct {
	Name            string   `json:"name" db:"name"`
	DisplayName     string   `json:"displayName" db:"display_name"`
	Parent          *string  `json:"parent,omitempty" db:"parent"`
	SourceURL       string   `json:"sourceUrl" db:"source_url"`
	SourcePbfSha256 *string  `json:"sourcePbfSha256,omitempty" db:"source_pbf_sha256"`
	SourcePbfBytes  *int64   `json:"sourcePbfBytes,omitempty" db:"source_pbf_bytes"`
	BBox            *[]float64 `json:"bbox,omitempty" db:"-"`
	BBoxRaw         *string  `json:"-" db:"bbox"`
	State           string   `json:"state" db:"state"`
	StateDetail     *string  `json:"stateDetail,omitempty" db:"state_detail"`
	LastError       *string  `json:"lastError,omitempty" db:"last_error"`
	InstalledAt     *string  `json:"installedAt,omitempty" db:"installed_at"`
	LastUpdatedAt   *string  `json:"lastUpdatedAt,omitempty" db:"last_updated_at"`
	NextUpdateAt    *string  `json:"nextUpdateAt,omitempty" db:"next_update_at"`
	Schedule        *string  `json:"schedule,omitempty" db:"schedule"`
	DiskBytes       *int64   `json:"diskBytes,omitempty" db:"disk_bytes"`
	ActiveJobID     *string  `json:"activeJobId,omitempty" db:"active_job_id"`
}

// hydrateBBox populates BBox from the stored JSON string in BBoxRaw.
// Returns nil if BBoxRaw is nil; otherwise decodes the JSON array.
func (r *Region) hydrateBBox() error {
	r.BBox = nil
	if r.BBoxRaw == nil || *r.BBoxRaw == "" {
		return nil
	}
	var arr []float64
	if err := json.Unmarshal([]byte(*r.BBoxRaw), &arr); err != nil {
		return err
	}
	if len(arr) != 4 {
		return nil
	}
	r.BBox = &arr
	return nil
}

// newRegionFromCatalog builds a Region row from a catalog entry, ready
// for INSERT. The state starts as StateNotInstalled; the caller flips
// it to StateDownloading when enqueuing the install.
func newRegionFromCatalog(entry catalog.Entry) Region {
	r := Region{
		Name:        entry.Name,
		DisplayName: entry.DisplayName,
		SourceURL:   entry.SourceURL,
		State:       StateNotInstalled,
	}
	if entry.Parent != nil {
		p := *entry.Parent
		r.Parent = &p
	}
	if entry.SourcePbfBytes != nil {
		n := *entry.SourcePbfBytes
		r.SourcePbfBytes = &n
	}
	return r
}

// Job mirrors the Job schema from contracts/openapi.yaml. Kept minimal —
// the handlers return the row directly.
type Job struct {
	ID           string   `json:"id" db:"id"`
	Kind         string   `json:"kind" db:"kind"`
	Region       *string  `json:"region,omitempty" db:"region"`
	State        string   `json:"state" db:"state"`
	Progress     *float64 `json:"progress,omitempty" db:"progress"`
	Message      *string  `json:"message,omitempty" db:"message"`
	StartedAt    *string  `json:"startedAt,omitempty" db:"started_at"`
	FinishedAt   *string  `json:"finishedAt,omitempty" db:"finished_at"`
	Error        *string  `json:"error,omitempty" db:"error"`
	ParentJobID  *string  `json:"parentJobId,omitempty" db:"parent_job_id"`
	CreatedBy    *string  `json:"-" db:"created_by"`
}

// nowRFC3339 returns the current time formatted for the DB columns.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// strPtr is a tiny helper for the optional TEXT columns.
func strPtr(s string) *string { return &s }

// ensureValidRegionName runs the shared normaliser and reports whether
// the result is canonical. Returns (canonical, nil) on success.
func ensureValidRegionName(input string) (string, error) {
	return sharedregions.NormaliseKey(input)
}

// scanRegion is a convenience wrapper for sql.Rows.Scan-ish callers to
// post-process. Not currently used outside tests.
func scanRegion(r *Region, src *sql.NullString) {
	if src != nil && src.Valid {
		r.BBoxRaw = &src.String
		_ = r.hydrateBBox()
	}
}
