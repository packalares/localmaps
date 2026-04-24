"use client";

import dynamic from "next/dynamic";
import type { EmbedPin } from "./params";
import type { EmbedMapInnerProps } from "./EmbedMapInner";

/**
 * Public entry point for the `/embed` viewer.
 *
 * Wrapped in `next/dynamic` with `ssr: false` for the same reason as
 * `MapView`: MapLibre touches `window`/`document` at module load, which
 * breaks SSR. The loading placeholder keeps the iframe visually non-empty
 * while the client bundle resolves.
 */
export interface EmbedMapProps {
  /** Initial map centre. Validated server-side before reaching this component. */
  center: { lat: number; lon: number };
  /** Initial map zoom. 0 ≤ zoom ≤ 22. */
  zoom: number;
  /** Named style to request from the gateway's `/api/styles/{name}.json`. */
  styleName: "light" | "dark" | "print";
  /** Canonical region key or `null` (world/no-filter). */
  region: string | null;
  /** Optional highlighted pin. */
  pin: EmbedPin | null;
}

// Cast: the inner component's props match 1:1 but `next/dynamic` generics
// default to `{}` when inferred from a dynamic-import promise.
const Inner = dynamic<EmbedMapInnerProps>(
  () => import("./EmbedMapInner").then((m) => m.EmbedMapInner),
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

export function EmbedMap(props: EmbedMapProps) {
  return <Inner {...props} />;
}

export default EmbedMap;
