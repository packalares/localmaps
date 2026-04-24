/**
 * Region state → UI presentation (label, chip colour class, progress
 * stage copy). The enum values mirror openapi `RegionState` exactly.
 *
 * Colour classes are Tailwind utility stacks using the shared theme
 * tokens so dark mode works automatically.
 */

import type { RegionStateValue } from "@/lib/api/schemas";

export interface RegionStateDisplay {
  label: string;
  /** Short pipeline stage label for progress UI. */
  stage: string;
  /** Tailwind class string for a chip. */
  chipClass: string;
  /** True if the region is currently in a long-running pipeline. */
  inProgress: boolean;
  /** True if the region is in a terminal error state. */
  isFailed: boolean;
  /** True if the region is installed and usable. */
  isReady: boolean;
}

const CHIP_AMBER =
  "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200";
const CHIP_GREEN =
  "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-200";
const CHIP_RED =
  "bg-destructive/15 text-destructive dark:bg-destructive/20";
const CHIP_SLATE =
  "bg-muted text-muted-foreground";

const TABLE: Record<RegionStateValue, RegionStateDisplay> = {
  not_installed: {
    label: "Not installed",
    stage: "Not installed",
    chipClass: CHIP_SLATE,
    inProgress: false,
    isFailed: false,
    isReady: false,
  },
  downloading: {
    label: "Downloading",
    stage: "Downloading pbf",
    chipClass: CHIP_AMBER,
    inProgress: true,
    isFailed: false,
    isReady: false,
  },
  processing_tiles: {
    label: "Building tiles",
    stage: "Building tiles",
    chipClass: CHIP_AMBER,
    inProgress: true,
    isFailed: false,
    isReady: false,
  },
  processing_routing: {
    label: "Building routing",
    stage: "Building routing",
    chipClass: CHIP_AMBER,
    inProgress: true,
    isFailed: false,
    isReady: false,
  },
  processing_geocoding: {
    label: "Indexing geocoder",
    stage: "Indexing geocoder",
    chipClass: CHIP_AMBER,
    inProgress: true,
    isFailed: false,
    isReady: false,
  },
  processing_poi: {
    label: "Fetching POIs",
    stage: "Fetching POIs",
    chipClass: CHIP_AMBER,
    inProgress: true,
    isFailed: false,
    isReady: false,
  },
  updating: {
    label: "Updating",
    stage: "Updating",
    chipClass: CHIP_AMBER,
    inProgress: true,
    isFailed: false,
    isReady: true,
  },
  ready: {
    label: "Ready",
    stage: "Ready",
    chipClass: CHIP_GREEN,
    inProgress: false,
    isFailed: false,
    isReady: true,
  },
  failed: {
    label: "Failed",
    stage: "Failed",
    chipClass: CHIP_RED,
    inProgress: false,
    isFailed: true,
    isReady: false,
  },
  archived: {
    label: "Archived",
    stage: "Archived",
    chipClass: CHIP_SLATE,
    inProgress: false,
    isFailed: false,
    isReady: false,
  },
};

export function formatRegionState(state: RegionStateValue): RegionStateDisplay {
  return TABLE[state] ?? TABLE.not_installed;
}
