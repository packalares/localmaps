/**
 * Human-friendly distance formatter. Matches Google Maps conventions:
 *
 * - metric: under 1 km → meters rounded to nearest 10 (or 1 if <100 m);
 *           ≥ 1 km → km with one decimal below 10 km, whole km above.
 * - imperial: under 0.1 mi → feet; else miles, one decimal below 10,
 *             whole above.
 *
 * Input is always meters — the OpenAPI contract uses SI everywhere.
 */

export type DistanceUnits = "metric" | "imperial";

export interface FormatDistanceOptions {
  units?: DistanceUnits;
}

const METERS_PER_MILE = 1609.344;
const METERS_PER_FOOT = 0.3048;

export function formatDistance(
  meters: number,
  options: FormatDistanceOptions = {},
): string {
  if (!Number.isFinite(meters) || meters < 0) return "—";
  const units = options.units ?? "metric";

  if (units === "imperial") {
    const miles = meters / METERS_PER_MILE;
    if (miles < 0.1) {
      const feet = Math.round(meters / METERS_PER_FOOT);
      return `${feet} ft`;
    }
    if (miles < 10) return `${miles.toFixed(1)} mi`;
    return `${Math.round(miles)} mi`;
  }

  if (meters < 100) {
    return `${Math.round(meters)} m`;
  }
  if (meters < 1000) {
    return `${Math.round(meters / 10) * 10} m`;
  }
  const km = meters / 1000;
  if (km < 10) return `${km.toFixed(1)} km`;
  return `${Math.round(km)} km`;
}
