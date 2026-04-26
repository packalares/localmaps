"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  ArrowUpDown,
  Bike,
  Bus,
  Car,
  Circle,
  Clock,
  Crosshair,
  Footprints,
  Plane,
  X,
} from "lucide-react";
import type { GeocodeResult, RouteMode } from "@/lib/api/schemas";
import { useMapStore } from "@/lib/state/map";
import { useDirectionsStore } from "@/lib/state/directions";
import { useRouteRender } from "@/lib/directions/use-route-render";
import { useRouteSync } from "@/lib/directions/use-route-sync";
import { useGeocodeAutocomplete } from "@/lib/api/hooks";
import { useRecentHistory } from "@/lib/search/history";
import { ResultCard } from "@/components/search/ResultCard";
import { ExportMenu } from "./ExportMenu";
import { RouteOptions } from "./RouteOptions";
import { RouteSummary } from "./RouteSummary";
import { TurnList } from "./TurnList";
import { cn } from "@/lib/utils";
import { useMessages } from "@/lib/i18n/provider";

/**
 * Google-Maps-style directions panel.
 *
 * Layout:
 *   1. Mode toggle row (Car / Bike / Walk / Bus / Plane) + close X.
 *   2. From input (Crosshair icon) + swap button + To input (Circle icon).
 *   3. "Your location" quick-pick row.
 *   4. Recent places list from `localmaps.search.history.v1`.
 *   5. RouteSummary / TurnList once a route resolves.
 */

const HISTORY_DISPLAY_MAX = 10;

interface ModeDef {
  mode: RouteMode | "transit" | "plane";
  /** Maps the UI mode to a valhalla-supported routing mode, when possible. */
  routeMode: RouteMode | null;
  icon: typeof Car;
  label: string;
  disabled?: boolean;
  tooltip?: string;
}

const MODE_DEFS: ModeDef[] = [
  { mode: "auto", routeMode: "auto", icon: Car, label: "Driving" },
  { mode: "bicycle", routeMode: "bicycle", icon: Bike, label: "Cycling" },
  {
    mode: "pedestrian",
    routeMode: "pedestrian",
    icon: Footprints,
    label: "Walking",
  },
  {
    mode: "transit",
    routeMode: null,
    icon: Bus,
    label: "Transit",
    disabled: true,
    tooltip: "Transit routing is not supported yet",
  },
  {
    mode: "plane",
    routeMode: null,
    icon: Plane,
    label: "Flights",
    disabled: true,
    tooltip: "Flight routing is not supported",
  },
];

export interface DirectionsPanelProps {
  /** Which modes to show in the toggle. Defaults to all five. */
  enabledModes?: Array<ModeDef["mode"]>;
  units?: "metric" | "imperial";
}

interface HistoryEntry {
  id: string;
  name: string;
  address?: string;
  hours?: string;
  result: GeocodeResult;
}

function toHistoryEntry(result: GeocodeResult): HistoryEntry {
  const address = result.address
    ? [
        result.address.street,
        result.address.city,
        result.address.region,
        result.address.country,
      ]
        .filter((s): s is string => typeof s === "string" && s.length > 0)
        .join(", ")
    : undefined;
  return {
    id: result.id,
    name: result.label,
    address: address || undefined,
    result,
  };
}

