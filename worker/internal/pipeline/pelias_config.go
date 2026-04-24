// Package pipeline — pelias_config.go generates the `pelias.json` file
// consumed by the Pelias openstreetmap importer. The shape mirrors
// upstream's sample (see deploy/pelias/pelias.json and
// https://github.com/pelias/docker) — do NOT invent keys. We emit only
// the minimum surface the importer needs: esclient, logger,
// imports.openstreetmap, imports.polylines (when enabled), and a
// minimal api block so the JSON round-trips through the pelias config
// loader.
package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
)

// ImportConfig describes a single per-region Pelias import run. Values
// come from the job payload + the settings tree (see
// docs/07-config-schema.md `search.pelias*`). The Go worker never talks
// to Elasticsearch directly — the importer container does. Go only
// generates the config and shells out.
type ImportConfig struct {
	// Region is the canonical hyphenated key (e.g. "europe-romania").
	Region string
	// PbfPath is the absolute path to the downloaded .osm.pbf INSIDE
	// the importer container (i.e. after the volume mount). Typically
	// "/data/source.osm.pbf".
	PbfPath string
	// ESHost + ESPort are parsed out of settings.search.peliasElasticUrl.
	ESHost string
	ESPort int
	// IndexName is the Elasticsearch alias+index the importer targets.
	// Convention: "pelias-<region>-<YYYYMMDD>".
	IndexName string
	// Languages is the acceptLanguage list; defaults to ["en"] upstream.
	Languages []string
	// PolylinesEnabled toggles the polylines importer (usually off for
	// city-scale regions; on for full country+ extracts where road
	// shapes help address interpolation).
	PolylinesEnabled bool
}

// GeneratePeliasJSON renders cfg to the pelias.json byte representation
// the upstream `./bin/start` loader expects. The output is pretty-printed
// with two-space indent so golden-file diffs stay reviewable.
func GeneratePeliasJSON(cfg ImportConfig) ([]byte, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	esHost := cfg.ESHost
	esPort := cfg.ESPort

	// datapath is the directory the importer reads the pbf from; the
	// filename key names the specific extract.
	pbfDir, pbfFile := path.Split(cfg.PbfPath)
	if pbfDir == "" {
		pbfDir = "/data/"
	}

	out := map[string]any{
		"logger": map[string]any{
			"level":     "info",
			"timestamp": true,
		},
		"esclient": map[string]any{
			// keepAlive:false avoids the importer hanging when the
			// server side closes an idle pooled connection (ES 7.17
			// default idle is 60s, importer batches can exceed that).
			"keepAlive":  false,
			"apiVersion": "7.5",
			"hosts": []map[string]any{
				{
					"host":     esHost,
					"port":     esPort,
					"protocol": "http",
				},
			},
		},
		"acceptLanguage": true,
		"api": map[string]any{
			// Minimal api block — only fields the importer reads at
			// load time. Full runtime config lives in deploy/pelias/pelias.json
			// for pelias-api itself.
			"indexName": cfg.IndexName,
			"languages": sortedCopy(cfg.Languages),
		},
		"imports": map[string]any{
			"openstreetmap": map[string]any{
				"datapath":    trimTrailingSlash(pbfDir),
				"leveldbpath": "/data/tmp",
				"import": []map[string]any{
					{"filename": pbfFile},
				},
			},
			"adminLookup": map[string]any{
				"enabled": false,
			},
		},
	}

	if cfg.PolylinesEnabled {
		imports := out["imports"].(map[string]any)
		imports["polylines"] = map[string]any{
			"datapath": "/data/polylines",
			"files":    []string{fmt.Sprintf("%s.polylines", cfg.Region)},
		}
	}

	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal pelias.json: %w", err)
	}
	// Trailing newline — matches upstream sample + keeps POSIX tools
	// happy on golden diffs.
	return append(buf, '\n'), nil
}

func (c ImportConfig) validate() error {
	if c.Region == "" {
		return errors.New("pelias config: region required")
	}
	if c.PbfPath == "" {
		return errors.New("pelias config: pbf path required")
	}
	if c.ESHost == "" {
		return errors.New("pelias config: ES host required")
	}
	if c.ESPort <= 0 || c.ESPort > 65535 {
		return fmt.Errorf("pelias config: ES port %d out of range", c.ESPort)
	}
	if c.IndexName == "" {
		return errors.New("pelias config: index name required")
	}
	return nil
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return []string{"en"}
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

func trimTrailingSlash(s string) string {
	if len(s) > 1 && s[len(s)-1] == '/' {
		return s[:len(s)-1]
	}
	return s
}
