"use client";

import { MapView } from "./MapView";

/**
 * Backwards-compatible default export.
 *
 * `MapView` is already client-only (it uses `next/dynamic` with
 * `{ ssr: false }` under the hood to keep MapLibre out of the SSR
 * bundle), so this wrapper simply re-exports it for any caller that
 * still imports from `./MapViewClient`.
 */
export default MapView;
