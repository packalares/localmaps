// handlers_agent_h.go — Agent H's pipeline:geocoding StageWork.
// Registered through registerAgentEHandlers → stageHandler wrapping.
//
// The runner shells out to the Pelias openstreetmap importer. Under
// docker-compose we use `docker run` against `search.peliasImporterImage`
// with the region dir mounted at /data. Under Kubernetes (single
// pod with multiple containers) the docker socket is not mounted; the
// long-term path is a sidecar watcher over the shared /data volume,
// tracked as NEEDED in the agent report. For now we keep docker-run so
// the dev-loop works (docs/10-deploy.md, "Dev loop (single machine)").
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/packalares/localmaps/internal/jobs"
	"github.com/packalares/localmaps/worker/internal/pipeline"
)

// geocodingWork returns the real `pipeline:geocoding` StageWork. It
// writes a pelias.json into the shared workDir, then invokes the
// importer image against the region's .new directory. All knobs come
// from settings (R3).
func geocodingWork(deps ChainDeps) StageWork {
	return func(ctx context.Context, regionDir string, p jobs.PipelineStagePayload, log zerolog.Logger) error {
		workDir := filepath.Join(filepath.Dir(filepath.Dir(regionDir)), "tools", "pelias-runs", p.JobID)
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			return fmt.Errorf("mkdir pelias work: %w", err)
		}

		esHost, esPort := resolvePeliasES(deps.Settings)
		langs := settingArrOrDefault(deps.Settings, "search.peliasLanguages", []string{"en"})
		polylines := settingBoolOrDefault(deps.Settings, "search.peliasPolylinesEnabled", false)
		timeoutMin := settingIntOrDefault(deps.Settings, "search.peliasBuildTimeoutMinutes", 120)
		importerImage := settingOrDefault(deps.Settings, "search.peliasImporterImage", "pelias/openstreetmap:6.4.0")

		cfg := pipeline.ImportConfig{
			Region:           p.Region,
			PbfPath:          "/data/source.osm.pbf", // container-side mount (regionDir → /data)
			ESHost:           esHost,
			ESPort:           esPort,
			IndexName:        fmt.Sprintf("pelias-%s-%s", p.Region, time.Now().UTC().Format("20060102")),
			Languages:        langs,
			PolylinesEnabled: polylines,
		}
		runner := &pipeline.PeliasRunner{
			Logger:       log,
			Config:       cfg,
			Executables:  peliasExecutables(regionDir, importerImage),
			BuildTimeout: time.Duration(timeoutMin) * time.Minute,
			WorkDir:      workDir,
		}

		paths := pipeline.RegionPaths{
			Root:      regionDir,
			PbfPath:   filepath.Join(regionDir, "source.osm.pbf"),
			RegionKey: p.Region,
		}
		progress := newStageProgress(ctx, deps, p.JobID, log)
		start := time.Now()
		if err := runner.Run(ctx, paths, progress); err != nil {
			return fmt.Errorf("pelias import: %w", err)
		}
		log.Info().Str("index", cfg.IndexName).
			Dur("duration", time.Since(start)).Msg("pipeline:geocoding complete")
		return pipeline.UpdateGeocodingSection(regionDir, p.Region, pipeline.GeocodingSection{
			BuiltAt:              time.Now().UTC(),
			BuildDurationSeconds: time.Since(start).Seconds(),
			Tool:                 "pelias-openstreetmap",
			ToolVersion:          importerImage,
			IndexName:            cfg.IndexName,
			ESHost:               fmt.Sprintf("%s:%d", esHost, esPort),
		})
	}
}

// resolvePeliasES reads search.peliasElasticUrl and splits it into host
// + port. LOCALMAPS_PELIAS_ES_URL env acts as a fallback for dev-compose
// (docs/07-config-schema.md); both paths point at pelias-es:9200 by
// default.
func resolvePeliasES(s StageSettings) (string, int) {
	if raw := settingOrDefault(s, "search.peliasElasticUrl", ""); raw != "" {
		if h, p, ok := parsePeliasURL(raw); ok {
			return h, p
		}
	}
	return parsePeliasESEnv()
}

// parsePeliasURL extracts host + port from a URL string. Returns ok=false
// when parsing fails or the URL lacks an explicit host.
func parsePeliasURL(raw string) (string, int, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", 0, false
	}
	host := u.Hostname()
	port := 9200
	if p := u.Port(); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			port = v
		}
	}
	return host, port, true
}

// parsePeliasESEnv pulls host+port out of LOCALMAPS_PELIAS_ES_URL.
// Falls back to pelias-es:9200 (compose default) if unset/unparsable.
func parsePeliasESEnv() (string, int) {
	raw := os.Getenv("LOCALMAPS_PELIAS_ES_URL")
	if raw == "" {
		return "pelias-es", 9200
	}
	if h, p, ok := parsePeliasURL(raw); ok {
		return h, p
	}
	return "pelias-es", 9200
}

// peliasExecutables returns the production invocation: run the pinned
// openstreetmap importer image against the region directory. The
// `localmaps` docker network is created by docker-compose; when the
// worker runs in a bare container we still need to reach pelias-es +
// pelias-api by name, so we attach to the same network.
func peliasExecutables(regionDir, image string) map[string][]string {
	return map[string][]string{
		"importer": {
			"docker", "run", "--rm",
			"--network", "localmaps",
			"-v", regionDir + ":/data",
			"-e", "PELIAS_CONFIG=/data/pelias.json",
			image, // TODO: pin by SHA-256 digest before prod (docs/08-security.md)
			"./bin/start",
		},
	}
}

// settingBoolOrDefault mirrors settingOrDefault / settingIntOrDefault
// for boolean settings keys.
func settingBoolOrDefault(s StageSettings, key string, def bool) bool {
	if s == nil {
		return def
	}
	if v, err := s.GetBool(key); err == nil {
		return v
	}
	return def
}
