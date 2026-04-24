"use client";

import { useCallback, useRef } from "react";
import { ArrowLeftRight, Plus } from "lucide-react";
import type { GeocodeResult, RouteMode } from "@/lib/api/schemas";
import { useMapStore } from "@/lib/state/map";
import { useDirectionsStore } from "@/lib/state/directions";
import { useRouteRender } from "@/lib/directions/use-route-render";
import { useRouteSync } from "@/lib/directions/use-route-sync";
import { ExportMenu } from "./ExportMenu";
import { ModeToggle } from "./ModeToggle";
import { RouteOptions } from "./RouteOptions";
import { RouteSummary } from "./RouteSummary";
import { TurnList } from "./TurnList";
import { WaypointRow } from "./WaypointRow";
import { useMessages } from "@/lib/i18n/provider";

function letterFor(index: number, count: number): string {
  if (index === 0) return "A";
  if (index === count - 1) return String.fromCharCode("A".charCodeAt(0) + 1);
  return String.fromCharCode("A".charCodeAt(0) + index);
}

export interface DirectionsPanelProps {
  /** Which modes to show in the toggle. Defaults to all four. */
  enabledModes?: RouteMode[];
  units?: "metric" | "imperial";
}

export function DirectionsPanel({
  enabledModes,
  units,
}: DirectionsPanelProps) {
  const waypoints = useDirectionsStore((s) => s.waypoints);
  const mode = useDirectionsStore((s) => s.mode);
  const options = useDirectionsStore((s) => s.options);
  const route = useDirectionsStore((s) => s.route);
  const alternatives = useDirectionsStore((s) => s.alternatives);
  const setMode = useDirectionsStore((s) => s.setMode);
  const setOptions = useDirectionsStore((s) => s.setOptions);
  const setWaypoint = useDirectionsStore((s) => s.setWaypoint);
  const addWaypoint = useDirectionsStore((s) => s.addWaypoint);
  const removeWaypoint = useDirectionsStore((s) => s.removeWaypoint);
  const reorderWaypoints = useDirectionsStore((s) => s.reorderWaypoints);
  const swapEnds = useDirectionsStore((s) => s.swapEnds);
  const setRoute = useDirectionsStore((s) => s.setRoute);
  const reset = useDirectionsStore((s) => s.reset);

  const viewport = useMapStore((s) => s.viewport);
  const { t } = useMessages();

  const { isError } = useRouteSync();
  useRouteRender();

  const dragFrom = useRef<number | null>(null);

  const handleSelect = useCallback(
    (index: number) => (r: GeocodeResult) => {
      setWaypoint(index, {
        label: r.label,
        lngLat: { lng: r.center.lon, lat: r.center.lat },
      });
    },
    [setWaypoint],
  );

  const onDragStart = (index: number) => {
    dragFrom.current = index;
  };
  const onDrop = (to: number) => {
    const from = dragFrom.current;
    dragFrom.current = null;
    if (from === null || from === to) return;
    reorderWaypoints(from, to);
  };

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <ModeToggle value={mode} onChange={setMode} modes={enabledModes} />
        <button
          type="button"
          aria-label={t("directions.swap")}
          onClick={swapEnds}
          className="inline-flex h-8 w-8 items-center justify-center rounded-full border border-border bg-background text-muted-foreground hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ArrowLeftRight className="h-4 w-4" aria-hidden={true} />
        </button>
      </div>

      <ul
        role="list"
        aria-label={t("directions.waypoints.ariaLabel")}
        className="flex flex-col gap-1"
      >
        {waypoints.map((w, i) => (
          <div
            key={w.id}
            onDrop={(e) => {
              e.preventDefault();
              onDrop(i);
            }}
            onDragOver={(e) => e.preventDefault()}
          >
            <WaypointRow
              waypoint={w}
              index={i}
              count={waypoints.length}
              letter={letterFor(i, waypoints.length)}
              canRemove={waypoints.length > 2}
              onChangeLabel={(label) => setWaypoint(i, { label })}
              onSelect={handleSelect(i)}
              onRemove={() => removeWaypoint(i)}
              onMoveUp={() => reorderWaypoints(i, i - 1)}
              onMoveDown={() => reorderWaypoints(i, i + 1)}
              onDragStart={onDragStart}
              onDrop={() => onDrop(i)}
              focusLngLat={{ lat: viewport.lat, lon: viewport.lon }}
            />
          </div>
        ))}
      </ul>

      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          onClick={addWaypoint}
          className="inline-flex items-center gap-1 rounded-full border border-border bg-background px-3 py-1 text-xs font-medium text-foreground hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Plus className="h-3.5 w-3.5" aria-hidden={true} />
          {t("directions.addStop")}
        </button>
        {route && <ExportMenu route={route} waypoints={waypoints} />}
      </div>

      <RouteOptions value={options} onChange={setOptions} />

      {isError && (
        <p
          role="alert"
          className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive"
        >
          {t("directions.error.noRoute")}
        </p>
      )}

      {route && (
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
      )}

      {route && (
        <div className="flex-1 overflow-auto rounded-md border border-border">
          <TurnList route={route} units={units} />
        </div>
      )}
    </div>
  );
}
