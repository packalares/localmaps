import type maplibregl from "maplibre-gl";

/**
 * Registers the `pmtiles://` protocol handler on MapLibre so styles that
 * reference archive-backed tiles resolve client-side without going through
 * the gateway for every tile. Idempotent: re-calling is a no-op after the
 * first successful registration.
 *
 * MapLibre's `addProtocol` API is module-scoped global state; we guard
 * against double-registration with a module-local flag. Tests use
 * `resetPmtilesProtocolForTests()` to reset.
 */

let registered = false;

type Protocol = {
  tile: (
    params: { url: string; type?: string },
    abort?: AbortController,
  ) => Promise<{ data: ArrayBuffer | null }>;
};

interface MapLibreLike {
  addProtocol: (
    scheme: string,
    loader: Protocol["tile"],
  ) => void;
  removeProtocol?: (scheme: string) => void;
}

export interface RegisterPmtilesOptions {
  /** Injection seam for tests. */
  maplibre?: MapLibreLike;
  /** Injection seam for tests. */
  pmtilesModule?: { Protocol: new () => Protocol };
}

export function registerPmtilesProtocol(
  options: RegisterPmtilesOptions = {},
): boolean {
  if (registered) return false;

  // Dynamic requires keep this callable in Node test envs where the real
  // maplibre/pmtiles modules aren't desirable to pull in.
  const maplibreModule: MapLibreLike =
    options.maplibre ??
    (require("maplibre-gl") as unknown as typeof maplibregl);
  const pmtilesModule =
    options.pmtilesModule ??
    (require("pmtiles") as { Protocol: new () => Protocol });

  const protocol = new pmtilesModule.Protocol();
  maplibreModule.addProtocol("pmtiles", protocol.tile);
  registered = true;
  return true;
}

/** Test-only reset. Safe to call from production code but a no-op-ish. */
export function resetPmtilesProtocolForTests(): void {
  registered = false;
}

/** Whether the protocol has been registered in this module's lifetime. */
export function isPmtilesProtocolRegistered(): boolean {
  return registered;
}
