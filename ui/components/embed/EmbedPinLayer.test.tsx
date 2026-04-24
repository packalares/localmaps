import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";

// Minimal MapLibre shim — we only need the bits EmbedPinLayer touches.
// `vi.mock` is hoisted above the imports, so internal state lives inside
// the factory and is fished back out via the mock module object.
vi.mock("maplibre-gl", () => {
  const popupInstance = {
    setLngLat: vi.fn().mockReturnThis(),
    setText: vi.fn().mockReturnThis(),
    addTo: vi.fn().mockReturnThis(),
    remove: vi.fn(),
  };
  const PopupCtor = vi.fn(() => popupInstance);
  return {
    default: { Popup: PopupCtor },
    Popup: PopupCtor,
    __popupInstance: popupInstance,
    __popupCtor: PopupCtor,
  };
});

// Imported after the mock so the bound reference resolves correctly.
import { EmbedPinLayer } from "./EmbedPinLayer";
import * as maplibreMock from "maplibre-gl";

const { __popupInstance: popupInstance, __popupCtor: PopupCtor } =
  maplibreMock as unknown as {
    __popupInstance: {
      setLngLat: ReturnType<typeof vi.fn>;
      setText: ReturnType<typeof vi.fn>;
      addTo: ReturnType<typeof vi.fn>;
      remove: ReturnType<typeof vi.fn>;
    };
    __popupCtor: ReturnType<typeof vi.fn>;
  };

type LayerHandler = (...args: unknown[]) => void;

// buildFakeMap yields a Map-like object with the subset of the MapLibre
// API EmbedPinLayer consults. It records source/layer add/remove calls so
// tests can assert on idempotency.
function buildFakeMap() {
  const sources = new Set<string>();
  const layers = new Set<string>();
  const onceHandlers = new Map<string, LayerHandler>();
  const clickHandlers = new Map<string, LayerHandler>();
  const map = {
    isStyleLoaded: vi.fn(() => true),
    addSource: vi.fn((id: string) => {
      sources.add(id);
    }),
    addLayer: vi.fn((spec: { id: string }) => {
      layers.add(spec.id);
    }),
    removeLayer: vi.fn((id: string) => {
      layers.delete(id);
    }),
    removeSource: vi.fn((id: string) => {
      sources.delete(id);
    }),
    getLayer: vi.fn((id: string) => (layers.has(id) ? { id } : undefined)),
    getSource: vi.fn((id: string) => (sources.has(id) ? { id } : undefined)),
    once: vi.fn((type: string, h: LayerHandler) => {
      onceHandlers.set(type, h);
    }),
    off: vi.fn(),
    on: vi.fn((type: string, idOrHandler: unknown, h?: LayerHandler) => {
      if (typeof idOrHandler === "string" && typeof h === "function") {
        clickHandlers.set(`${type}:${idOrHandler}`, h);
      } else if (typeof idOrHandler === "function") {
        clickHandlers.set(type, idOrHandler as LayerHandler);
      }
    }),
  };
  return { map, sources, layers, clickHandlers };
}

describe("<EmbedPinLayer />", () => {
  beforeEach(() => {
    popupInstance.setLngLat.mockClear();
    popupInstance.setText.mockClear();
    popupInstance.addTo.mockClear();
    popupInstance.remove.mockClear();
    PopupCtor.mockClear();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("adds a source + layer when a pin is present", () => {
    const fake = buildFakeMap();
    // Cast: the fake is structurally compatible with the props we use.
    render(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={{ lat: 44.43, lon: 26.1, label: "Bucharest" }}
      />,
    );
    expect(fake.map.addSource).toHaveBeenCalledTimes(1);
    expect(fake.map.addLayer).toHaveBeenCalledTimes(1);
    expect(fake.sources.size).toBe(1);
    expect(fake.layers.size).toBe(1);
  });

  it("is idempotent across pin updates (removes before re-adding)", () => {
    const fake = buildFakeMap();
    const { rerender } = render(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={{ lat: 0, lon: 0 }}
      />,
    );
    rerender(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={{ lat: 1, lon: 2 }}
      />,
    );
    // The second pin should have cleaned up before adding — one add per
    // pin, never stacked; removeLayer/removeSource were called for the
    // previous pin's lifetime.
    expect(fake.map.addLayer).toHaveBeenCalledTimes(2);
    expect(fake.map.removeLayer).toHaveBeenCalled();
    expect(fake.layers.size).toBe(1);
    expect(fake.sources.size).toBe(1);
  });

  it("removes the pin when pin becomes null", () => {
    const fake = buildFakeMap();
    const { rerender } = render(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={{ lat: 0, lon: 0 }}
      />,
    );
    rerender(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={null}
      />,
    );
    expect(fake.layers.size).toBe(0);
    expect(fake.sources.size).toBe(0);
  });

  it("cleans up on unmount", () => {
    const fake = buildFakeMap();
    const { unmount } = render(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={{ lat: 0, lon: 0 }}
      />,
    );
    unmount();
    expect(fake.layers.size).toBe(0);
    expect(fake.sources.size).toBe(0);
  });

  it("opens a popup when the layer is clicked", () => {
    const fake = buildFakeMap();
    render(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={{ lat: 10, lon: 20, label: "Hi" }}
      />,
    );
    const handler = fake.clickHandlers.get(
      "click:localmaps-embed-pin-layer",
    );
    expect(handler).toBeTypeOf("function");
    act(() => {
      handler?.();
    });
    expect(PopupCtor).toHaveBeenCalledTimes(1);
    expect(popupInstance.setText).toHaveBeenCalledWith("Hi");
    expect(popupInstance.addTo).toHaveBeenCalledTimes(1);
  });

  it("does nothing until the map prop is non-null", () => {
    const { rerender } = render(
      <EmbedPinLayer map={null} pin={{ lat: 0, lon: 0 }} />,
    );
    // Provide the map; pin should install.
    const fake = buildFakeMap();
    rerender(
      <EmbedPinLayer
        map={fake.map as unknown as import("maplibre-gl").Map}
        pin={{ lat: 0, lon: 0 }}
      />,
    );
    expect(fake.map.addSource).toHaveBeenCalled();
  });
});
