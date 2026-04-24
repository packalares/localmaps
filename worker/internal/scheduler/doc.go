// Package scheduler drives per-region update cadence. It periodically
// scans the regions table, and for any region whose next_update_at is
// due, it consults the Geofabrik md5 sidecar to decide whether the
// extract on disk is still current. When an update is warranted, an
// Asynq task of kind KindRegionUpdate is enqueued; next_update_at is
// advanced either way so the policy matches regions.schedule.
//
// See docs/01-architecture.md (region lifecycle), docs/04-data-model.md
// (regions.schedule + regions.next_update_at), docs/07-config-schema.md
// (regions.updateCheckCron).
package scheduler
