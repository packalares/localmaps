import {
  Building2,
  Coffee,
  Fuel,
  Hotel,
  MapPin,
  Pizza,
  ShoppingBag,
  Store,
  TreePine,
  Utensils,
  GraduationCap,
  Hospital,
  Landmark,
  Bus,
  Car,
  type LucideIcon,
} from "lucide-react";
import type { Poi } from "@/lib/api/schemas";

/**
 * POI presentation helpers — kept separate from the React components so
 * the pure string/icon logic can be unit-tested without rendering. Icon
 * choice is driven by the `category` field (Overture places taxonomy)
 * with a coarse mapping; `tags` is consulted as a secondary signal
 * (OSM's amenity/shop tags).
 *
 * See contracts/openapi.yaml → components.schemas.Poi.
 */

interface IconRule {
  icon: LucideIcon;
  keywords: string[];
}

// Ordered: the first matching rule wins.
const ICON_RULES: IconRule[] = [
  { icon: Coffee, keywords: ["cafe", "coffee"] },
  { icon: Pizza, keywords: ["pizza", "fast_food", "fast-food"] },
  { icon: Utensils, keywords: ["restaurant", "food", "eat"] },
  { icon: Hotel, keywords: ["hotel", "hostel", "motel", "lodging"] },
  { icon: Fuel, keywords: ["fuel", "gas", "petrol"] },
  { icon: Car, keywords: ["parking", "car_rental", "car-rental"] },
  { icon: Bus, keywords: ["bus", "transit", "station", "stop_position"] },
  { icon: Hospital, keywords: ["hospital", "clinic", "doctor", "pharmacy"] },
  { icon: GraduationCap, keywords: ["school", "university", "college"] },
  { icon: Landmark, keywords: ["museum", "monument", "landmark", "attraction"] },
  { icon: TreePine, keywords: ["park", "forest", "playground"] },
  { icon: ShoppingBag, keywords: ["mall", "shopping"] },
  { icon: Store, keywords: ["shop", "supermarket", "retail", "convenience"] },
  { icon: Building2, keywords: ["office", "commercial", "business"] },
];

export function iconForPoi(poi: Pick<Poi, "category" | "tags">): LucideIcon {
  const needles: string[] = [];
  if (poi.category) needles.push(poi.category.toLowerCase());
  const tags = poi.tags ?? {};
  for (const k of ["amenity", "shop", "tourism", "leisure", "office"]) {
    const v = tags[k];
    if (v) needles.push(v.toLowerCase());
  }
  if (!needles.length) return MapPin;
  for (const rule of ICON_RULES) {
    for (const n of needles) {
      for (const kw of rule.keywords) {
        if (n.includes(kw)) return rule.icon;
      }
    }
  }
  return MapPin;
}

/** Primary line: the name. Falls back to a category word if missing. */
export function primaryText(poi: Pick<Poi, "label" | "category">): string {
  const label = (poi.label ?? "").trim();
  if (label) return label;
  if (poi.category) return humaniseCategory(poi.category);
  return "Unnamed place";
}

/**
 * Secondary line: a short, humanised category / subtype. We prefer
 * `category`, then `tags.amenity`, then `tags.shop`, then `tags.tourism`.
 */
export function secondaryText(poi: Pick<Poi, "category" | "tags">): string {
  if (poi.category) return humaniseCategory(poi.category);
  const tags = poi.tags ?? {};
  const pick =
    tags["amenity"] ??
    tags["shop"] ??
    tags["tourism"] ??
    tags["leisure"] ??
    tags["office"];
  return pick ? humaniseCategory(pick) : "";
}

export function humaniseCategory(s: string): string {
  if (!s) return "";
  return s
    .replace(/[._:-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

/**
 * Address derivation. OSM / Overture address parts commonly ship as
 * `addr:housenumber`, `addr:street`, `addr:postcode`, `addr:city`.
 * Return an ordered list of lines suitable for rendering one-per-line.
 */
export function addressLines(tags: Record<string, string> | undefined): string[] {
  const t = tags ?? {};
  const pick = (k: string) => (t[k] ?? t[`addr:${k}`] ?? "").trim();
  const street = pick("street");
  const hn = pick("housenumber");
  const line1 = [hn, street].filter(Boolean).join(" ").trim();
  const postcode = pick("postcode");
  const city = pick("city");
  const line2 = [postcode, city].filter(Boolean).join(" ").trim();
  const country = pick("country");
  const out: string[] = [];
  if (line1) out.push(line1);
  if (line2) out.push(line2);
  if (country) out.push(country);
  return out;
}

/** Opening-hours raw string from tags, or undefined. */
export function openingHoursTag(
  tags: Record<string, string> | undefined,
): string | undefined {
  if (!tags) return undefined;
  return tags["opening_hours"] || tags["opening_hours:covid19"] || undefined;
}

/** Phone number from tags (OSM uses `phone` / `contact:phone`). */
export function phoneOf(tags: Record<string, string> | undefined): string | undefined {
  if (!tags) return undefined;
  return tags["phone"] || tags["contact:phone"] || undefined;
}

/** Website URL from tags. */
export function websiteOf(
  tags: Record<string, string> | undefined,
): string | undefined {
  if (!tags) return undefined;
  return tags["website"] || tags["contact:website"] || tags["url"] || undefined;
}
