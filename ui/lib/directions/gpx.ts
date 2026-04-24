/**
 * Convert a /api/route response into a standards-compliant GPX 1.1
 * document. Uses the decoded polyline as the <trkseg> point list and
 * emits a <wpt> per named waypoint (if supplied).
 *
 * Spec: https://www.topografix.com/GPX/1/1/
 */

import { decodePolyline } from "./polyline";

export interface GpxWaypoint {
  lat: number;
  lon: number;
  name?: string;
}

export interface GpxInput {
  /** Opaque route id from the API. Used for <trk><name>. */
  routeId: string;
  /** Primary leg polyline(s), precision 6. */
  polylines: string[];
  /** Named waypoints (A, B, stops). Optional. */
  waypoints?: GpxWaypoint[];
  /** Display name for the track. */
  trackName?: string;
  /** App version to embed in the XML header. */
  appVersion?: string;
  /** Override the ISO timestamp (tests). */
  nowIso?: string;
}

const AUTHOR = "LocalMaps";
const DEFAULT_VERSION = "dev";

function escapeXml(v: string): string {
  return v
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

function fmt(n: number): string {
  // 6 decimal places covers ~11 cm precision — more than enough for road
  // routes and consistent with the polyline precision.
  return n.toFixed(6);
}

export function routeToGpx(input: GpxInput): string {
  const version = input.appVersion ?? DEFAULT_VERSION;
  const timestamp = input.nowIso ?? new Date().toISOString();
  const trackName = input.trackName ?? `Route ${input.routeId}`;

  const lines: string[] = [];
  lines.push('<?xml version="1.0" encoding="UTF-8"?>');
  lines.push(
    `<gpx version="1.1" creator="${AUTHOR} ${escapeXml(version)}" ` +
      'xmlns="http://www.topografix.com/GPX/1/1" ' +
      'xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" ' +
      'xsi:schemaLocation="http://www.topografix.com/GPX/1/1 ' +
      'http://www.topografix.com/GPX/1/1/gpx.xsd">',
  );
  lines.push("  <metadata>");
  lines.push(`    <name>${escapeXml(trackName)}</name>`);
  lines.push(`    <author><name>${AUTHOR}</name></author>`);
  lines.push(`    <time>${escapeXml(timestamp)}</time>`);
  lines.push("  </metadata>");

  for (const wp of input.waypoints ?? []) {
    lines.push(`  <wpt lat="${fmt(wp.lat)}" lon="${fmt(wp.lon)}">`);
    if (wp.name) lines.push(`    <name>${escapeXml(wp.name)}</name>`);
    lines.push("  </wpt>");
  }

  lines.push("  <trk>");
  lines.push(`    <name>${escapeXml(trackName)}</name>`);
  for (const poly of input.polylines) {
    const pts = decodePolyline(poly, 6);
    if (pts.length === 0) continue;
    lines.push("    <trkseg>");
    for (const p of pts) {
      lines.push(`      <trkpt lat="${fmt(p.lat)}" lon="${fmt(p.lng)}"/>`);
    }
    lines.push("    </trkseg>");
  }
  lines.push("  </trk>");
  lines.push("</gpx>");
  return lines.join("\n") + "\n";
}
