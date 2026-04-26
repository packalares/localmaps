"use client";

import { create } from "zustand";

/**
 * Client-only "selected feature" slice owned by the bottom-center
 * info card(s). Populated when the user clicks a POI marker or an
 * empty spot on the map; cleared when the card is dismissed or
 * another click lands somewhere else.
 *
 * The map-click handler in MapCanvas already publishes a canonical
 * `pendingClick` to `useMapStore` — the info card listens to that and
 * derives the corresponding `selectedFeature` (POI when the click
 * hits a tiled poi layer, plain point otherwise).
 *
 * State on this slice is deliberately tiny: the async details for a
 * POI (hours, phone, website, amenities) stay in TanStack Query via
 * `usePoi(id)`, and the address label for a plain point comes from
 * `useReverseGeocode(...)`. Only fields that flow between sibling
 * components (ContextMenu, Directions panel, future Save/Share
 * actions) live here.
 */

/** Discriminated union: either a raw map coordinate or a tiled POI. */
export type SelectedFeatureKind = "point" | "poi";

export interface SelectedFeature {
  kind: SelectedFeatureKind;
  /** Latitude of the click, in degrees. */
  lat: number;
  /** Longitude of the click, in degrees. */
  lon: number;
  /** POI id when `kind === "poi"`; absent for plain points. */
  id?: string;
  /** Pre-resolved display name (e.g. feature property or POI label). */
  name?: string;
  /** Pre-resolved address line; otherwise filled by reverse-geocode. */
  address?: string;
  /** Opening-hours raw string from tags (e.g. "Mo-Fr 09:00-18:00"). */
  hours?: string;
  /** Phone number from tags. */
  phone?: string;
  /** Website URL from tags. */
  website?: string;
  /** Maki/lucide icon hint derived from the POI's class/subclass. */
  categoryIcon?: string;
}

export interface PlaceState {
  selectedFeature: SelectedFeature | null;
  /** Replace (or clear) the active feature atomically. */
  setSelectedFeature: (feature: SelectedFeature | null) => void;
  /** Shortcut for the card's close button; equivalent to null-setter. */
  clearSelectedFeature: () => void;
}

export const usePlaceStore = create<PlaceState>((set) => ({
  selectedFeature: null,
  setSelectedFeature: (selectedFeature) => set({ selectedFeature }),
  clearSelectedFeature: () => set({ selectedFeature: null }),
}));
