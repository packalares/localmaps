/**
 * Shared chip metadata: each POI category maps to a label, an icon, and
 * a Google-Maps-style fill colour. Both the chip row and the per-result
 * map markers read from this single source so adding a category in one
 * place automatically lights it up everywhere.
 */

import {
  Bus,
  CreditCard,
  Cross,
  Film,
  GraduationCap,
  Hotel,
  MoreHorizontal,
  ShoppingBag,
  UtensilsCrossed,
  type LucideIcon,
} from "lucide-react";
import type { PoiCategory } from "@/lib/state/map";

export interface CategoryDescriptor {
  key: PoiCategory;
  label: string;
  short: string;
  /** Lucide icon component used for the chip + result row. */
  icon: LucideIcon;
  /** Hex colour for the per-result map marker fill. */
  color: string;
  /**
   * Inline SVG path data (24×24 viewBox) for the per-result map marker
   * glyph. Mirrors the visual of the matching Lucide icon so the chip
   * and the marker stay visually consistent. We inline the paths
   * (instead of rendering Lucide React) because the marker DOM is built
   * by `marker-elements.ts` outside React's render tree.
   */
  iconPath: string;
}

/**
 * Inline glyphs for each category. The SVG `<path>` strings come from
 * the Lucide source for the matching icon (`utensils-crossed`,
 * `shopping-bag`, `hotel`, `bus`, `cross`, `credit-card`, `film`,
 * `graduation-cap`, `ellipsis`). They render with `fill: none; stroke:
 * currentColor; stroke-width: 1.6` so the markers stay legible at
 * small sizes.
 */
const FOOD_PATH =
  '<path d="m16 2-2.3 2.3a3 3 0 0 0 0 4.2l1.8 1.8a3 3 0 0 0 4.2 0L22 8"/>' +
  '<path d="M15 15 3.3 3.3a4.2 4.2 0 0 0 0 6l7.3 7.3c.7.7 2 .7 2.8 0L15 15Zm0 0 7 7"/>' +
  '<path d="m2.1 21.8 6.4-6.3"/>' +
  '<path d="m19 5-7 7"/>';
const SHOPPING_PATH =
  '<path d="M16 10a4 4 0 0 1-8 0"/>' +
  '<path d="M3.103 6.034h17.794"/>' +
  '<path d="M3.4 5.467a2 2 0 0 0-.4 1.2V20a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6.667a2 2 0 0 0-.4-1.2l-2-2.667A2 2 0 0 0 17 2H7a2 2 0 0 0-1.6.8z"/>';
const LODGING_PATH =
  '<rect x="4" y="2" width="16" height="20" rx="2"/>' +
  '<path d="M10 22v-6.57"/>' +
  '<path d="M14 15.43V22"/>' +
  '<path d="M15 16a5 5 0 0 0-6 0"/>';
const TRANSIT_PATH =
  '<path d="M8 6v6"/>' +
  '<path d="M15 6v6"/>' +
  '<path d="M2 12h19.6"/>' +
  '<path d="M18 18h3s.5-1.7.8-2.8c.1-.4.2-.8.2-1.2 0-.4-.1-.8-.2-1.2l-1.4-5C20.1 6.8 19.1 6 18 6H4a2 2 0 0 0-2 2v10h3"/>' +
  '<circle cx="7" cy="18" r="2"/>' +
  '<path d="M9 18h5"/>' +
  '<circle cx="16" cy="18" r="2"/>';
const HEALTHCARE_PATH =
  '<path d="M4 9a2 2 0 0 0-2 2v2a2 2 0 0 0 2 2h4a1 1 0 0 1 1 1v4a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2v-4a1 1 0 0 1 1-1h4a2 2 0 0 0 2-2v-2a2 2 0 0 0-2-2h-4a1 1 0 0 1-1-1V4a2 2 0 0 0-2-2h-2a2 2 0 0 0-2 2v4a1 1 0 0 1-1 1z"/>';
const SERVICES_PATH =
  '<rect width="20" height="14" x="2" y="5" rx="2"/>' +
  '<line x1="2" x2="22" y1="10" y2="10"/>';
const ENTERTAINMENT_PATH =
  '<rect width="18" height="18" x="3" y="3" rx="2"/>' +
  '<path d="M7 3v18"/>' +
  '<path d="M3 7.5h4"/>' +
  '<path d="M3 12h18"/>' +
  '<path d="M3 16.5h4"/>' +
  '<path d="M17 3v18"/>' +
  '<path d="M17 7.5h4"/>' +
  '<path d="M17 16.5h4"/>';
const EDUCATION_PATH =
  '<path d="M21.42 10.922a1 1 0 0 0-.019-1.838L12.83 5.18a2 2 0 0 0-1.66 0L2.6 9.08a1 1 0 0 0 0 1.832l8.57 3.908a2 2 0 0 0 1.66 0z"/>' +
  '<path d="M22 10v6"/>' +
  '<path d="M6 12.5V16a6 3 0 0 0 12 0v-3.5"/>';
const OTHER_PATH =
  '<circle cx="12" cy="12" r="1"/>' +
  '<circle cx="19" cy="12" r="1"/>' +
  '<circle cx="5" cy="12" r="1"/>';

export const CATEGORY_DESCRIPTORS: readonly CategoryDescriptor[] = [
  {
    key: "food",
    label: "Food & drink",
    short: "Food",
    icon: UtensilsCrossed,
    color: "#ea580c", // orange-600
    iconPath: FOOD_PATH,
  },
  {
    key: "shopping",
    label: "Shopping",
    short: "Shopping",
    icon: ShoppingBag,
    color: "#2563eb", // blue-600
    iconPath: SHOPPING_PATH,
  },
  {
    key: "lodging",
    label: "Hotels",
    short: "Hotels",
    icon: Hotel,
    color: "#7c3aed", // violet-600
    iconPath: LODGING_PATH,
  },
  {
    key: "transit",
    label: "Transit",
    short: "Transit",
    icon: Bus,
    color: "#0d9488", // teal-600
    iconPath: TRANSIT_PATH,
  },
  {
    key: "healthcare",
    label: "Pharmacies",
    short: "Pharmacies",
    icon: Cross,
    color: "#dc2626", // red-600
    iconPath: HEALTHCARE_PATH,
  },
  {
    key: "services",
    label: "ATMs & banks",
    short: "ATMs & banks",
    icon: CreditCard,
    color: "#0891b2", // cyan-600
    iconPath: SERVICES_PATH,
  },
  {
    key: "entertainment",
    label: "Things to do",
    short: "Things to do",
    icon: Film,
    color: "#db2777", // pink-600
    iconPath: ENTERTAINMENT_PATH,
  },
  {
    key: "education",
    label: "Education",
    short: "Education",
    icon: GraduationCap,
    color: "#65a30d", // lime-600
    iconPath: EDUCATION_PATH,
  },
  {
    key: "other",
    label: "More",
    short: "More",
    icon: MoreHorizontal,
    color: "#475569", // slate-600
    iconPath: OTHER_PATH,
  },
] as const;

/** Look up a descriptor by category key. */
export function descriptorFor(cat: PoiCategory): CategoryDescriptor {
  return (
    CATEGORY_DESCRIPTORS.find((d) => d.key === cat) ??
    CATEGORY_DESCRIPTORS[CATEGORY_DESCRIPTORS.length - 1]
  );
}
