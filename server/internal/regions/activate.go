package regions

// activate.go implements per-region routing selection. The admin picks
// an installed region as the "active" one; the gateway records that in
// the settings table and drops a pointer file beneath the data dir
// where the Valhalla container's startup loop polls for it.
//
// Layering:
//   * The settings row at `routing.activeRegion` is the source of
//     truth (UI reads it; PUT /api/settings can also write it).
//   * The pointer file is a side-effect copy that decouples the
//     Valhalla container from the gateway DB — Valhalla only needs
//     read access to the shared hostPath.
//
// File path: <DataDir>/regions/.active-region
//   - Same hostPath subPath ("regions") that Valhalla mounts at
//     /valhalla-tiles, so from the routing container's POV the file
//     is /valhalla-tiles/.active-region.
//   - Single-line content: the canonical region key, no trailing nl.
//   - Empty / missing → Valhalla falls back to the largest tar.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ActiveRegionFileName is the dotfile Valhalla polls for the active
// region key. Lives beside the per-region tile dirs, never inside one.
const ActiveRegionFileName = ".active-region"

// activeRegionSettingKey is the settings row that mirrors the file on
// disk. Lives under the same `routing.*` namespace as the rest of the
// routing configuration.
const activeRegionSettingKey = "routing.activeRegion"

// Activate flips the active routing region. The region must exist and
// be in the `ready` state (no point pointing Valhalla at half-built
// tiles). Persists the choice in `routing.activeRegion` AND writes the
// pointer file at <DataDir>/regions/.active-region. Both writes are
// best-effort independent: if the file write fails, the settings row
// still wins on the next gateway restart, but the operator gets an
// error so they can investigate.
func (s *Service) Activate(ctx context.Context, input, triggeredBy string) (Region, error) {
	canonical, err := ensureValidRegionName(input)
	if err != nil {
		return Region{}, fmt.Errorf("validate name: %w", err)
	}
	existing, err := s.Get(ctx, canonical)
	if err != nil {
		return Region{}, err
	}
	if existing.State != StateReady {
		return Region{}, fmt.Errorf("%w: state=%s (must be ready)",
			ErrConflict, existing.State)
	}
	if err := s.writeActiveRegionSetting(ctx, canonical, triggeredBy); err != nil {
		return Region{}, fmt.Errorf("persist active region: %w", err)
	}
	if err := s.writeActiveRegionFile(canonical); err != nil {
		// File-side failure is surfaced — Valhalla won't reload until
		// it sees the new content.
		return existing, fmt.Errorf("write active-region file: %w", err)
	}
	return existing, nil
}

// writeActiveRegionSetting upserts the settings row. Mirrors the
// shape of the settings PATCH path (json-encoded value, updated_at +
// updated_by columns).
func (s *Service) writeActiveRegionSetting(ctx context.Context, canonical, user string) error {
	if user == "" {
		user = "system"
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return err
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at, updated_by)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		     value = excluded.value,
		     updated_at = excluded.updated_at,
		     updated_by = excluded.updated_by`,
		activeRegionSettingKey, string(body), ts, user)
	return err
}

// writeActiveRegionFile drops the canonical key into the pointer file.
// The file is written atomically (temp + rename) so the Valhalla
// poller never sees a partial write. dataDir == "" disables the write
// (tests / in-memory boot).
func (s *Service) writeActiveRegionFile(canonical string) error {
	if s.dataDir == "" {
		return nil
	}
	dir := filepath.Join(s.dataDir, "regions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(dir, ActiveRegionFileName)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(canonical), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ReadActiveRegionFile reads the pointer file back, trimming
// whitespace. Returns ("", nil) if the file is missing or the dir is
// empty — the caller treats that as "fall back to default".
func ReadActiveRegionFile(dataDir string) (string, error) {
	if dataDir == "" {
		return "", nil
	}
	target := filepath.Join(dataDir, "regions", ActiveRegionFileName)
	b, err := os.ReadFile(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
