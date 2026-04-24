"use client";

import { useEffect, useState } from "react";
import {
  Bookmark,
  ChevronLeft,
  ChevronRight,
  List,
  MapPin,
  Route,
  Search,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { SearchBar } from "./SearchBar";
import { RegionSwitcher } from "./RegionSwitcher";
import { LocaleSelector } from "./LocaleSelector";
import { SearchPanel } from "@/components/search/SearchPanel";
import { DirectionsPanel } from "@/components/directions/DirectionsPanel";
import { PoiPane, useWhatsHere, type WhatsHereHit } from "@/components/poi";
import { ShareButton } from "@/components/share/ShareButton";
import { MobileChrome } from "@/components/mobile/MobileChrome";
import { useMapStore, type LeftRailTab } from "@/lib/state/map";
import { useBreakpoint } from "@/lib/responsive/use-breakpoint";
import { useMessages } from "@/lib/i18n/provider";

/**
 * Responsive entry point. Below 768px renders the mobile chrome
 * (BottomSheet + BottomNav + top-pinned SearchBar); at tablet/desktop
 * renders the classic collapsible side panel. During SSR the breakpoint
 * hook returns `null` — we fall through to the desktop rail so the
 * server-rendered markup matches what most visitors see.
 */
export function LeftRail() {
  const bp = useBreakpoint();
  if (bp === "mobile") {
    return <MobileChrome />;
  }
  return <LeftRailDesktop />;
}

/**
 * Collapsible left rail, mirroring Google Maps' side panel. Contains the
 * search pill + region switcher at the top, a tab strip below for
 * Search / Directions / Place / Saved, and the active tab body.
 *
 * Tab state lives in the Zustand store (`leftRailTab`) so the search
 * bar, context menu, and POI resolver can flip tabs imperatively.
 */
export function LeftRailDesktop() {
  const [collapsed, setCollapsed] = useState(false);
  const leftRailTab = useMapStore((s) => s.leftRailTab);
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const selectedPoi = useMapStore((s) => s.selectedPoi);
  const selectedResult = useMapStore((s) => s.selectedResult);
  const { t } = useMessages();

  // Watch pendingClick → run useWhatsHere → auto-open the place tab.
  const pendingClick = useMapStore((s) => s.pendingClick);
  const clearPendingClick = useMapStore((s) => s.clearPendingClick);
  const [hit, setHit] = useState<WhatsHereHit | null>(null);
  useEffect(() => {
    if (!pendingClick) return;
    setHit({ lngLat: pendingClick.lngLat });
    openLeftRail("place");
    clearPendingClick();
  }, [pendingClick, openLeftRail, clearPendingClick]);
  const whatsHere = useWhatsHere(hit);

  return (
    <aside
      className={cn(
        "pointer-events-auto relative z-10 flex h-full flex-col gap-3 transition-[width] duration-200 ease-in-out",
        collapsed ? "w-12" : "w-[380px]",
      )}
      aria-label={t("leftRail.ariaLabel")}
    >
      {!collapsed && (
        <>
          <div className="flex items-center gap-2 px-3 pt-3">
            <div className="flex-1">
              <SearchBar embedPanel={false} />
            </div>
            <RegionSwitcher />
            <LocaleSelector />
            <ShareButton />
          </div>
          <nav
            className="mx-3 chrome-card flex overflow-hidden text-sm"
            aria-label={t("leftRail.tabs.ariaLabel")}
          >
            <TabButton
              active={leftRailTab === "search"}
              onClick={() => openLeftRail("search")}
              icon={<Search className="h-4 w-4" aria-hidden="true" />}
              label={t("leftRail.tabs.search")}
            />
            <TabButton
              active={leftRailTab === "directions"}
              onClick={() => openLeftRail("directions")}
              icon={<Route className="h-4 w-4" aria-hidden="true" />}
              label={t("leftRail.tabs.directions")}
            />
            <TabButton
              active={leftRailTab === "place"}
              onClick={() => openLeftRail("place")}
              icon={<MapPin className="h-4 w-4" aria-hidden="true" />}
              label={t("leftRail.tabs.place")}
              hidden={!selectedPoi && whatsHere.status === "idle" && !hit}
            />
            <TabButton
              active={leftRailTab === "saved"}
              onClick={() => openLeftRail("saved")}
              icon={<Bookmark className="h-4 w-4" aria-hidden="true" />}
              label={t("leftRail.tabs.saved")}
            />
          </nav>

          <div className="chrome-card mx-3 mb-3 flex-1 overflow-y-auto">
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
                }}
              />
            )}
            {leftRailTab === "saved" && (
              <div className="p-4 text-sm text-muted-foreground">
                <List className="mb-2 h-4 w-4 opacity-60" aria-hidden="true" />
                <p>{t("leftRail.saved.title")}</p>
                <p className="mt-1 text-xs">{t("leftRail.saved.subtitle")}</p>
              </div>
            )}
            {leftRailTab === "results" && (
              <div className="p-4 text-sm text-muted-foreground">
                <p>Browse installed regions and pinned results here.</p>
              </div>
            )}
          </div>
        </>
      )}

      <Button
        variant="chrome"
        size="icon"
        onClick={() => setCollapsed((c) => !c)}
        aria-label={collapsed ? t("leftRail.expand") : t("leftRail.collapse")}
        aria-expanded={!collapsed}
        className="absolute -right-4 top-1/2 h-8 w-8 -translate-y-1/2"
      >
        {collapsed ? (
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        ) : (
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
        )}
      </Button>
    </aside>
  );
}

function TabButton(props: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  hidden?: boolean;
}) {
  if (props.hidden) return null;
  return (
    <button
      type="button"
      onClick={props.onClick}
      aria-pressed={props.active}
      className={cn(
        "flex flex-1 items-center justify-center gap-2 px-3 py-2 text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        props.active
          ? "bg-primary/10 text-primary"
          : "text-foreground hover:bg-muted",
      )}
    >
      {props.icon}
      <span>{props.label}</span>
    </button>
  );
}

// Narrowing for external consumers + ts check.
export type { LeftRailTab };
