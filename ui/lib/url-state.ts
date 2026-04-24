/**
 * URL hash <-> map viewport serialiser.
 *
 * Format: `#<zoom>/<lat>/<lon>/<bearing>/<pitch>`
 *
 * Mirrors the deep-link shape used by Google Maps / openstreetmap.org so
 * pasted links Just Work. All numeric fields are kept at fixed precision
 * to keep copy-pasted URLs stable.
 */

export interface MapViewport {
  lat: number;
  lon: number;
  zoom: number;
  bearing: number;
  pitch: number;
}

const LAT_PRECISION = 4;
const LON_PRECISION = 4;
const ZOOM_PRECISION = 2;
const ANGLE_PRECISION = 1;

/** Clamp helper shared by parse + format. */
function clamp(value: number, min: number, max: number): number {
  if (Number.isNaN(value)) return min;
  return Math.min(max, Math.max(min, value));
}

/** Normalise bearing to [0, 360). */
function normaliseBearing(bearing: number): number {
  if (Number.isNaN(bearing)) return 0;
  const mod = bearing % 360;
  return mod < 0 ? mod + 360 : mod;
}

/** Parse a URL hash fragment into a viewport, or null if malformed. */
export function parseHash(hash: string): MapViewport | null {
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  if (raw.length === 0) return null;

  const parts = raw.split("/");
  if (parts.length < 3) return null;

  const zoom = Number.parseFloat(parts[0] ?? "");
  const lat = Number.parseFloat(parts[1] ?? "");
  const lon = Number.parseFloat(parts[2] ?? "");

  if (!Number.isFinite(zoom) || !Number.isFinite(lat) || !Number.isFinite(lon)) {
    return null;
  }

  const bearingRaw =
    parts.length > 3 ? Number.parseFloat(parts[3] ?? "0") : 0;
  const pitchRaw = parts.length > 4 ? Number.parseFloat(parts[4] ?? "0") : 0;

  return {
    zoom: clamp(zoom, 0, 22),
    lat: clamp(lat, -90, 90),
    lon: clamp(lon, -180, 180),
    bearing: normaliseBearing(bearingRaw),
    pitch: clamp(pitchRaw, 0, 85),
  };
}

/** Serialise a viewport to a URL hash fragment (no leading `#`). */
export function formatHash(viewport: MapViewport): string {
  const zoom = clamp(viewport.zoom, 0, 22).toFixed(ZOOM_PRECISION);
  const lat = clamp(viewport.lat, -90, 90).toFixed(LAT_PRECISION);
  const lon = clamp(viewport.lon, -180, 180).toFixed(LON_PRECISION);
  const bearing = normaliseBearing(viewport.bearing).toFixed(ANGLE_PRECISION);
  const pitch = clamp(viewport.pitch, 0, 85).toFixed(ANGLE_PRECISION);

  // Keep default angles terse when possible so shared URLs stay short.
  if (bearing === "0.0" && pitch === "0.0") {
    return `${zoom}/${lat}/${lon}`;
  }
  return `${zoom}/${lat}/${lon}/${bearing}/${pitch}`;
}

/**
 * Parse with a default fallback. Never throws.
 */
export function parseHashOr(
  hash: string,
  fallback: MapViewport,
): MapViewport {
  return parseHash(hash) ?? fallback;
}
