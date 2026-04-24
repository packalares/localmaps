import { describe, expect, it, vi } from "vitest";
import {
  HIGHLIGHT_LAYER_ID,
  HIGHLIGHT_SOURCE_ID,
  clearPoiHighlight,
  setPoiHighlight,
  type HighlightMap,
} from "./highlight";

function makeMapStub() {
  const sources = new Map<string, { data: unknown; setData: (d: unknown) => void }>();
  const layers = new Set<string>();
  const map: HighlightMap = {
    getSource(id: string) {
      return sources.get(id);
    },
    getLayer(id: string) {
      return layers.has(id) ? { id } : undefined;
    },
    addSource(id: string, source: { data?: unknown }) {
      sources.set(id, {
        data: source.data,
        setData: (d: unknown) => {
          const s = sources.get(id);
          if (s) s.data = d;
        },
      });
    },
    addLayer(layer: { id: string }) {
      layers.add(layer.id);
    },
    removeLayer(id: string) {
      layers.delete(id);
    },
    removeSource(id: string) {
      sources.delete(id);
    },
  };
  return { map, sources, layers };
}

describe("setPoiHighlight", () => {
  it("no-ops when map is null", () => {
    expect(setPoiHighlight(null, { center: { lat: 1, lon: 2 } })).toBe(false);
  });

  it("adds a source + layer on first call", () => {
    const { map, sources, layers } = makeMapStub();
    const ok = setPoiHighlight(map, { center: { lat: 10, lon: 20 } });
    expect(ok).toBe(true);
    expect(sources.has(HIGHLIGHT_SOURCE_ID)).toBe(true);
    expect(layers.has(HIGHLIGHT_LAYER_ID)).toBe(true);
  });

  it("updates the source in place on subsequent calls", () => {
    const { map, sources } = makeMapStub();
    setPoiHighlight(map, { center: { lat: 10, lon: 20 } });

    const addSourceSpy = vi.spyOn(map, "addSource");
    const addLayerSpy = vi.spyOn(map, "addLayer");
    setPoiHighlight(map, { center: { lat: 11, lon: 21 } });

    expect(addSourceSpy).not.toHaveBeenCalled();
    expect(addLayerSpy).not.toHaveBeenCalled();
    const data = sources.get(HIGHLIGHT_SOURCE_ID)!.data as {
      features: Array<{ geometry: { coordinates: [number, number] } }>;
    };
    expect(data.features[0].geometry.coordinates).toEqual([21, 11]);
  });

  it("clears on null POI", () => {
    const { map, layers, sources } = makeMapStub();
    setPoiHighlight(map, { center: { lat: 1, lon: 2 } });
    expect(setPoiHighlight(map, null)).toBe(true);
    expect(layers.has(HIGHLIGHT_LAYER_ID)).toBe(false);
    expect(sources.has(HIGHLIGHT_SOURCE_ID)).toBe(false);
  });
});

describe("clearPoiHighlight", () => {
  it("returns false when nothing to clear", () => {
    const { map } = makeMapStub();
    expect(clearPoiHighlight(map)).toBe(false);
  });

  it("removes both layer + source when present", () => {
    const { map, layers, sources } = makeMapStub();
    setPoiHighlight(map, { center: { lat: 1, lon: 2 } });
    expect(clearPoiHighlight(map)).toBe(true);
    expect(layers.size).toBe(0);
    expect(sources.size).toBe(0);
  });
});
