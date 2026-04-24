import {
  Building,
  Home,
  Hospital,
  Landmark,
  MapPin,
  Road,
  School,
  Store,
  Utensils,
  type LucideIcon,
} from "lucide-react";
import type { GeocodeResult } from "@/lib/api/schemas";

/**
 * Derive display props for a GeocodeResult.
 *
 * The OpenAPI `GeocodeResult` carries `label`, `address`, `category`,
 * `center`, `bbox`, `confidence` and `region` — no dedicated
 * `placeType` field exists. This module uses `category` (free-form
 * string, e.g. "road", "venue", "address", "locality") to pick a
 * Google-Maps-style icon.
 *
 * `primary` is the short title drawn from `label`'s first comma-
 * separated chunk (the common Pelias convention). `secondary` holds
 * the rest of the label so the row reads like a Google Maps card.
 *
 * `distance` is the haversine distance from a reference point —
 * typically the current map centre — in metres, with a human string
 * e.g. "1.2 km" or "350 m".
 */

export interface FormattedResult {
  /** lucide icon component. Never null. */
  icon: LucideIcon;
  /** Primary display line (bold in Google Maps cards). */
  primary: string;
  /** Secondary muted line; may be empty. */
  secondary: string;
  /** Distance in metres from the reference point; null if unavailable. */
  distanceMeters: number | null;
  /** Pretty-printed distance for display; empty string when unknown. */
  distanceLabel: string;
}

/** Pelias `category` / OSM class buckets we map to specific icons. */
const ICON_TABLE: ReadonlyArray<{ match: RegExp; icon: LucideIcon }> = [
  { match: /address|housenumber|house|building/i, icon: Home },
  { match: /road|street|way|path|highway|motorway/i, icon: Road },
  { match: /restaurant|food|bar|cafe|pub|eatery|venue/i, icon: Utensils },
  { match: /shop|store|mall|retail|market/i, icon: Store },
  { match: /hospital|clinic|pharmacy|health/i, icon: Hospital },
  { match: /school|university|college|campus|library|education/i, icon: School },
  { match: /landmark|monument|attraction|museum|tourism/i, icon: Landmark },
  { match: /venue|office|company|commercial/i, icon: Building },
];

/** Pick an icon for a GeocodeResult. Falls back to MapPin. */
export function iconFor(result: Pick<GeocodeResult, "category">): LucideIcon {
  const cat = result.category;
  if (!cat) return MapPin;
  for (const { match, icon } of ICON_TABLE) {
    if (match.test(cat)) return icon;
  }
  return MapPin;
}

/** Split a comma-separated label into `[primary, secondary]`. */
export function splitLabel(label: string): { primary: string; secondary: string } {
  const trimmed = label.trim();
  const comma = trimmed.indexOf(",");
  if (comma < 0) return { primary: trimmed, secondary: "" };
  return {
    primary: trimmed.slice(0, comma).trim(),
    secondary: trimmed.slice(comma + 1).trim(),
  };
}

/**
 * Haversine great-circle distance in metres between two WGS84 points.
 * Accepts `{lat, lon}` in decimal degrees.
 */
export function haversineMeters(
  a: { lat: number; lon: number },
  b: { lat: number; lon: number },
): number {
  const R = 6371008.8; // mean Earth radius in metres
  const φ1 = (a.lat * Math.PI) / 180;
  const φ2 = (b.lat * Math.PI) / 180;
  const dφ = ((b.lat - a.lat) * Math.PI) / 180;
  const dλ = ((b.lon - a.lon) * Math.PI) / 180;
  const sinDφ = Math.sin(dφ / 2);
  const sinDλ = Math.sin(dλ / 2);
  const h = sinDφ * sinDφ + Math.cos(φ1) * Math.cos(φ2) * sinDλ * sinDλ;
  return 2 * R * Math.asin(Math.min(1, Math.sqrt(h)));
}

/** Format a metre distance as "120 m", "1.2 km", or "980 km". */
export function formatDistance(metres: number): string {
  if (!Number.isFinite(metres) || metres < 0) return "";
  if (metres < 1000) {
    return `${Math.round(metres)} m`;
  }
  const km = metres / 1000;
  if (km < 10) return `${km.toFixed(1)} km`;
  return `${Math.round(km)} km`;
}

/**
 * Build display props for a result row relative to an optional origin.
 * When `origin` is undefined (no map centre yet), distance fields are
 * null/empty but the rest of the row still renders.
 */
export function formatResult(
  result: GeocodeResult,
  origin?: { lat: number; lon: number } | null,
): FormattedResult {
  const icon = iconFor(result);
  const { primary, secondary } = splitLabel(result.label);
  if (!origin) {
    return { icon, primary, secondary, distanceMeters: null, distanceLabel: "" };
  }
  const distanceMeters = haversineMeters(origin, result.center);
  return {
    icon,
    primary,
    secondary,
    distanceMeters,
    distanceLabel: formatDistance(distanceMeters),
  };
}
