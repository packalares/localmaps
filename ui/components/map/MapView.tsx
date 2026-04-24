"use client";

import dynamic from "next/dynamic";
import type { ComponentProps } from "react";
import type { MapCanvas as MapCanvasComponent } from "./MapCanvas";

/**
 * Public map component. Renders a `<div>` that MapLibre mounts into,
 * with all controls, event wiring, and URL round-tripping handled by
 * `MapCanvas`. The indirection via `next/dynamic` keeps MapLibre out of
 * the SSR bundle (it touches `window` at import time), while preserving
 * the ergonomic API `<MapView />` → map in a div.
 *
 * Sibling modules that need the live MapLibre instance subscribe to
 * `useMapStore(s => s.map)` — do NOT prop-drill the instance.
 */

export type MapViewProps = ComponentProps<typeof MapCanvasComponent>;

const MapCanvasClient = dynamic<MapViewProps>(
  () => import("./MapCanvas").then((m) => m.MapCanvas),
  {
    ssr: false,
    loading: () => (
      <div
        className="absolute inset-0 flex items-center justify-center bg-muted text-sm text-muted-foreground"
        aria-label="Map loading"
        role="region"
      >
        Loading map…
      </div>
    ),
  },
);

/**
 * Production MapView. Accepts the same props as `MapCanvas` but defers
 * the import so Node / SSR environments receive only the loading stub.
 */
export function MapView(props: MapViewProps) {
  return <MapCanvasClient {...props} />;
}

export default MapView;
