"use client";

import { useState } from "react";
import { List } from "lucide-react";
import { useMapStore } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import { SearchBar } from "@/components/chrome/SearchBar";
import { RegionSwitcher } from "@/components/chrome/RegionSwitcher";
import { SearchPanel } from "@/components/search/SearchPanel";
import { DirectionsPanel } from "@/components/directions/DirectionsPanel";
import { ShareButton } from "@/components/share/ShareButton";
import { BottomNav } from "./BottomNav";
import { BottomSheet, type SheetSnap } from "./BottomSheet";

/**
 * Mobile chrome orchestrator. Same inputs as the desktop LeftRail —
 * only the layout differs:
 *
 *   ┌───────────────────────────────┐
 *   │  SearchBar        · RegionChip│  ← pinned to the top
 *   │                               │
 *   │          <map fills>          │
 *   │                               │
 *   │ ▁ handle ▁                    │  ← BottomSheet
 *   │  Header (tab title + Share)   │
 *   │  [ body of active tab       ] │
 *   │                               │
 *   │ [Search][Dirs][Place][Saved]  │  ← BottomNav
 *   └───────────────────────────────┘
 *
 * The sheet starts at `peek`; opening the search bar or choosing a tab
 * promotes it to `half`. Tapping anywhere in the map (which fires a
 * `pendingClick`) opens the Place tab and expands the sheet.
 */
export function MobileChrome() {
  const leftRailTab = useMapStore((s) => s.leftRailTab);
  const selectedFeature = usePlaceStore((s) => s.selectedFeature);

  const [snap, setSnap] = useState<SheetSnap>("peek");

  // The Place tab in BottomNav surfaces only when a feature is
  // currently selected. The bottom-sheet content for `place` lives in
  // the bottom-center PointInfoCard now (driven by SelectedFeatureSync
  // + PointInfoCardHost), so the mobile sheet only renders the always-
  // available `search` / `directions` / `saved` panels.
  const hasPlace = !!selectedFeature;

  return (
    <>
      {/* Search bar pinned to the very top, Google-Maps style. */}
      <div className="pointer-events-auto fixed inset-x-0 top-0 z-30 flex items-start gap-2 p-3">
        <div className="flex-1">
          <SearchBar embedPanel={false} />
        </div>
        <RegionSwitcher className="h-9" />
      </div>

      <BottomSheet
        snap={snap}
        onSnapChange={setSnap}
        label={`${leftRailTab} panel`}
        header={
          <>
            <span className="flex-1 text-sm font-medium capitalize">
              {leftRailTab}
            </span>
            <ShareButton />
          </>
        }
        className="pb-[env(safe-area-inset-bottom)]"
      >
        <div className="p-2">
          {leftRailTab === "search" && <SearchPanel query="" />}
          {leftRailTab === "directions" && <DirectionsPanel />}
          {leftRailTab === "saved" && (
            <div className="p-4 text-sm text-muted-foreground">
              <List className="mb-2 h-4 w-4 opacity-60" aria-hidden="true" />
              <p>Saved places will appear here.</p>
              <p className="mt-1 text-xs">Sign in to sync across devices.</p>
            </div>
          )}
        </div>
      </BottomSheet>

      <BottomNav snap={snap} onSnapChange={setSnap} hasPlace={hasPlace} />
    </>
  );
}