export function DirectionsPanel({
  enabledModes,
  units,
}: DirectionsPanelProps = {}) {
  const waypoints = useDirectionsStore((s) => s.waypoints);
  const mode = useDirectionsStore((s) => s.mode);
  const options = useDirectionsStore((s) => s.options);
  const route = useDirectionsStore((s) => s.route);
  const alternatives = useDirectionsStore((s) => s.alternatives);
  const setMode = useDirectionsStore((s) => s.setMode);
  const setOptions = useDirectionsStore((s) => s.setOptions);
  const setWaypoint = useDirectionsStore((s) => s.setWaypoint);
  const setWaypointFromPoint = useDirectionsStore(
    (s) => s.setWaypointFromPoint,
  );
  const swapEnds = useDirectionsStore((s) => s.swapEnds);
  const setRoute = useDirectionsStore((s) => s.setRoute);
  const reset = useDirectionsStore((s) => s.reset);

  const viewport = useMapStore((s) => s.viewport);
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const { t } = useMessages();

  const { isError } = useRouteSync();
  useRouteRender();

  // From = waypoint[0]; To = last waypoint.
  const fromIndex = 0;
  const toIndex = waypoints.length - 1;
  const from = waypoints[fromIndex];
  const to = waypoints[toIndex];

  /** Tracks which input was most recently focused so a recents click
   * routes to the right slot. Destination wins by default. */
  const [lastFocused, setLastFocused] = useState<"from" | "to">("to");

  const visibleModes = useMemo(() => {
    const allowed =
      enabledModes ?? MODE_DEFS.map((m) => m.mode);
    return MODE_DEFS.filter((m) => allowed.includes(m.mode));
  }, [enabledModes]);

  const handleModeClick = useCallback(
    (def: ModeDef) => {
      if (def.disabled || !def.routeMode) return;
      setMode(def.routeMode);
    },
    [setMode],
  );

  const handleClose = useCallback(() => {
    openLeftRail("search");
  }, [openLeftRail]);

  const handleUseMyLocation = useCallback(() => {
    if (typeof navigator === "undefined" || !navigator.geolocation) return;
    try {
      navigator.geolocation.getCurrentPosition(
        (pos) => {
          setWaypointFromPoint(
            fromIndex,
            { lng: pos.coords.longitude, lat: pos.coords.latitude },
            "Your location",
          );
        },
        () => {
          /* silent fallback */
        },
        { enableHighAccuracy: false, timeout: 8000, maximumAge: 60_000 },
      );
    } catch {
      /* silent fallback */
    }
  }, [setWaypointFromPoint]);

  const handleSelectResult = useCallback(
    (index: number, r: GeocodeResult) => {
      setWaypoint(index, {
        label: r.label,
        lngLat: { lng: r.center.lon, lat: r.center.lat },
      });
    },
    [setWaypoint],
  );

  const handleRecentClick = useCallback(
    (entry: HistoryEntry) => {
      const targetIndex = lastFocused === "from" ? fromIndex : toIndex;
      handleSelectResult(targetIndex, entry.result);
    },
    [handleSelectResult, lastFocused, toIndex],
  );

  const history = useRecentHistory();
  const recents = useMemo(
    () =>
      history
        .slice(0, HISTORY_DISPLAY_MAX)
        .map((r) => toHistoryEntry(r)),
    [history],
  );

  return (
    <div className="flex h-full flex-col">
      {/* Mode toggle row + close */}
      <div
        role="tablist"
        aria-label="Travel mode"
        className="flex items-center gap-1 border-b border-border px-2 py-2"
      >
        {visibleModes.map((def) => {
          const Icon = def.icon;
          const active =
            def.routeMode !== null && def.routeMode === mode;
          return (
            <button
              key={def.mode}
              type="button"
              role="tab"
              aria-selected={active}
              aria-label={def.label}
              title={def.tooltip ?? def.label}
              disabled={def.disabled}
              onClick={() => handleModeClick(def)}
              className={cn(
                "inline-flex h-10 w-10 items-center justify-center rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                active
                  ? "bg-primary/10 text-primary"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground",
                def.disabled && "cursor-not-allowed opacity-40 hover:bg-transparent",
              )}
            >
              <Icon className="h-5 w-5" aria-hidden={true} />
            </button>
          );
        })}
        <div className="flex-1" />
        <button
          type="button"
          aria-label="Close directions"
          onClick={handleClose}
          className="inline-flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <X className="h-4 w-4" aria-hidden={true} />
        </button>
      </div>

      {/* From / To inputs + swap */}
      <div className="relative px-3 pt-3">
        <EndpointInput
          kind="from"
          waypoint={from}
          placeholder="Choose starting point, or click on the map"
          onChangeLabel={(label) => setWaypoint(fromIndex, { label })}
          onSelect={(r) => handleSelectResult(fromIndex, r)}
          onFocus={() => setLastFocused("from")}
          focusLngLat={{ lat: viewport.lat, lon: viewport.lon }}
          leading={
            <span
              aria-hidden={true}
              className="inline-flex h-5 w-5 items-center justify-center text-muted-foreground"
            >
              <Crosshair className="h-4 w-4" />
            </span>
          }
        />
        <EndpointInput
          kind="to"
          waypoint={to}
          placeholder="Choose destination"
          onChangeLabel={(label) => setWaypoint(toIndex, { label })}
          onSelect={(r) => handleSelectResult(toIndex, r)}
          onFocus={() => setLastFocused("to")}
          focusLngLat={{ lat: viewport.lat, lon: viewport.lon }}
          leading={
            <span
              aria-hidden={true}
              className="inline-flex h-5 w-5 items-center justify-center text-muted-foreground"
            >
              <Circle className="h-3.5 w-3.5" />
            </span>
          }
        />
        <button
          type="button"
          aria-label={t("directions.swap")}
          onClick={swapEnds}
          className="absolute right-4 top-1/2 inline-flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-full border border-border bg-background text-muted-foreground shadow-sm hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ArrowUpDown className="h-4 w-4" aria-hidden={true} />
        </button>
      </div>

      {/* Route options + status */}
      <div className="flex flex-col gap-2 px-3 pt-3">
        <RouteOptions value={options} onChange={setOptions} />
        {isError && (
          <p
            role="alert"
            className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive"
          >
            {t("directions.error.noRoute")}
          </p>
        )}
      </div>

      {/* Your location + recents */}
      <div className="mt-3 flex flex-col border-t border-border">
        <button
          type="button"
          onClick={handleUseMyLocation}
          className="flex items-center gap-3 px-3 py-3 text-left hover:bg-muted focus:outline-none focus-visible:bg-muted"
        >
          <span
            aria-hidden={true}
            className="inline-flex h-5 w-5 items-center justify-center text-primary"
          >
            <Crosshair className="h-5 w-5" />
          </span>
          <span className="text-sm font-medium text-foreground">
            Your location
          </span>
        </button>

        {recents.length > 0 && (
          <ul
            aria-label="Recent places"
            className="flex flex-col border-t border-border"
          >
            {recents.map((entry) => (
              <li key={entry.id}>
                <button
                  type="button"
                  onClick={() => handleRecentClick(entry)}
                  className="flex w-full items-start gap-3 px-3 py-3 text-left hover:bg-muted focus:outline-none focus-visible:bg-muted"
                >
                  <span
                    aria-hidden={true}
                    className="mt-0.5 inline-flex h-5 w-5 flex-shrink-0 items-center justify-center text-muted-foreground"
                  >
                    <Clock className="h-4 w-4" />
                  </span>
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate text-sm font-medium text-foreground">
                      {entry.name}
                    </span>
                    {entry.address && (
                      <span className="truncate text-xs text-muted-foreground">
                        {entry.address}
                      </span>
                    )}
                    {entry.hours && (
                      <span className="truncate text-xs text-muted-foreground">
                        {entry.hours}
                      </span>
                    )}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {route && (
        <div className="flex flex-col gap-2 border-t border-border px-3 py-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Route
            </span>
            <ExportMenu route={route} waypoints={waypoints} />
          </div>
          <RouteSummary
            route={route}
            alternatives={alternatives}
            onSelectAlternative={(id) => {
              const next = alternatives.find((r) => r.id === id);
              if (next) setRoute(next, alternatives);
            }}
            onClear={() => {
              reset();
            }}
            units={units}
          />
          <div className="flex-1 overflow-auto rounded-md border border-border">
            <TurnList route={route} units={units} />
          </div>
        </div>
      )}
    </div>
  );
}

/** Single From/To endpoint field with leading icon and autocomplete. */
interface EndpointInputProps {
  kind: "from" | "to";
  waypoint: { label: string; lngLat: { lng: number; lat: number } | null };
  placeholder: string;
  leading: ReactNode;
  onChangeLabel: (label: string) => void;
  onSelect: (r: GeocodeResult) => void;
  onFocus?: () => void;
  focusLngLat?: { lat: number; lon: number };
}

function EndpointInput({
  kind,
  waypoint,
  placeholder,
  leading,
  onChangeLabel,
  onSelect,
  onFocus,
  focusLngLat,
}: EndpointInputProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const ariaLabel = kind === "from" ? "From" : "To";

  const ac = useGeocodeAutocomplete({
    q: waypoint.label,
    focus: focusLngLat,
    enabled: open && !waypoint.lngLat,
  });
  const results = ac.data?.results ?? [];

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [open]);

  return (
    <div
      ref={containerRef}
      className="relative mb-2 flex items-center gap-2 rounded-full border border-border bg-background px-3 py-2 focus-within:border-ring"
    >
      {leading}
      <input
        type="text"
        aria-label={ariaLabel}
        placeholder={placeholder}
        value={waypoint.label}
        onChange={(e) => {
          onChangeLabel(e.target.value);
          setOpen(true);
        }}
        onFocus={() => {
          setOpen(true);
          onFocus?.();
        }}
        className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
      />
      {open && results.length > 0 && (
        <div
          role="listbox"
          aria-label={`${ariaLabel} suggestions`}
          className="absolute left-0 right-0 top-full z-10 mt-1 max-h-60 overflow-auto rounded-md border border-border bg-popover text-sm shadow-md"
        >
          {results.map((r) => (
            <ResultCard
              key={r.id}
              result={r}
              origin={focusLngLat ?? undefined}
              onSelect={(picked) => {
                onSelect(picked);
                setOpen(false);
              }}
            />
          ))}
        </div>
      )}
    </div>
  );
}
