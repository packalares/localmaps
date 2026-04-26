"use client";

import { useEffect } from "react";
import { useMapStore, POI_CATEGORIES } from "@/lib/state/map";
import { usePlaceStore, type SelectedFeature } from "@/lib/state/place";

/**
 * Listens to the canonical `pendingClick` stream from `MapCanvas` and
 * translates each click into either:
 *   1. A cascade-close action (Change 1): if the bottom info card is
 *      already open, the share dialog is open, or a side panel is
 *      open, the click first DISMISSES the topmost surface instead of
 *      dropping a new pin.
 *   2. A `SelectedFeature` on the place store (otherwise): tiled-POI
 *      hit becomes a `poi`, empty space becomes a `point` (the card
 *      then reverse-geocodes the address label on its own).
 *
 * The cascade order mirrors Google Maps desktop:
 *   info card → share dialog → side panel → chip marker → fresh pin.
 *
 * Note on the share dialog: it's a Radix `<Dialog>` whose modal overlay
 * intercepts pointer events before they reach the MapLibre canvas, so
 * `pendingClick` does not even fire while the dialog is open. That step
 * of the cascade is therefore handled implicitly by Radix — we only need
 * to handle the info-card and side-panel steps explicitly.
 *
 * The component renders no DOM — it's pure glue. Mounting it inside
 * the chrome layer in `app/page.tsx` keeps the coupling between click
 * events and the info card out of `MapCanvas`, which this task is
 * explicitly forbidden from modifying. Because `MapCanvas` binds to
 * MapLibre's `map.on("click")`, only clicks on the actual map canvas
 * produce a `pendingClick` — clicks inside the side panel never reach
 * us, satisfying the "outside the panel DOM" requirement for free.
 */
export function SelectedFeatureSync() {
  const pendingClick = useMapStore((s) => s.pendingClick);
  const map = useMapStore((s) => s.map);
  const clearPendingClick = useMapStore((s) => s.clearPendingClick);
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const setSelectedFeature = usePlaceStore((s) => s.setSelectedFeature);

  useEffect(() => {
    if (!pendingClick) return;

    // Read transient state outside the React store subscription so
    // toggling them from inside the same effect doesn't loop.
    const placeState = usePlaceStore.getState();
    const mapState = useMapStore.getState();

    // ------------------------------------------------------------
    // Cascade step 0: search dropdown open → close it (blur input),
    // no new pin. Setting searchDropdownOpen to false in the store
    // is sufficient — the SearchBar effect that pushes isFocused →
    // store also reads it back as authoritative.
    // ------------------------------------------------------------
    if (mapState.searchDropdownOpen) {
      mapState.setSearchDropdownOpen(false);
      // Blur whichever input currently has focus (the search bar).
      if (typeof document !== "undefined" && document.activeElement) {
        (document.activeElement as HTMLElement).blur?.();
      }
      clearPendingClick();
      return;
    }

    // ------------------------------------------------------------
    // Cascade step 1: bottom info card open → close it, no new pin.
    // ------------------------------------------------------------
    if (placeState.selectedFeature !== null) {
      placeState.clearSelectedFeature();
      clearPendingClick();
      return;
    }

    // ------------------------------------------------------------
    // Cascade step 2: share dialog. The Radix modal overlay blocks
    // map clicks entirely, so this branch is unreachable in practice.
    // It is documented above; no code is needed here.
    // ------------------------------------------------------------

    // ------------------------------------------------------------
    // Cascade step 3: any side panel open → close it. The user just
    // clicked the map (panel was outside the click target by virtue
    // of `map.on("click")` only firing for canvas clicks), so the
    // intent is "dismiss the panel".
    // ------------------------------------------------------------
    const tab = mapState.leftRailTab;
    if (tab !== "search") {
      openLeftRail("search");
      clearPendingClick();
      return;
    }

    // ------------------------------------------------------------
    // Cascade step 4 + 5: chip-marker hit (handled inside the
    // marker's own click listener via stopPropagation, so by the
    // time we get here the click is canvas-only) OR a fresh pin
    // drop. The original SelectedFeatureSync behaviour resumes.
    // ------------------------------------------------------------
    const { lng, lat } = pendingClick.lngLat;
    const point = pendingClick.point;

    let feature: SelectedFeature = { kind: "point", lat, lon: lng };

    if (map) {
      try {
        const layers = POI_CATEGORIES
          .map((c) => `poi-${c}`)
          // Narrow to layers that actually exist on the active style.
          .filter((id) => {
            try {
              return !!map.getLayer(id);
            } catch {
              return false;
            }
          });
        if (layers.length > 0) {
          const hits = map.queryRenderedFeatures(
            [point.x, point.y] as [number, number],
            { layers },
          );
          const hit = hits && hits[0];
          if (hit) {
            const props = (hit.properties ?? {}) as Record<string, unknown>;
            const id = (props["id"] as string | undefined)
              ?? (hit.id != null ? String(hit.id) : undefined);
            const name = (props["name"] as string | undefined)
              ?? (props["name:en"] as string | undefined);
            const cls = (props["class"] as string | undefined)
              ?? (props["subclass"] as string | undefined);
            feature = {
              kind: "poi",
              lat,
              lon: lng,
              id,
              name,
              categoryIcon: cls,
            };
          }
        }
      } catch {
        // Style still loading, or the property shape changed — fall
        // through to the plain `point` feature already in hand.
      }
    }

    setSelectedFeature(feature);
    // pendingClick is updated atomically; consume the latest value
    // each time it changes.
  }, [pendingClick, map, setSelectedFeature, clearPendingClick, openLeftRail]);

  return null;
}
