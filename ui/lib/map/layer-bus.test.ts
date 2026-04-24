import { describe, expect, it, vi } from "vitest";
import { registerLayer, unregisterLayer, type LayerBusMap } from "./layer-bus";

/** Hand-rolled fake that records every call and maintains an id registry. */
function makeFakeMap() {
  const sources = new Set<string>();
  const layers = new Set<string>();
  const calls: Array<{ fn: string; args: unknown[] }> = [];
  const map: LayerBusMap = {
    getSource: (id) => (sources.has(id) ? { id } : undefined),
    getLayer: (id) => (layers.has(id) ? { id } : undefined),
    addSource: vi.fn((id, src) => {
      sources.add(id);
      calls.push({ fn: "addSource", args: [id, src] });
    }),
    addLayer: vi.fn((layer, beforeId) => {
      layers.add(
        (layer as { id: string }).id,
      );
      calls.push({ fn: "addLayer", args: [layer, beforeId] });
    }),
    removeSource: vi.fn((id) => {
      sources.delete(id);
      calls.push({ fn: "removeSource", args: [id] });
    }),
    removeLayer: vi.fn((id) => {
      layers.delete(id);
      calls.push({ fn: "removeLayer", args: [id] });
    }),
  };
  return { map, sources, layers, calls };
}

describe("layer-bus", () => {
  const source = {
    type: "geojson" as const,
    data: { type: "FeatureCollection", features: [] },
  };
  const lineLayer = {
    type: "line" as const,
    paint: { "line-width": 4 },
  };

  it("adds a new source + layer on first register", () => {
    const { map, sources, layers } = makeFakeMap();
    registerLayer(
      map,
      "route",
      source as unknown as import("maplibre-gl").SourceSpecification,
      lineLayer,
    );
    expect(sources.has("route")).toBe(true);
    expect(layers.has("route")).toBe(true);
    expect(map.addSource).toHaveBeenCalledTimes(1);
    expect(map.addLayer).toHaveBeenCalledTimes(1);
  });

  it("replaces an existing source + layer when id is reused", () => {
    const { map, sources, layers } = makeFakeMap();
    registerLayer(
      map,
      "route",
      source as unknown as import("maplibre-gl").SourceSpecification,
      lineLayer,
    );
    registerLayer(
      map,
      "route",
      source as unknown as import("maplibre-gl").SourceSpecification,
      lineLayer,
    );
    expect(map.removeLayer).toHaveBeenCalledWith("route");
    expect(map.removeSource).toHaveBeenCalledWith("route");
    expect(map.addSource).toHaveBeenCalledTimes(2);
    expect(map.addLayer).toHaveBeenCalledTimes(2);
    expect(sources.has("route")).toBe(true);
    expect(layers.has("route")).toBe(true);
  });

  it("unregister removes both layer and source", () => {
    const { map, sources, layers } = makeFakeMap();
    registerLayer(
      map,
      "poi",
      source as unknown as import("maplibre-gl").SourceSpecification,
      { type: "circle" as const },
    );
    unregisterLayer(map, "poi");
    expect(sources.has("poi")).toBe(false);
    expect(layers.has("poi")).toBe(false);
  });

  it("unregister is a no-op for unknown ids", () => {
    const { map } = makeFakeMap();
    unregisterLayer(map, "nope");
    expect(map.removeLayer).not.toHaveBeenCalled();
    expect(map.removeSource).not.toHaveBeenCalled();
  });

  it("forwards beforeId to addLayer when supplied", () => {
    const { map } = makeFakeMap();
    registerLayer(
      map,
      "highlight",
      source as unknown as import("maplibre-gl").SourceSpecification,
      { type: "circle" as const },
      "labels",
    );
    expect(map.addLayer).toHaveBeenCalledWith(
      expect.objectContaining({ id: "highlight", source: "highlight" }),
      "labels",
    );
  });
});
