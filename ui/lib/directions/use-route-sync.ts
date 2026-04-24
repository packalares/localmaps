"use client";

import { useEffect, useMemo } from "react";
import { useRoute } from "@/lib/api/hooks";
import type { RouteRequest } from "@/lib/api/schemas";
import { useDirectionsStore } from "@/lib/state/directions";

/**
 * Fires `/api/route` whenever the resolved waypoints, mode, or avoid
 * options change. Writes the first returned route (and the full
 * alternatives list) into the directions store.
 *
 * The TanStack Query cache is keyed off the serialised waypoints via
 * the mutation key, giving us scoped invalidation (only re-runs when
 * something actually changed).
 */
export function useRouteSync(): { isError: boolean; isPending: boolean } {
  const waypoints = useDirectionsStore((s) => s.waypoints);
  const mode = useDirectionsStore((s) => s.mode);
  const options = useDirectionsStore((s) => s.options);
  const setRoute = useDirectionsStore((s) => s.setRoute);
  const routeMutation = useRoute();

  const depsKey = useMemo(
    () =>
      JSON.stringify({
        ids: waypoints.map((w) =>
          w.lngLat
            ? `${w.id}:${w.lngLat.lng.toFixed(6)},${w.lngLat.lat.toFixed(6)}`
            : `${w.id}:none`,
        ),
        mode,
        options,
      }),
    [waypoints, mode, options],
  );

  useEffect(() => {
    const resolved = waypoints.filter((w) => w.lngLat);
    if (resolved.length < 2) {
      setRoute(null, []);
      return;
    }
    const req: RouteRequest = {
      locations: resolved.map((w) => ({
        lat: w.lngLat!.lat,
        lon: w.lngLat!.lng,
      })),
      mode,
      avoidHighways: options.avoidHighways || undefined,
      avoidTolls: options.avoidTolls || undefined,
      avoidFerries: options.avoidFerries || undefined,
    };
    let cancelled = false;
    routeMutation.mutate(req, {
      onSuccess: (data) => {
        if (cancelled) return;
        const first = data.routes[0] ?? null;
        setRoute(first, data.routes);
      },
    });
    return () => {
      cancelled = true;
    };
    // depsKey captures all relevant inputs in a stable string.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [depsKey]);

  return {
    isError: routeMutation.isError,
    isPending: routeMutation.isPending,
  };
}
