/**
 * Convert a /api/route response into a standards-compliant KML 2.2
 * document. Emits a <Placemark> for each leg's <LineString>, plus one
 * <Placemark> per named waypoint.
 *
 * Spec: https://developers.google.com/kml/documentation/kmlreference
 */

import { decodePolyline } from "./polyline";

export interface KmlWaypoint {
  lat: number;
  lon: number;
  name?: string;
}

export interface KmlInput {
  routeId: string;
  polylines: string[];
  waypoints?: KmlWaypoint[];
  documentName?: string;
  appVersion?: string;
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
  return n.toFixed(6);
}

export function routeToKml(input: KmlInput): string {
  const version = input.appVersion ?? DEFAULT_VERSION;
  const docName = input.documentName ?? `Route ${input.routeId}`;

  const lines: string[] = [];
  lines.push('<?xml version="1.0" encoding="UTF-8"?>');
  lines.push('<kml xmlns="http://www.opengis.net/kml/2.2">');
  lines.push("  <Document>");
  lines.push(`    <name>${escapeXml(docName)}</name>`);
  lines.push(
    `    <atom:author xmlns:atom="http://www.w3.org/2005/Atom">` +
      `<atom:name>${AUTHOR} ${escapeXml(version)}</atom:name></atom:author>`,
  );
  lines.push('    <Style id="route">');
  lines.push("      <LineStyle>");
  lines.push("        <color>ff2b85ec</color>");
  lines.push("        <width>6</width>");
  lines.push("      </LineStyle>");
  lines.push("    </Style>");

  for (let i = 0; i < (input.waypoints ?? []).length; i++) {
    const wp = input.waypoints![i];
    lines.push("    <Placemark>");
    if (wp.name) lines.push(`      <name>${escapeXml(wp.name)}</name>`);
    lines.push("      <Point>");
    lines.push(
      `        <coordinates>${fmt(wp.lon)},${fmt(wp.lat)},0</coordinates>`,
    );
    lines.push("      </Point>");
    lines.push("    </Placemark>");
  }

  for (let legIndex = 0; legIndex < input.polylines.length; legIndex++) {
    const pts = decodePolyline(input.polylines[legIndex], 6);
    if (pts.length === 0) continue;
    lines.push("    <Placemark>");
    lines.push(`      <name>Leg ${legIndex + 1}</name>`);
    lines.push("      <styleUrl>#route</styleUrl>");
    lines.push("      <LineString>");
    lines.push("        <tessellate>1</tessellate>");
    const coords = pts
      .map((p) => `${fmt(p.lng)},${fmt(p.lat)},0`)
      .join(" ");
    lines.push(`        <coordinates>${coords}</coordinates>`);
    lines.push("      </LineString>");
    lines.push("    </Placemark>");
  }

  lines.push("  </Document>");
  lines.push("</kml>");
  return lines.join("\n") + "\n";
}
