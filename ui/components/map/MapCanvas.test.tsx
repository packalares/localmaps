import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useMapStore } from "@/lib/state/map";
import { resetPmtilesProtocolForTests } from "@/lib/map/protocol";

// MapLibre gets a wholesale mock — we assert on the constructor + handlers
// rather than running the real WebGL renderer.
type Handler = (...args: unknown[]) => void;

vi.mock("maplibre-gl", () => {
  const handlers = new Map<string, Handler>();
  const oneShotHandlers = new Map<string, Handler>();
  const instance = {
    addControl: vi.fn(),
    on: vi.fn((type: string, h: Handler) => {
      handlers.set(type, h);
    }),
    once: vi.fn((type: string, h: Handler) => {
      oneShotHandlers.set(type, h);
    }),
    off: vi.fn((type: string, _h: Handler) => {
      handlers.delete(type);
    }),
    remove: vi.fn(),
    getCenter: () => ({ lat: 10, lng: 20 }),
    getZoom: () => 5,
    getBearing: () => 0,
    getPitch: () => 0,
    getStyle: () => ({ sprite: "spr", sources: {}, layers: [] }),
    setStyle: vi.fn(),
    addProtocol: vi.fn(),
  };
  const MapCtor = vi.fn(() => instance);
  return {
    default: {
      Map: MapCtor,
      NavigationControl: class {},
      GeolocateControl: class {},
      ScaleControl: class {},
      AttributionControl: class {},
      addProtocol: vi.fn(),
    },
    Map: MapCtor,
    NavigationControl: class {},
    GeolocateControl: class {},
    ScaleControl: class {},
    AttributionControl: class {},
    addProtocol: vi.fn(),
    __instance: instance,
    __handlers: handlers,
    __oneShot: oneShotHandlers,
  };
});

vi.mock("pmtiles", () => ({
  Protocol: class {
    tile = vi.fn();
  },
}));

async function loadCanvas() {
  const mod = await import("./MapCanvas");
  return mod.MapCanvas;
}

function withQueryClient(children: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

describe("<MapCanvas />", () => {
  beforeEach(() => {
    resetPmtilesProtocolForTests();
    useMapStore.getState().clear();
    // Stub fetch to a never-resolving placeholder: /api/regions is polled
    // but we don't need to assert on it here.
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})));
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("constructs a MapLibre map and publishes it to the store on load", async () => {
    const MapCanvas = await loadCanvas();
    const maplibre = await import("maplibre-gl");
    const state: { load: Handler | undefined } = { load: undefined };

    render(withQueryClient(<MapCanvas />));

    // Wait for the initial URL-viewport resolution + map construction.
    await waitFor(() => {
      expect((maplibre as unknown as { Map: ReturnType<typeof vi.fn> }).Map)
        .toHaveBeenCalled();
    });

    // Simulate the style finishing loading — this is what populates the
    // Zustand `map` field.
    state.load = (maplibre as unknown as { __handlers: Map<string, Handler> })
      .__handlers.get("load");
    expect(state.load).toBeTypeOf("function");
    act(() => state.load?.());
    expect(useMapStore.getState().map).not.toBeNull();
  });

  it("clears the store map on unmount", async () => {
    const MapCanvas = await loadCanvas();
    const maplibre = await import("maplibre-gl");
    const { unmount } = render(withQueryClient(<MapCanvas />));
    await waitFor(() => {
      expect((maplibre as unknown as { Map: ReturnType<typeof vi.fn> }).Map)
        .toHaveBeenCalled();
    });
    const load = (maplibre as unknown as { __handlers: Map<string, Handler> })
      .__handlers.get("load");
    act(() => load?.());
    expect(useMapStore.getState().map).not.toBeNull();

    unmount();
    expect(useMapStore.getState().map).toBeNull();
  });
});
