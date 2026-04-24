package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ManifestFileName is the canonical manifest filename inside a region
// directory, per docs/04-data-model.md.
const ManifestFileName = "manifest.json"

// TilesSection is the planetiler-owned block inside manifest.json.
// Each pipeline stage (tiles, routing, geocoding, poi) owns its own
// section; other stages' sections MUST be preserved on update — see
// Merge().
type TilesSection struct {
	SourceURL            string    `json:"sourceUrl"`
	SourceSHA256         string    `json:"sourceSha256,omitempty"`
	SourceBytes          int64     `json:"sourceBytes,omitempty"`
	BuiltAt              time.Time `json:"builtAt"`
	BuildDurationSeconds float64   `json:"buildDurationSeconds"`
	Tool                 string    `json:"tool"`
	ToolVersion          string    `json:"toolVersion,omitempty"`
	OutputFile           string    `json:"outputFile"`
	OutputBytes          int64     `json:"outputBytes"`
	OutputTileCount      int64     `json:"outputTileCount,omitempty"`
}

// RoutingSection is the valhalla-owned block.
type RoutingSection struct {
	BuiltAt              time.Time `json:"builtAt"`
	BuildDurationSeconds float64   `json:"buildDurationSeconds,omitempty"`
	Tool                 string    `json:"tool"`
	ToolVersion          string    `json:"toolVersion,omitempty"`
	TileDir              string    `json:"tileDir"`
	TarPath              string    `json:"tarPath,omitempty"`
	AdminDB              string    `json:"adminDb,omitempty"`
	TimezoneDB           string    `json:"timezoneDb,omitempty"`
}

// GeocodingSection is the pelias-owned block.
type GeocodingSection struct {
	BuiltAt              time.Time `json:"builtAt"`
	BuildDurationSeconds float64   `json:"buildDurationSeconds,omitempty"`
	Tool                 string    `json:"tool"`
	ToolVersion          string    `json:"toolVersion,omitempty"`
	IndexName            string    `json:"indexName"`
	ESHost               string    `json:"esHost,omitempty"`
}

// POISection is the overture/DuckDB-owned block (wired by Phase 2+
// POI agent; structure stable now so manifest layout is fixed).
type POISection struct {
	BuiltAt              time.Time `json:"builtAt"`
	BuildDurationSeconds float64   `json:"buildDurationSeconds,omitempty"`
	Tool                 string    `json:"tool"`
	Source               string    `json:"source,omitempty"`
	OutputDir            string    `json:"outputDir"`
	Features             int64     `json:"features,omitempty"`
}

// Manifest is the root of <regionDir>/manifest.json. Each Phase 2 stage
// owns one section; unknown fields are preserved on round-trip so
// stages never clobber each other.
type Manifest struct {
	Region    string            `json:"region"`
	Version   int               `json:"version"`
	Tiles     *TilesSection     `json:"tiles,omitempty"`
	Routing   *RoutingSection   `json:"routing,omitempty"`
	Geocoding *GeocodingSection `json:"geocoding,omitempty"`
	POI       *POISection       `json:"poi,omitempty"`

	// raw holds fields not modelled above so Merge can round-trip them
	// untouched. Populated only by ReadManifest.
	raw map[string]json.RawMessage
}

// ManifestVersion is the on-disk manifest schema version. Bump when
// stages reshape their sections.
const ManifestVersion = 1

// ReadManifest loads <dir>/manifest.json. Returns (nil, nil) if the
// file does not yet exist — callers treat that as "first stage to
// write this region".
func ReadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, ManifestFileName)
	data, err := os.ReadFile(path) // #nosec G304 -- dir is caller-controlled
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	// Keep a raw copy for Merge to preserve unknown sections.
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode manifest raw: %w", err)
	}
	m.raw = raw
	return m, nil
}

// WriteManifest atomically writes <dir>/manifest.json. It writes to a
// same-directory temp file and renames into place so a crash leaves
// either the old or new content, never a truncated file.
func WriteManifest(dir string, m *Manifest) error {
	if m == nil {
		return errors.New("manifest: nil")
	}
	if m.Version == 0 {
		m.Version = ManifestVersion
	}
	out, err := mergeForWrite(m)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir manifest dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".manifest.*.json")
	if err != nil {
		return fmt.Errorf("temp manifest: %w", err)
	}
	tmpPath := tmp.Name()
	if _, werr := tmp.Write(b); werr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp manifest: %w", werr)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fsync manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp manifest: %w", err)
	}
	final := filepath.Join(dir, ManifestFileName)
	if err := os.Rename(tmpPath, final); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename manifest: %w", err)
	}
	return nil
}

// UpdateTilesSection reads, mutates the tiles section, and atomically
// rewrites the manifest. Other stages' sections are preserved.
func UpdateTilesSection(dir, region string, tiles TilesSection) error {
	return updateSection(dir, region, func(m *Manifest) { t := tiles; m.Tiles = &t })
}

// UpdateRoutingSection persists the valhalla section.
func UpdateRoutingSection(dir, region string, routing RoutingSection) error {
	return updateSection(dir, region, func(m *Manifest) { r := routing; m.Routing = &r })
}

// UpdateGeocodingSection persists the pelias section.
func UpdateGeocodingSection(dir, region string, geo GeocodingSection) error {
	return updateSection(dir, region, func(m *Manifest) { g := geo; m.Geocoding = &g })
}

// UpdatePOISection persists the overture/poi section.
func UpdatePOISection(dir, region string, poi POISection) error {
	return updateSection(dir, region, func(m *Manifest) { p := poi; m.POI = &p })
}

func updateSection(dir, region string, mutate func(*Manifest)) error {
	m, err := ReadManifest(dir)
	if err != nil {
		return err
	}
	if m == nil {
		m = &Manifest{Region: region, Version: ManifestVersion}
	}
	if m.Region == "" {
		m.Region = region
	}
	mutate(m)
	return WriteManifest(dir, m)
}

// mergeForWrite produces the final JSON object, overlaying modelled
// sections onto any raw-preserved unknown fields so that future section
// types added upstream don't get clobbered here.
func mergeForWrite(m *Manifest) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range m.raw {
		if knownTopLevelKey(k) {
			continue
		}
		var decoded any
		if err := json.Unmarshal(v, &decoded); err != nil {
			return nil, fmt.Errorf("preserve field %q: %w", k, err)
		}
		out[k] = decoded
	}
	out["region"] = m.Region
	out["version"] = m.Version
	if m.Tiles != nil {
		out["tiles"] = m.Tiles
	}
	if m.Routing != nil {
		out["routing"] = m.Routing
	}
	if m.Geocoding != nil {
		out["geocoding"] = m.Geocoding
	}
	if m.POI != nil {
		out["poi"] = m.POI
	}
	return out, nil
}

func knownTopLevelKey(k string) bool {
	switch k {
	case "region", "version", "tiles", "routing", "geocoding", "poi":
		return true
	}
	return false
}
