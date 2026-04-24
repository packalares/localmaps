/**
 * Google-style encoded polyline decoder used by Valhalla (precision 6).
 *
 * See https://valhalla.github.io/valhalla/decoding/ — this is the
 * widely-used ASCII encoding of a list of (lat, lon) pairs. The
 * `/api/route` response gives us `geometry.polyline` at precision 6
 * per the OpenAPI contract, so the factor is 1e-6.
 *
 * Pure, no imports; easy to test.
 */

export interface LngLat {
  lng: number;
  lat: number;
}

const DEFAULT_PRECISION = 6;

/** Decode an encoded polyline into an array of `{lng, lat}` pairs. */
export function decodePolyline(
  encoded: string,
  precision: number = DEFAULT_PRECISION,
): LngLat[] {
  const factor = Math.pow(10, precision);
  const points: LngLat[] = [];
  let index = 0;
  let lat = 0;
  let lng = 0;

  while (index < encoded.length) {
    let result = 0;
    let shift = 0;
    let byte: number;
    do {
      byte = encoded.charCodeAt(index++) - 63;
      result |= (byte & 0x1f) << shift;
      shift += 5;
    } while (byte >= 0x20);
    const dLat = (result & 1) !== 0 ? ~(result >> 1) : result >> 1;
    lat += dLat;

    result = 0;
    shift = 0;
    do {
      byte = encoded.charCodeAt(index++) - 63;
      result |= (byte & 0x1f) << shift;
      shift += 5;
    } while (byte >= 0x20);
    const dLng = (result & 1) !== 0 ? ~(result >> 1) : result >> 1;
    lng += dLng;

    points.push({ lng: lng / factor, lat: lat / factor });
  }

  return points;
}

export interface RouteBounds {
  /** [minLon, minLat, maxLon, maxLat] */
  bbox: [number, number, number, number];
}

/** Compute a bounding box from a list of lng/lat points. */
export function boundsFromPoints(points: LngLat[]): RouteBounds | null {
  if (points.length === 0) return null;
  let minLon = points[0].lng;
  let maxLon = points[0].lng;
  let minLat = points[0].lat;
  let maxLat = points[0].lat;
  for (const p of points) {
    if (p.lng < minLon) minLon = p.lng;
    if (p.lng > maxLon) maxLon = p.lng;
    if (p.lat < minLat) minLat = p.lat;
    if (p.lat > maxLat) maxLat = p.lat;
  }
  return { bbox: [minLon, minLat, maxLon, maxLat] };
}
