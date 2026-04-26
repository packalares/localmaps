// Package peliasindex is the in-process replacement for the
// pelias/openstreetmap:6.4.0 Node.js importer. It reads an OSM PBF
// extract, produces pelias-shaped documents for POI / named-place /
// named-street features, and bulk-indexes them into the `pelias` index
// on pelias-es via the Elasticsearch _bulk HTTP API.
//
// Why in-process? The upstream importer is a Node binary that runs in
// its own container; under Kubernetes we can't mount a docker socket
// in the worker pod (docs/10-deploy.md). Re-writing the small surface
// we care about in Go keeps the pipeline self-contained without a
// sidecar container.
//
// What it indexes:
//   - Nodes with amenity/shop/tourism/leisure/historic/office tags →
//     layer=venue (POI).
//   - Nodes with place=city/town/village/suburb/neighbourhood →
//     layer=locality.
//   - Named ways whose `highway` tag is set → layer=street.
//   - Ways tagged building/amenity are emitted as layer=venue anchored
//     on the first node's lat/lon (when present in the decoder).
//   - Nodes/ways carrying addr:street + addr:housenumber but no POI/
//     place/highway tag → layer=address; lets reverse-geocode return a
//     real "Strada Lipscani 12" instead of the nearest POI.
//
// The produced docs carry the fields pelias-api reads at query time:
// gid, source, source_id, layer, center_point, name.default,
// address_parts, category. See https://github.com/pelias/schema for
// the full canonical schema.
//
// Build is the only exported entry point for write-side ingestion;
// PurgeRegion mirrors it for the cleanup path used by KindRegionDelete.
// ES helpers (ensureIndex, bulkIndex, bulkDelete) live in esbulk.go;
// doc shaping lives in doc.go.
package peliasindex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
	"github.com/rs/zerolog"
)

// Options controls a single Build run. Zero-valued fields fall back to
// internal defaults (see defaulted).
type Options struct {
	// BatchSize is the number of docs per ES _bulk request.
	BatchSize int
	// ESTimeout bounds one _bulk HTTP roundtrip.
	ESTimeout time.Duration
	// HTTP lets tests swap in an httptest client. nil → http.DefaultClient.
	HTTP *http.Client
	// IndexName overrides the default "pelias" index.
	IndexName string
}

func (o Options) defaulted() Options {
	if o.BatchSize <= 0 {
		o.BatchSize = 1000
	}
	if o.ESTimeout <= 0 {
		o.ESTimeout = 30 * time.Second
	}
	if o.HTTP == nil {
		o.HTTP = http.DefaultClient
	}
	if o.IndexName == "" {
		o.IndexName = "pelias"
	}
	return o
}

// Stats is what Build returns on success — useful for the manifest
// update + structured logs.
type Stats struct {
	NodesSeen   int64
	WaysSeen    int64
	DocsIndexed int64
	Duration    time.Duration
}

// Build reads pbfPath, produces pelias docs, and bulk-indexes them
// into esURL under the `pelias` index alias.
//
// esURL is the Elasticsearch base URL (e.g. http://pelias-es:9200);
// trailing slashes are trimmed. region is carried into each document's
// `region` field so documents from different regions co-exist in the
// same index.
func Build(ctx context.Context, pbfPath, esURL, region string, log zerolog.Logger) (Stats, error) {
	return BuildWithOptions(ctx, pbfPath, esURL, region, Options{}, log)
}

// BuildWithOptions is the test-friendly Build. The zero Options produces
// the production configuration.
func BuildWithOptions(ctx context.Context, pbfPath, esURL, region string, opts Options, log zerolog.Logger) (Stats, error) {
	opts = opts.defaulted()
	esURL = strings.TrimRight(strings.TrimSpace(esURL), "/")
	if pbfPath == "" {
		return Stats{}, errors.New("peliasindex: pbf path required")
	}
	if esURL == "" {
		return Stats{}, errors.New("peliasindex: es URL required")
	}
	if region == "" {
		return Stats{}, errors.New("peliasindex: region required")
	}
	start := time.Now()

	if err := ensureIndex(ctx, opts, esURL, log); err != nil {
		return Stats{}, fmt.Errorf("ensure index: %w", err)
	}

	f, err := os.Open(pbfPath) // #nosec G304 — caller-controlled path
	if err != nil {
		return Stats{}, fmt.Errorf("open pbf: %w", err)
	}
	defer f.Close() //nolint:errcheck

	scanner := osmpbf.New(ctx, f, 1)
	defer scanner.Close() //nolint:errcheck
	scanner.SkipRelations = true

	stats := Stats{}
	batch := make([]doc, 0, opts.BatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		n, err := bulkIndex(ctx, opts, esURL, batch)
		if err != nil {
			return err
		}
		stats.DocsIndexed += int64(n)
		batch = batch[:0]
		return nil
	}

	for scanner.Scan() {
		obj := scanner.Object()
		switch v := obj.(type) {
		case *osm.Node:
			stats.NodesSeen++
			if d, ok := nodeDoc(v, region); ok {
				batch = append(batch, d)
			}
		case *osm.Way:
			stats.WaysSeen++
			if d, ok := wayDoc(v, region); ok {
				batch = append(batch, d)
			}
		}
		if len(batch) >= opts.BatchSize {
			if err := flush(); err != nil {
				return stats, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("pbf scan: %w", err)
	}
	if err := flush(); err != nil {
		return stats, err
	}

	stats.Duration = time.Since(start)
	log.Info().
		Int64("nodes", stats.NodesSeen).
		Int64("ways", stats.WaysSeen).
		Int64("docs", stats.DocsIndexed).
		Dur("duration", stats.Duration).
		Msg("peliasindex build complete")
	return stats, nil
}

// PurgeRegion removes every pelias doc tagged with
// `addendum.osm.region == regionKey` from the canonical `pelias`
// index. Returns the number of documents the server deleted.
//
// This is the cleanup primitive used by the KindRegionDelete handler.
// It wraps bulkDelete with sane defaults (30s timeout, default HTTP
// client, "pelias" index name) so callers don't have to hand-build
// Options for the common case.
func PurgeRegion(ctx context.Context, esURL, regionKey string, log zerolog.Logger) (int64, error) {
	esURL = strings.TrimRight(strings.TrimSpace(esURL), "/")
	if esURL == "" {
		return 0, errors.New("peliasindex: es URL required")
	}
	if regionKey == "" {
		return 0, errors.New("peliasindex: region required")
	}
	opts := Options{}.defaulted()
	deleted, err := bulkDelete(ctx, opts, esURL, regionKey, log)
	if err != nil {
		return 0, fmt.Errorf("purge region: %w", err)
	}
	log.Info().
		Str("region", regionKey).
		Int64("deleted", deleted).
		Msg("peliasindex purge complete")
	return deleted, nil
}
