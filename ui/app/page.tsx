"use client";

import { MainMap } from "@/components/map/MainMap";
import { FabStack } from "@/components/chrome/FabStack";
import { LayersCard } from "@/components/chrome/LayersCard";
import { LeftIconRail } from "@/components/chrome/LeftIconRail";
import { LeftRail } from "@/components/chrome/LeftRail";
import { SearchBar } from "@/components/chrome/SearchBar";
import { ContextMenuHost } from "@/components/chrome/ContextMenuHost";
import { PoiSearchChips } from "@/components/chrome/PoiSearchChips";
import { useMapStore } from "@/lib/state/map";
import { useRestoreOnMount } from "@/lib/url-state/restore";
import { MeasureTool } from "@/components/tools/MeasureTool";
import { MeasureOverlay } from "@/components/tools/MeasureOverlay";
import { IsochroneTool } from "@/components/tools/IsochroneTool";
import { IsochronePanel } from "@/components/tools/IsochronePanel";
import {
  PointInfoCardHost,
  SelectedFeatureSync,
} from "@/components/place";

/**
 * Main map page. Layout mirrors Google Maps desktop:
 *
 * - The MapLibre canvas fills the viewport.
 * - A permanent 56px-wide left rail (Saved, Recents, recent avatars,
 *   Region, Language) is glued to the left edge.
 * - The floating SearchBar is pinned to `top-4 left-[72px]` so it
 *   clears the rail.
 * - The detached 400px side panel slides in from `left-14` when a
 *   place / directions / saved view is active.
 * - The right rail floats FABs (zoom / compass / locate).
 * - A discreet attribution strip sits bottom-center.
 */
export default function HomePage() {
  // Hydrate Zustand stores from the current URL (viewport, region,
  // selected POI, route, tab) before MapCanvas reads its initial
  // viewport. The hook itself uses `useLayoutEffect` for that ordering
  // guarantee. Without this the share-link round-trip silently drops.
  useRestoreOnMount();
  return (
    <main className="relative h-dvh w-screen overflow-hidden">
      <MainMap />

      {/* Permanent far-left icon rail (56px, viewport height). Desktop
          only — mobile chrome provides its own bottom nav. */}
      <div className="hidden md:block">
        <LeftIconRail />
      </div>

      {/* Left detail panel — directions / saved / place detail.
          Hidden while the active tab is `search`. Renders to the
          right of the permanent rail. */}
      <LeftRail />

      {/* Floating search pill. 320px wide on desktop, sits just to the
          right of the 56px rail (16px gap). */}
      <div className="pointer-events-none absolute left-[72px] top-4 z-30 hidden w-[320px] md:block">
        <SearchBar />
      </div>

      {/* Right rail — floating action buttons anchored to the
          bottom-right corner. */}
      {/* FabStack sits above the MapLibre footer (scale + attribution
          live at the very bottom-right edge). bottom-10 leaves ~24px
          for the footer strip so they never overlap. */}
      <div className="pointer-events-none absolute bottom-10 right-4">
        <FabStack />
      </div>

      {/* Bottom-left Layers card — sits to the right of the rail; when
          a left panel is open it shifts further right so it doesn't
          hide behind the panel, matching Google Maps' layout in 3.png.
          The explicit z-30 keeps it above the panel (z-20). */}
      <LayersCardSlot />

      {/* Helper: see definition below for tab-driven left offset. */}

      {/* POI category chip row — search triggers along the top edge.
          Starts just right of the search bar. */}
      <div className="pointer-events-none absolute left-[404px] right-4 top-4 z-20 hidden h-12 items-center md:flex">
        <PoiSearchChips />
      </div>

      {/* Bottom attribution + scale live inside the MapLibre canvas now —
          MapCanvas mounts AttributionControl + ScaleControl in the
          bottom-right corner so they read the live source list and the
          scale ticks with zoom. See `app/globals.css` for the
          flex-row-reverse override that keeps them on the same row. */}

      {/* Bottom-center info card — point / POI details for the most
          recent map click. The sync component turns `pendingClick`
          into a `selectedFeature` without touching `MapCanvas`. */}
      <SelectedFeatureSync />
      <div className="pointer-events-none absolute inset-x-0 bottom-10 z-20 flex justify-center px-4">
        <PointInfoCardHost />
      </div>

      {/* Right-click context menu (renders on demand). */}
      <ContextMenuHost />

      {/* Phase-7 tools: measure + isochrone. The drivers are invisible;
          their UI companions render conditionally on active tool. */}
      <MeasureTool />
      <IsochroneTool />
      <MeasureOverlay />
      <IsochronePanel />
    </main>
  );
}

// LayersCardSlot — the bottom-left Layers tile shifts right when a
// side panel is open so it doesn't hide behind it. Side panels are
// 400px wide and start at left:56px (after the icon rail), so when
// a non-search tab is active the card sits at left:472px. Otherwise
// it stays at the default 72px (just clearing the icon rail).
function LayersCardSlot() {
  const tab = useMapStore((s) => s.leftRailTab);
  const panelOpen = tab !== "search";
  return (
    <div
      className="pointer-events-none absolute bottom-4 z-30 transition-[left] duration-200"
      style={{ left: panelOpen ? 472 : 72 }}
    >
      <LayersCard />
    </div>
  );
}
