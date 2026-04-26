"use client";

import { List, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { DirectionsPanel } from "@/components/directions/DirectionsPanel";
import { CategoryResultsPanel } from "@/components/chrome/CategoryResultsPanel";
import { RecentsPanel } from "@/components/chrome/RecentsPanel";
import { MobileChrome } from "@/components/mobile/MobileChrome";
import {
  assertNeverTab,
  useMapStore,
  type LeftRailTab,
} from "@/lib/state/map";
import { useBreakpoint } from "@/lib/responsive/use-breakpoint";
import { useMessages } from "@/lib/i18n/provider";

/**
 * Responsive entry point. Below 768px renders the mobile chrome
 * (BottomSheet + BottomNav + top-pinned SearchBar); at tablet/desktop
 * renders Google's side panel column. During SSR the breakpoint hook
 * returns `null` — we fall through to the desktop rail so the
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
 * Sliding panel column that mimics Google Maps desktop: a detached
 * 400px-wide white column anchored to the left edge of the viewport,
 * top-0 / bottom-0, housing whichever detail panel the user asked
 * for (directions form, selected POI, saved list). Typing in the
 * floating search bar does NOT open this column — autocomplete
 * results appear in the SearchBar's own dropdown. The column is only
 * shown when the user has explicitly navigated to a panel:
 * Directions button, context menu "What's here?", or a saved-list
 * entry.
 *
 * The canonical `leftRailTab` state still drives what renders. The
 * `search` value means the column is hidden (just the floating
 * SearchBar + map remain visible). The X button restores that state.
 */
export function LeftRailDesktop() {
  const leftRailTab = useMapStore((s) => s.leftRailTab);
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const { t } = useMessages();

  // The column is hidden when the only active tab is `search` — the
  // floating SearchBar owns that UX. Everything else promotes the
  // column into view.
  const visible = leftRailTab !== "search";

  if (!visible) return null;

  const handleClose = () => {
    // Full reset: deactivate any active chip, clear the search query,
    // and close the panel. Mirrors Google's "X closes everything".
    useMapStore.getState().closeCategoryResults();
    useMapStore.getState().requestClearSearchQuery();
    openLeftRail("search");
  };

  // Some panels (DirectionsPanel, CategoryResultsPanel) render their
  // own header X. Skip the rail's global X for those tabs to avoid the
  // double-close-button stack the audit flagged.
  const hidesPanelX =
    leftRailTab === "directions" || leftRailTab === "categoryResults";

  return (
    <aside
      className={cn(
        // Sits to the right of the permanent 56px LeftIconRail on desktop;
        // full-width on mobile (MobileChrome handles that branch before
        // we get here). Uses the chrome surface tokens so the elevation
        // cue reads in both light and dark mode (was hardcoded
        // bg-white + shadow-xl with no dark variant).
        "pointer-events-auto absolute inset-y-0 left-14 z-20 flex w-[400px] flex-col bg-chrome-surface text-foreground shadow-chrome-lg",
      )}
      aria-label={t("leftRail.ariaLabel")}
    >
      {/* Close affordance (top-right of the panel). Skipped for tabs
          that already render their own header X (Directions, category
          results) — keeps the chrome to one X button per panel. */}
      {!hidesPanelX && (
        <button
          type="button"
          onClick={handleClose}
          aria-label={t("leftRail.collapse")}
          className={cn(
            "absolute right-2 top-2 z-10 inline-flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground",
            "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      )}

      <div
        className={cn(
          "flex min-h-0 flex-1 flex-col overflow-y-auto pt-16",
        )}
      >
        {renderTabBody(leftRailTab, t)}
      </div>
    </aside>
  );
}

/**
 * Exhaustive switch over `LeftRailTab` — adding a new tab forces a
 * TS error here so the panel always knows what to render.
 */
function renderTabBody(
  tab: LeftRailTab,
  t: ReturnType<typeof useMessages>["t"],
): React.ReactNode {
  switch (tab) {
    case "search":
      // Hidden state — the column itself is unmounted before we get
      // here, so this branch only exists to keep the switch exhaustive.
      return null;
    case "directions":
      return <DirectionsPanel />;
    case "categoryResults":
      return <CategoryResultsPanel />;
    case "saved":
      return (
        <div className="p-4 text-sm text-muted-foreground">
          <List className="mb-2 h-4 w-4 opacity-60" aria-hidden="true" />
          <p>{t("leftRail.saved.title")}</p>
          <p className="mt-1 text-xs">{t("leftRail.saved.subtitle")}</p>
        </div>
      );
    case "recents":
      return <RecentsPanel />;
    default:
      return assertNeverTab(tab);
  }
}

// Narrowing for external consumers + ts check.
export type { LeftRailTab };
