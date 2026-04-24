"use client";

import { useEffect, useState } from "react";
import { List } from "lucide-react";
import { useMapStore } from "@/lib/state/map";
import { SearchBar } from "@/components/chrome/SearchBar";
import { RegionSwitcher } from "@/components/chrome/RegionSwitcher";
import { SearchPanel } from "@/components/search/SearchPanel";
import { DirectionsPanel } from "@/components/directions/DirectionsPanel";
import { PoiPane, useWhatsHere, type WhatsHereHit } from "@/components/poi";
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
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const selectedPoi = useMapStore((s) => s.selectedPoi);
  const selectedResult = useMapStore((s) => s.selectedResult);
  const pendingClick = useMapStore((s) => s.pendingClick);
  const clearPendingClick = useMapStore((s) => s.clearPendingClick);

  const [snap, setSnap] = useState<SheetSnap>("peek");
  const [hit, setHit] = useState<WhatsHereHit | null>(null);

  useEffect(() => {
    if (!pendingClick) return;
    setHit({ lngLat: pendingClick.lngLat });
    openLeftRail("place");
    setSnap((s) => (s === "peek" ? "half" : s));
    clearPendingClick();
  }, [pendingClick, openLeftRail, clearPendingClick]);

  const whatsHere = useWhatsHere(hit);
  const hasPlace = !!selectedPoi || whatsHere.status !== "idle" || !!hit;

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
          {leftRailTab === "search" && (
            <SearchPanel query={selectedResult?.label ?? ""} />
          )}
          {leftRailTab === "directions" && <DirectionsPanel />}
          {leftRailTab === "place" && (
            <PoiPane
              poi={whatsHere.poi}
              status={whatsHere.status}
              onClose={() => {
                setHit(null);
                openLeftRail("search");
                setSnap("peek");
              }}
            />
          )}
          {leftRailTab === "saved" && (
            <div className="p-4 text-sm text-muted-foreground">
              <List className="mb-2 h-4 w-4 opacity-60" aria-hidden="true" />
              <p>Saved places will appear here.</p>
              <p className="mt-1 text-xs">Sign in to sync across devices.</p>
            </div>
          )}
          {leftRailTab === "results" && (
            <div className="p-4 text-sm text-muted-foreground">
              <p>Browse installed regions and pinned results here.</p>
            </div>
          )}
        </div>
      </BottomSheet>

      <BottomNav snap={snap} onSnapChange={setSnap} hasPlace={hasPlace} />
    </>
  );
}
