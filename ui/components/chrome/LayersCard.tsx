"use client";

import { useMemo, type ReactNode } from "react";
import {
  Bus,
  CreditCard,
  Cross,
  Film,
  GraduationCap,
  Hotel,
  Layers,
  MoreHorizontal,
  ShoppingBag,
  UtensilsCrossed,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useBreakpoint } from "@/lib/responsive/use-breakpoint";
import {
  POI_CATEGORIES,
  defaultPoiVisibility,
  useMapStore,
  type PoiCategory,
} from "@/lib/state/map";

/**
 * Bottom-left "Layers" card, modelled on Google Maps desktop. Collapsed
 * state is a compact tile labelled "Layers" with the Lucide `Layers`
 * icon; clicking opens a popover with two sections:
 *
 * - "Show POIs" master toggle that flips every POI category at once
 *   via `setPoiVisibility`.
 * - Per-category checklist (Food, Shopping, Hotels, Transit,
 *   Pharmacies, ATMs & banks, Things to do, Education, More) — each
 *   checkbox drives `setPoiVisibility(cat, visible)`. When the master
 *   toggle is OFF the checkboxes are greyed out (disabled).
 *
 * The light/dark theme switch used to live in this popover; it now
 * sits as a standalone icon button in the LeftIconRail (above the
 * Region + Language pickers), matching Google Maps' separation of
 * style preferences from POI overlays.
 *
 * On mobile the trigger shrinks to an icon-only round button, matching
 * the narrower chrome on small viewports. Popover dismissal (outside
 * click, Escape) is handled by Radix.
 */

interface CategoryDescriptor {
  key: PoiCategory;
  label: string;
  icon: (props: { className?: string }) => ReactNode;
}

const CATEGORY_LIST: readonly CategoryDescriptor[] = [
  {
    key: "food",
    label: "Food",
    icon: ({ className }) => (
      <UtensilsCrossed className={className} aria-hidden="true" />
    ),
  },
  {
    key: "shopping",
    label: "Shopping",
    icon: ({ className }) => (
      <ShoppingBag className={className} aria-hidden="true" />
    ),
  },
  {
    key: "lodging",
    label: "Hotels",
    icon: ({ className }) => <Hotel className={className} aria-hidden="true" />,
  },
  {
    key: "transit",
    label: "Transit",
    icon: ({ className }) => <Bus className={className} aria-hidden="true" />,
  },
  {
    key: "healthcare",
    label: "Pharmacies",
    icon: ({ className }) => <Cross className={className} aria-hidden="true" />,
  },
  {
    key: "services",
    label: "ATMs & banks",
    icon: ({ className }) => (
      <CreditCard className={className} aria-hidden="true" />
    ),
  },
  {
    key: "entertainment",
    label: "Things to do",
    icon: ({ className }) => <Film className={className} aria-hidden="true" />,
  },
  {
    key: "education",
    label: "Education",
    icon: ({ className }) => (
      <GraduationCap className={className} aria-hidden="true" />
    ),
  },
  {
    key: "other",
    label: "More",
    icon: ({ className }) => (
      <MoreHorizontal className={className} aria-hidden="true" />
    ),
  },
] as const;

function allPoisHidden(map: Record<PoiCategory, boolean>): boolean {
  return POI_CATEGORIES.every((c) => !map[c]);
}

export function LayersCard() {
  const bp = useBreakpoint();
  const isMobile = bp === "mobile";

  const poiVisibility = useMapStore((s) => s.poiVisibility);
  const setPoiVisibility = useMapStore((s) => s.setPoiVisibility);
  const replacePoiVisibility = useMapStore((s) => s.replacePoiVisibility);

  const poisOn = useMemo(() => !allPoisHidden(poiVisibility), [poiVisibility]);

  const handlePoisToggle = (next: boolean) => {
    if (next) {
      replacePoiVisibility(defaultPoiVisibility());
    } else {
      for (const cat of POI_CATEGORIES) {
        setPoiVisibility(cat, false);
      }
    }
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        {isMobile ? (
          <button
            type="button"
            aria-label="Layers"
            title="Layers"
            className={cn(
              "chrome-surface-sm pointer-events-auto inline-flex h-8 w-8 items-center justify-center rounded-lg hover:bg-muted",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            )}
          >
            <Layers className="h-4 w-4" aria-hidden="true" />
          </button>
        ) : (
          <button
            type="button"
            aria-label="Layers"
            title="Layers"
            className={cn(
              "chrome-surface-sm pointer-events-auto flex h-14 w-14 flex-col items-center justify-center gap-0.5 rounded-lg text-[11px] font-medium hover:bg-muted",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            )}
          >
            <Layers className="h-4 w-4" aria-hidden="true" />
            <span>Layers</span>
          </button>
        )}
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        sideOffset={8}
        className="pointer-events-auto w-72 p-3"
      >
        <section
          aria-label="Points of interest"
          className="flex flex-col gap-1.5"
        >
          <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Points of interest
          </span>
          <label className="flex cursor-pointer items-center justify-between gap-2 rounded-md px-1 py-1.5 text-sm hover:bg-muted">
            <span>Show POIs</span>
            <span
              role="switch"
              aria-checked={poisOn}
              tabIndex={0}
              onClick={(e) => {
                e.preventDefault();
                handlePoisToggle(!poisOn);
              }}
              onKeyDown={(e) => {
                if (e.key === " " || e.key === "Enter") {
                  e.preventDefault();
                  handlePoisToggle(!poisOn);
                }
              }}
              className={cn(
                "relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border transition-colors",
                "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                poisOn
                  ? "border-transparent bg-primary"
                  : "border-border bg-muted",
              )}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "inline-block h-4 w-4 transform rounded-full bg-background shadow transition-transform",
                  poisOn ? "translate-x-4" : "translate-x-0.5",
                )}
              />
            </span>
          </label>

          <ul
            role="group"
            aria-label="POI categories"
            className={cn(
              "flex flex-col gap-0.5",
              !poisOn && "opacity-60",
            )}
          >
            {CATEGORY_LIST.map((desc) => {
              const checked = poiVisibility[desc.key];
              return (
                <li key={desc.key}>
                  <label
                    className={cn(
                      "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm",
                      poisOn
                        ? "cursor-pointer hover:bg-muted"
                        : "cursor-not-allowed",
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      disabled={!poisOn}
                      data-category={desc.key}
                      aria-label={desc.label}
                      onChange={(e) =>
                        setPoiVisibility(desc.key, e.target.checked)
                      }
                      className={cn(
                        "h-4 w-4 shrink-0 rounded border-border accent-primary",
                        "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                      )}
                    />
                    <desc.icon
                      className={cn(
                        "h-4 w-4",
                        checked && poisOn ? "text-primary" : "opacity-60",
                      )}
                    />
                    <span className="flex-1 text-left">{desc.label}</span>
                  </label>
                </li>
              );
            })}
          </ul>
        </section>
      </PopoverContent>
    </Popover>
  );
}
