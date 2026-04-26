"use client";

import { useMemo } from "react";
import {
  Bookmark,
  Check,
  Clock,
  Globe,
  Languages,
  Moon,
  Sun,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useTheme } from "@/components/providers/theme";
import { useMapStore } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import type { Region } from "@/lib/api/schemas";
import { useRecentHistory } from "@/lib/search/history";
import { useMessages } from "@/lib/i18n/provider";
import {
  LOCALE_NATIVE_NAMES,
  SUPPORTED_LOCALES,
  type Locale,
} from "@/lib/i18n/types";
import { toCanonicalRegionKey } from "@/lib/map/region-key";

/**
 * Far-left vertical rail mirroring Google Maps' desktop chrome: a 56px
 * viewport-height column pinned to the screen edge with Saved, Recents,
 * up to four recent-place avatars, and (pinned to the bottom) the
 * region + language pickers. The rail is permanent — not a floating
 * card — so the rest of the chrome sits inside `left-14` of the
 * viewport.
 *
 * Hidden on mobile (`md:flex` on the page wrapper), so this component
 * can assume a desktop/tablet viewport.
 */

const MAX_AVATARS = 4;

/** Four stable gradient classes cycled by index. Tailwind-literal so the
 *  JIT picks them up — do not build these from template strings. */
const AVATAR_GRADIENTS: readonly string[] = [
  "bg-gradient-to-br from-indigo-400 to-purple-500",
  "bg-gradient-to-br from-sky-400 to-cyan-500",
  "bg-gradient-to-br from-emerald-400 to-teal-500",
  "bg-gradient-to-br from-amber-400 to-rose-500",
] as const;

const ACTIVE_REGION_STATES: Region["state"][] = ["ready", "updating"];

function firstLetter(label: string): string {
  const trimmed = label.trim();
  if (trimmed.length === 0) return "?";
  const initial = trimmed[0];
  return initial ? initial.toUpperCase() : "?";
}

export function LeftIconRail() {
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const setViewport = useMapStore((s) => s.setViewport);
  const setSelectedFeature = usePlaceStore((s) => s.setSelectedFeature);
  const map = useMapStore((s) => s.map);
  const viewport = useMapStore((s) => s.viewport);

  // Centralised hook handles localStorage read + storage-event sync;
  // we just slice down to the avatar count.
  const history = useRecentHistory();
  const entries = useMemo(
    () => history.slice(0, MAX_AVATARS),
    [history],
  );

  const iconBtn = cn(
    "inline-flex h-10 w-10 items-center justify-center rounded-full",
    "text-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
  );

  return (
    <nav
      aria-label="Saved, recents, recent places, region and language"
      className={cn(
        "pointer-events-auto fixed inset-y-0 left-0 z-30 flex w-14 flex-col items-center",
        "bg-white dark:bg-neutral-900",
        "border-r border-black/10 dark:border-white/10",
      )}
    >
      <div className="flex flex-col items-center gap-2 pt-4">
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={() => openLeftRail("saved")}
              aria-label="Saved places"
              className={iconBtn}
            >
              <Bookmark className="h-5 w-5" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" align="center">
            Saved
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={() => openLeftRail("recents")}
              aria-label="Recent searches"
              className={iconBtn}
            >
              <Clock className="h-5 w-5" aria-hidden="true" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" align="center">
            Recents
          </TooltipContent>
        </Tooltip>

        {entries.map((entry, index) => {
          const gradient = AVATAR_GRADIENTS[index % AVATAR_GRADIENTS.length];
          const letter = firstLetter(entry.label);
          return (
            <Tooltip key={entry.id}>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  onClick={() => {
                    // Surface the recent in the canonical place store
                    // so the bottom info card opens, matching what a
                    // dropdown-recents click does.
                    setSelectedFeature({
                      kind: "poi",
                      id: entry.id,
                      lat: entry.center.lat,
                      lon: entry.center.lon,
                      name: entry.label,
                      address: entry.label,
                    });
                    // Pan only — keep current zoom (Change 6).
                    let z = viewport.zoom;
                    try {
                      if (map) z = map.getZoom();
                    } catch {
                      /* fall back to store zoom */
                    }
                    if (map) {
                      try {
                        map.flyTo({
                          center: [entry.center.lon, entry.center.lat],
                          zoom: z,
                          essential: true,
                        });
                      } catch {
                        /* ignore — viewport sync below covers fallback */
                      }
                    }
                    setViewport({
                      lat: entry.center.lat,
                      lon: entry.center.lon,
                      zoom: z,
                      bearing: viewport.bearing,
                      pitch: viewport.pitch,
                    });
                  }}
                  aria-label={`Recenter map on ${entry.label}`}
                  className={cn(
                    "inline-flex h-10 w-10 items-center justify-center rounded-full text-[12px] font-semibold text-white shadow-sm",
                    "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                    gradient,
                  )}
                >
                  <span aria-hidden="true">{letter}</span>
                </button>
              </TooltipTrigger>
              <TooltipContent side="right" align="center">
                {entry.label}
              </TooltipContent>
            </Tooltip>
          );
        })}
      </div>

      {/* Spacer pushes the theme + region + language group to the bottom. */}
      <div className="flex-1" aria-hidden="true" />

      <div className="flex flex-col items-center gap-2 pb-4">
        <ThemeRailButton className={iconBtn} />
        <RegionRailButton className={iconBtn} />
        <LanguageRailButton className={iconBtn} />
      </div>
    </nav>
  );
}

