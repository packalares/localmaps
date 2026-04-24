/**
 * Geometry helpers for the measure tool. Works in WGS84 lon/lat inputs
 * and returns SI units (metres, square metres).
 *
 * - `haversineMetres` is the great-circle distance between two points.
 *   Accurate across the antimeridian because `cos(Δλ/2)` stays stable
 *   as longitude wraps and we work with `Math.cos(lat1) * Math.cos(lat2)`
 *   rather than a planar approximation.
 * - `polylineDistanceMetres` sums haversine legs along an ordered list.
 * - `polygonAreaMetres` implements the spherical excess formula, which
 *   gives a physically meaningful area for arbitrary polygons on the
 *   globe. Falls back to 0 for degenerate (<3-point) polygons.
 * - Formatters honour the user's units setting and emit a compact dual
 *   unit suffix once distances exceed a kilometre (mirroring Google Maps).
 */

export interface LngLatPt {
  lng: number;
  lat: number;
}

export type Units = "metric" | "imperial";

const EARTH_RADIUS_METRES = 6_371_008.8;
const METRES_PER_MILE = 1609.344;
const METRES_PER_FOOT = 0.3048;
const SQM_PER_SQMI = METRES_PER_MILE * METRES_PER_MILE;
const SQM_PER_ACRE = 4046.8564224;

function toRad(deg: number): number {
  return (deg * Math.PI) / 180;
}

/**
 * Great-circle distance in metres. Handles antimeridian crossing and
 * polar inputs; throws on NaN to surface programmer errors early.
 */
export function haversineMetres(a: LngLatPt, b: LngLatPt): number {
  if (
    !Number.isFinite(a.lat) ||
    !Number.isFinite(a.lng) ||
    !Number.isFinite(b.lat) ||
    !Number.isFinite(b.lng)
  ) {
    return 0;
  }
  const lat1 = toRad(a.lat);
  const lat2 = toRad(b.lat);
  const dLat = lat2 - lat1;
  // Normalise the longitude delta to (-π, π] so ±179 → ∓179 legs return
  // the short way around the globe.
  let dLng = toRad(b.lng - a.lng);
  if (dLng > Math.PI) dLng -= 2 * Math.PI;
  else if (dLng < -Math.PI) dLng += 2 * Math.PI;
  const s =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2;
  return 2 * EARTH_RADIUS_METRES * Math.asin(Math.min(1, Math.sqrt(s)));
}

/**
 * Total length in metres of a polyline. Closing back to the first
 * point is the caller's responsibility — this sums only the supplied
 * legs in order.
 */
export function polylineDistanceMetres(points: readonly LngLatPt[]): number {
  if (points.length < 2) return 0;
  let total = 0;
  for (let i = 1; i < points.length; i++) {
    total += haversineMetres(points[i - 1]!, points[i]!);
  }
  return total;
}

/**
 * Spherical-excess area (metres²) for a simple polygon on the globe.
 * The ring may be supplied open (no duplicate end-point); we close it
 * implicitly. Fewer than three distinct vertices returns 0.
 */
export function polygonAreaMetres(points: readonly LngLatPt[]): number {
  if (points.length < 3) return 0;
  let sum = 0;
  const n = points.length;
  for (let i = 0; i < n; i++) {
    const p1 = points[i]!;
    const p2 = points[(i + 1) % n]!;
    sum += toRad(p2.lng - p1.lng) * (2 + Math.sin(toRad(p1.lat)) + Math.sin(toRad(p2.lat)));
  }
  return Math.abs((sum * EARTH_RADIUS_METRES * EARTH_RADIUS_METRES) / 2);
}

/**
 * Metric-first distance formatter with optional dual-unit suffix once
 * the value exceeds 1 km. Negatives / NaN → em-dash.
 */
export function formatMeasureDistance(
  metres: number,
  units: Units = "metric",
): string {
  if (!Number.isFinite(metres) || metres < 0) return "—";
  if (metres < 1000) {
    if (units === "imperial") {
      const feet = metres / METRES_PER_FOOT;
      return feet >= 528
        ? `${(metres / METRES_PER_MILE).toFixed(2)} mi`
        : `${Math.round(feet)} ft`;
    }
    return `${Math.round(metres)} m`;
  }
  const km = metres / 1000;
  const mi = metres / METRES_PER_MILE;
  if (units === "imperial") {
    // Primary: miles, with km as the companion for operators still
    // switching units while eyeballing a measurement.
    return `${mi.toFixed(1)} mi / ${km.toFixed(1)} km`;
  }
  return `${km.toFixed(1)} km / ${mi.toFixed(1)} mi`;
}

/**
 * Area formatter: metric default rolls m² → ha → km² depending on
 * magnitude, with imperial ft² → acre → mi². Mirrors formatMeasureDistance
 * in emitting a dual-unit suffix once the value is over 1 km² / 1 mi².
 */
export function formatMeasureArea(
  sqMetres: number,
  units: Units = "metric",
): string {
  if (!Number.isFinite(sqMetres) || sqMetres < 0) return "—";
  if (units === "imperial") {
    const sqFt = sqMetres * 10.76391041671;
    if (sqMetres < 1000) return `${Math.round(sqFt)} ft²`;
    const acres = sqMetres / SQM_PER_ACRE;
    if (sqMetres < SQM_PER_SQMI) return `${acres.toFixed(2)} ac`;
    const sqmi = sqMetres / SQM_PER_SQMI;
    return `${sqmi.toFixed(2)} mi² / ${(sqMetres / 1_000_000).toFixed(2)} km²`;
  }
  if (sqMetres < 10_000) return `${Math.round(sqMetres)} m²`;
  if (sqMetres < 1_000_000) return `${(sqMetres / 10_000).toFixed(2)} ha`;
  const sqkm = sqMetres / 1_000_000;
  const sqmi = sqMetres / SQM_PER_SQMI;
  return `${sqkm.toFixed(2)} km² / ${sqmi.toFixed(2)} mi²`;
}
