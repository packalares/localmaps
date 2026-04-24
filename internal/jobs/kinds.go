// Package jobs declares the Asynq task kinds + their JSON payloads
// that flow between the gateway (enqueuer) and the worker (consumer).
// Every kind here has a matching handler registered in
// worker/cmd/worker/main.go.
//
// IMPORTANT: Asynq task kinds and openapi JobKind values are separate
// concepts. The jobs table's `kind` column uses the openapi enum
// (download_pbf, build_pmtiles, ...). Asynq's task type is an
// internal routing string — we use colon-namespaced identifiers here
// so logs and the queue UI make the orchestration explicit.
//
// Payloads are intentionally minimal — only fields actually used by
// handlers appear; invented fields are forbidden per
// docs/06-agent-rules.md R2.
package jobs

// Asynq task kinds. The "region:*" namespace wraps the end-to-end
// install/update/delete orchestration; the "pipeline:*" kinds are the
// per-stage build tasks run after the pbf is on disk.
const (
	// KindRegionInstall orchestrates the full install pipeline: download
	// the pbf, then fan out to pipeline:* stages, then swap.
	KindRegionInstall = "region:install"
	// KindRegionUpdate re-downloads + rebuilds into <region>.new and
	// swaps on success.
	KindRegionUpdate = "region:update"
	// KindRegionDelete archives + eventually removes a region from disk.
	KindRegionDelete = "region:delete"

	// KindPipelineTiles runs planetiler to produce map.pmtiles.
	KindPipelineTiles = "pipeline:tiles"
	// KindPipelineRouting runs valhalla_build_tiles to produce valhalla_tiles/.
	KindPipelineRouting = "pipeline:routing"
	// KindPipelineGeocoding runs the Pelias importer to populate pelias_index/.
	KindPipelineGeocoding = "pipeline:geocoding"
	// KindPipelinePOI fetches the Overture parquet subset for the region bbox.
	KindPipelinePOI = "pipeline:poi"

	// KindRegionSwap atomically renames <region>.new to <region>.
	KindRegionSwap = "region:swap"
)

// AllKinds enumerates every declared kind. Handy for ensuring the
// worker's mux has complete coverage.
func AllKinds() []string {
	return []string{
		KindRegionInstall,
		KindRegionUpdate,
		KindRegionDelete,
		KindPipelineTiles,
		KindPipelineRouting,
		KindPipelineGeocoding,
		KindPipelinePOI,
		KindRegionSwap,
	}
}

// RegionInstallPayload is the payload for KindRegionInstall /
// KindRegionUpdate. The canonical region key (see internal/regions) is
// the single identifier; the pbf URL + md5 are looked up from the
// catalog at handler time so a catalog refresh since enqueue doesn't
// leave us installing stale data.
type RegionInstallPayload struct {
	// Region is the canonical key ("europe-romania").
	Region string `json:"region"`
	// JobID is the row id in the jobs table this Asynq task belongs to.
	JobID string `json:"jobId"`
	// TriggeredBy is the authenticated user id or "scheduler".
	TriggeredBy string `json:"triggeredBy,omitempty"`
}

// RegionDeletePayload is the payload for KindRegionDelete. The worker
// removes on-disk artifacts then clears the regions row.
type RegionDeletePayload struct {
	Region      string `json:"region"`
	JobID       string `json:"jobId"`
	TriggeredBy string `json:"triggeredBy,omitempty"`
}

// RegionSwapPayload is the payload for KindRegionSwap. After all
// pipeline stages succeed, the worker renames <region>.new to
// <region>.
type RegionSwapPayload struct {
	Region string `json:"region"`
	JobID  string `json:"jobId"`
}

// PipelineStagePayload is the shape for every pipeline:* task. The
// worker reads source.osm.pbf from <dataDir>/regions/<region>.new/ and
// writes its stage-specific output into that same .new directory.
type PipelineStagePayload struct {
	Region string `json:"region"`
	JobID  string `json:"jobId"`
	// ParentJobID is the orchestrating region:install job; workers use
	// it to walk the chain when reporting progress.
	ParentJobID string `json:"parentJobId,omitempty"`
}

// --- openapi JobKind mapping ---------------------------------------

// These constants mirror contracts/openapi.yaml components/schemas/JobKind.
// They are the string values written to the jobs.kind column.
const (
	OpenAPIJobKindDownloadPBF   = "download_pbf"
	OpenAPIJobKindBuildPMTiles  = "build_pmtiles"
	OpenAPIJobKindBuildValhalla = "build_valhalla"
	OpenAPIJobKindBuildPelias   = "build_pelias"
	OpenAPIJobKindBuildOverture = "build_overture"
	OpenAPIJobKindSwapRegion    = "swap_region"
	OpenAPIJobKindUpdateRegion  = "update_region"
	OpenAPIJobKindArchiveRegion = "archive_region"
)