/**
 * Standalone theme toggle. Single-click flips light ⇄ dark; the
 * `system` mode is reserved for users who explicitly want OS tracking
 * (no UI for it here — they can clear `localmaps-theme` from devtools).
 * Keeps the rail rhythm: 40×40 round icon button, same hit area as
 * Region + Language. Lives ABOVE the region selector, matching the
 * Saved → Recents → … → Theme → Region → Language order called out
 * in the brief.
 */
function ThemeRailButton({ className }: { className: string }) {
  const { resolvedTheme, setTheme } = useTheme();
  const isDark = resolvedTheme === "dark";
  const next = isDark ? "light" : "dark";
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={() => setTheme(next)}
          aria-label={`Switch to ${next} theme`}
          className={className}
        >
          {isDark ? (
            <Sun className="h-5 w-5" aria-hidden="true" />
          ) : (
            <Moon className="h-5 w-5" aria-hidden="true" />
          )}
        </button>
      </TooltipTrigger>
      <TooltipContent side="right" align="center">
        Theme
      </TooltipContent>
    </Tooltip>
  );
}

/**
 * Region picker anchored to the rail. Reuses the `useMapStore`
 * installed-region + active-region wiring that `RegionSwitcher`
 * exposed from the top-right chrome; the trigger is a 40×40 Globe
 * icon so it fits the rail rhythm.
 */
function RegionRailButton({ className }: { className: string }) {
  const installedRegions = useMapStore((s) => s.installedRegions);
  const activeRegion = useMapStore((s) => s.activeRegion);
  const setActiveRegion = useMapStore((s) => s.setActiveRegion);

  const ready = installedRegions.filter((r) =>
    ACTIVE_REGION_STATES.includes(r.state),
  );

  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-label="Choose active region"
              className={className}
            >
              <Globe className="h-5 w-5" aria-hidden="true" />
            </button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side="right" align="center">
          Region
        </TooltipContent>
      </Tooltip>
      <DropdownMenuContent side="right" align="end" className="min-w-[14rem]">
        <DropdownMenuItem
          onSelect={() => setActiveRegion(null)}
          aria-label="Show all installed regions"
        >
          <span className="flex w-5 items-center justify-center">
            {activeRegion === null ? (
              <Check className="h-4 w-4" aria-hidden="true" />
            ) : null}
          </span>
          <span>All regions</span>
        </DropdownMenuItem>
        {ready.map((r) => {
          const k = toCanonicalRegionKey(r.name);
          const label = r.displayName || r.name;
          return (
            <DropdownMenuItem
              key={k}
              onSelect={() => setActiveRegion(k)}
              aria-label={`Switch to ${label}`}
            >
              <span className="flex w-5 items-center justify-center">
                {activeRegion === k ? (
                  <Check className="h-4 w-4" aria-hidden="true" />
                ) : null}
              </span>
              <span>{label}</span>
            </DropdownMenuItem>
          );
        })}
        {ready.length === 0 && (
          <DropdownMenuItem disabled aria-label="No regions installed">
            <span className="flex w-5 items-center justify-center" />
            <span className="text-muted-foreground">No regions installed</span>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * Language picker. Inline dropdown so the rail stays self-contained —
 * we deliberately do NOT defer to `<LocaleSelector />` here because
 * that component renders its own pill-shaped trigger; the rail needs
 * an icon-only 40×40 button.
 */
function LanguageRailButton({ className }: { className: string }) {
  const { locale, setLocale, t } = useMessages();

  return (
    <Popover>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <button
              type="button"
              aria-label={t("locale.ariaLabel")}
              className={className}
            >
              <Languages className="h-5 w-5" aria-hidden="true" />
            </button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent side="right" align="center">
          Language
        </TooltipContent>
      </Tooltip>
      <PopoverContent side="right" align="end" className="w-48 p-1">
        <ul role="listbox" aria-label="Choose interface language" className="flex flex-col">
          {SUPPORTED_LOCALES.map((code) => (
            <LocaleRailItem
              key={code}
              code={code}
              active={code === locale}
              onSelect={() => setLocale(code)}
            />
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  );
}

function LocaleRailItem({
  code,
  active,
  onSelect,
}: {
  code: Locale;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <li>
      <button
        type="button"
        role="option"
        aria-selected={active}
        onClick={onSelect}
        aria-label={`Switch to ${LOCALE_NATIVE_NAMES[code]}`}
        className={cn(
          "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm",
          "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          active ? "text-foreground" : "text-muted-foreground",
        )}
      >
        <span className="flex w-5 items-center justify-center">
          {active ? <Check className="h-4 w-4" aria-hidden="true" /> : null}
        </span>
        <span>{LOCALE_NATIVE_NAMES[code]}</span>
      </button>
    </li>
  );
}
