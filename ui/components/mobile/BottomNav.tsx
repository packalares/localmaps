"use client";

import type { ReactNode } from "react";
import { Bookmark, MapPin, Route, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import { usePlaceStore } from "@/lib/state/place";
import { useMapStore, type LeftRailTab } from "@/lib/state/map";
import type { SheetSnap } from "./BottomSheet";

/** Mobile-only "Place" pseudo-tab. Picked when a `selectedFeature`
 *  exists — surfaces the bottom-center PointInfoCard rather than
 *  flipping the LeftRailTab union. */
type MobileNavTab = LeftRailTab | "place";

/**
 * 4-tab bottom nav modelled on Google Maps Mobile: Search / Directions /
 * Place / Saved. Tapping a tab updates the store (so LeftRail / chrome
 * stay in sync) and promotes the sheet from `peek` to `half` so the
 * content is actually visible. If the user tapped the *currently active*
 * tab and the sheet is already `half`/`full`, we collapse back to `peek`
 * — the same toggle gesture the Google app uses.
 *
 * Place tab is hidden until a POI is selected or a reverse-geocode hit
 * has been surfaced, mirroring the desktop LeftRail.
 */
export interface BottomNavProps {
  snap: SheetSnap;
  onSnapChange: (next: SheetSnap) => void;
  /** Whether the `place` tab should be shown (driven by MobileChrome). */
  hasPlace: boolean;
  className?: string;
}

interface TabDef {
  id: MobileNavTab;
  label: string;
  icon: ReactNode;
}

const TABS: TabDef[] = [
  { id: "search", label: "Search", icon: <Search className="h-5 w-5" aria-hidden="true" /> },
  { id: "directions", label: "Directions", icon: <Route className="h-5 w-5" aria-hidden="true" /> },
  { id: "place", label: "Place", icon: <MapPin className="h-5 w-5" aria-hidden="true" /> },
  { id: "saved", label: "Saved", icon: <Bookmark className="h-5 w-5" aria-hidden="true" /> },
];

export function BottomNav({
  snap,
  onSnapChange,
  hasPlace,
  className,
}: BottomNavProps) {
  const leftRailTab = useMapStore((s) => s.leftRailTab);
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const selectedFeature = usePlaceStore((s) => s.selectedFeature);

  // The active pseudo-tab is `place` whenever a feature is selected
  // (so the user can spot what they last clicked); otherwise we
  // mirror the canonical `leftRailTab` from the store.
  const activeTab: MobileNavTab =
    hasPlace && selectedFeature ? "place" : leftRailTab;

  const onSelect = (id: MobileNavTab) => {
    const sameTab = id === activeTab;
    if (id !== "place") {
      openLeftRail(id);
    }
    if (sameTab && snap !== "peek") {
      onSnapChange("peek");
      return;
    }
    if (snap === "peek") onSnapChange("half");
  };

  const tabs = TABS.filter((t) => (t.id === "place" ? hasPlace : true));

  return (
    <nav
      aria-label="Map navigation"
      className={cn(
        "pointer-events-auto fixed inset-x-0 bottom-0 z-30 flex items-stretch border-t border-border bg-background/95 backdrop-blur",
        "pb-[env(safe-area-inset-bottom)]",
        className,
      )}
    >
      {tabs.map((t) => {
        const active = activeTab === t.id;
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onSelect(t.id)}
            aria-pressed={active}
            aria-label={t.label}
            className={cn(
              "flex flex-1 flex-col items-center justify-center gap-0.5 py-2 text-xs transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              active
                ? "text-primary"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {t.icon}
            <span>{t.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
