// handlers_agent_h.go — Agent H's pipeline:geocoding StageWork.
// Registered through registerAgentEHandlers → stageHandler wrapping.
//
// Indexing runs in-process: we read source.osm.pbf with paulmach/osm,
// project the features pelias-api cares about (POIs, localities, named
// streets) onto pelias's document schema, and bulk-POST them to the
// pelias-es _bulk endpoint. No docker socket + no Node importer
// sidecar required, so this works uniformly under docker-compose and
// under Kubernetes (docs/10-deploy.md).
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/jobs"
	"github.com/packalares/localmaps/worker/internal/pipeline"
	"github.com/packalares/localmaps/worker/internal/pipeline/peliasindex"
)

// geocodingWork returns the real `pipeline:geocoding` StageWork. It
// calls peliasindex.Build against the region's source.osm.pbf and
// bulk-indexes pelias-es directly. On success the manifest is updated
// with the pelias index metadata; on failure the error propagates up
// to stageHandler which marks the region failed + Asynq retries.
func geocodingWork(deps ChainDeps) StageWork {
	return func(ctx context.Context, regionDir string, p jobs.PipelineStagePayload, log zerolog.Logger) error {
		// The chain always hands us `<root>/regions/<region>.new`. On a
		// retry after the swap already ran (earlier DNS error path), that
		// dir is gone and the pbf is in the live dir. Fall back to it.
		pbfPath := filepath.Join(regionDir, "source.osm.pbf")
		if _, err := os.Stat(pbfPath); err != nil {
			liveDir := strings.TrimSuffix(regionDir, ".new")
			if liveDir != regionDir {
				alt := filepath.Join(liveDir, "source.osm.pbf")
				if _, altErr := os.Stat(alt); altErr == nil {
					log.Info().Str("pbf", alt).Msg("pelias import: using live pbf (.new already swapped)")
					pbfPath = alt
					regionDir = liveDir
				} else {
					return fmt.Errorf("pelias import: source.osm.pbf missing at %s and %s: %w", pbfPath, alt, err)
				}
			} else {
				return fmt.Errorf("pelias import: source.osm.pbf missing at %s: %w", pbfPath, err)
			}
		}
		esURL := peliasESURL(deps.Settings)
		indexName := fmt.Sprintf("pelias-%s-%s", p.Region, time.Now().UTC().Format("20060102"))
		// pelias-api hard-codes the index name "pelias" (see
		// deploy/pelias/pelias.json + pelias's schema/esclient). We
		// honour that by writing docs into the `pelias` alias/index; the
		// per-region `indexName` is surfaced only in the manifest for
		// human audit.
		batchSize := settingIntOrDefault(deps.Settings, "search.peliasBatchSize", 1000)
		timeoutMin := settingIntOrDefault(deps.Settings, "search.peliasBuildTimeoutMinutes", 120)

		runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMin)*time.Minute)
		defer cancel()

		log.Info().
			Str("region", p.Region).
			Str("pbf", pbfPath).
			Str("esURL", esURL).
			Str("indexAlias", "pelias").
			Str("indexName", indexName).
			Int("batchSize", batchSize).
			Msg("pelias in-process import starting")

		start := time.Now()
		stats, err := peliasindex.BuildWithOptions(runCtx, pbfPath, esURL, p.Region,
			peliasindex.Options{BatchSize: batchSize}, log)
		if err != nil {
			return fmt.Errorf("pelias import: %w", err)
		}
		log.Info().
			Int64("docs", stats.DocsIndexed).
			Int64("nodes", stats.NodesSeen).
			Int64("ways", stats.WaysSeen).
			Dur("duration", stats.Duration).
			Msg("pelias in-process import complete")

		return pipeline.UpdateGeocodingSection(regionDir, p.Region, pipeline.GeocodingSection{
			BuiltAt:              time.Now().UTC(),
			BuildDurationSeconds: time.Since(start).Seconds(),
			Tool:                 "peliasindex",
			ToolVersion:          "in-process/go",
			IndexName:            indexName,
			ESHost:               esURL,
		})
	}
}

// peliasESURL assembles the Elasticsearch base URL from settings. It
// prefers settings.search.peliasElasticUrl (as persisted by the admin
// UI); falls back to LOCALMAPS_PELIAS_ES_URL, then to the docker-compose
// default http://pelias-es:9200 (docs/07-config-schema.md).
func peliasESURL(s StageSettings) string {
	if raw := settingOrDefault(s, "search.peliasElasticUrl", ""); raw != "" {
		if normalised, ok := normalisePeliasURL(raw); ok {
			return normalised
		}
	}
	if raw := os.Getenv("LOCALMAPS_PELIAS_ES_URL"); raw != "" {
		if normalised, ok := normalisePeliasURL(raw); ok {
			return normalised
		}
	}
	return "http://pelias-es:9200"
}

// normalisePeliasURL strips whitespace + trailing slash, sets a default
// port (9200) when the URL has none, and returns (url, false) on a
// parse failure so the caller can fall back.
func normalisePeliasURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	host := u.Hostname()
	port := 9200
	if p := u.Port(); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			port = v
		}
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port), true
}

// settingBoolOrDefault mirrors settingOrDefault / settingIntOrDefault
// for boolean settings keys. Kept here because handlers_agent_fg.go
// does not export it.
func settingBoolOrDefault(s StageSettings, key string, def bool) bool {
	if s == nil {
		return def
	}
	if v, err := s.GetBool(key); err == nil {
		return v
	}
	return def
}
