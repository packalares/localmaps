import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render } from "@testing-library/react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { SelectedFeatureSync } from "./SelectedFeatureSync";
import { useMapStore } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";

/**
 * Build a minimal MapLibre double that exposes just enough surface for
 * the hit-test path: `getLayer` returns truthy for a given id and
 * `queryRenderedFeatures` returns the canned hits.
 */
function fakeMap(
  knownLayers: string[],
  hits: Array<{
    id?: string | number;
    properties?: Record<string, unknown>;
  }>,
): MapLibreMap {
  const layerSet = new Set(knownLayers);
  return {
    getLayer: (id: string) => (layerSet.has(id) ? ({ id } as unknown) : undefined),
    queryRenderedFeatures: () => hits as unknown[],
  } as unknown as MapLibreMap;
}

describe("<SelectedFeatureSync />", () => {
  beforeEach(() => {
    usePlaceStore.getState().clearSelectedFeature();
    useMapStore.getState().clear();
  });
  afterEach(() => {
    usePlaceStore.getState().clearSelectedFeature();
    useMapStore.getState().clear();
  });

  it("publishes a plain point feature when no map is attached", () => {
    render(<SelectedFeatureSync />);
    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 26.1, lat: 44.5 },
        point: { x: 100, y: 200 },
        timestamp: Date.now(),
      });
    });

    const f = usePlaceStore.getState().selectedFeature;
    expect(f).not.toBeNull();
    expect(f!.kind).toBe("point");
    expect(f!.lat).toBeCloseTo(44.5);
    expect(f!.lon).toBeCloseTo(26.1);
  });

  it("publishes a POI feature when the click hits a poi layer", () => {
    const map = fakeMap(
      ["poi-food"],
      [
        {
          id: "abc",
          properties: {
            id: "abc",
            name: "Cafe Lume",
            class: "cafe",
          },
        },
      ],
    );
    useMapStore.getState().setMap(map);

    render(<SelectedFeatureSync />);
    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 13.4, lat: 52.5 },
        point: { x: 50, y: 60 },
        timestamp: Date.now(),
      });
    });

    const f = usePlaceStore.getState().selectedFeature;
    expect(f).not.toBeNull();
    expect(f!.kind).toBe("poi");
    expect(f!.id).toBe("abc");
    expect(f!.name).toBe("Cafe Lume");
    expect(f!.categoryIcon).toBe("cafe");
  });

  it("falls back to a point feature when the map throws on hit-test", () => {
    const map = {
      getLayer: () => ({ id: "poi-food" }),
      queryRenderedFeatures: () => {
        throw new Error("style not loaded");
      },
    } as unknown as MapLibreMap;
    useMapStore.getState().setMap(map);

    render(<SelectedFeatureSync />);
    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 1, lat: 2 },
        point: { x: 0, y: 0 },
        timestamp: Date.now(),
      });
    });

    const f = usePlaceStore.getState().selectedFeature;
    expect(f).not.toBeNull();
    expect(f!.kind).toBe("point");
  });
});

describe("<SelectedFeatureSync /> cascade-close", () => {
  beforeEach(() => {
    usePlaceStore.getState().clearSelectedFeature();
    useMapStore.getState().clear();
  });
  afterEach(() => {
    usePlaceStore.getState().clearSelectedFeature();
    useMapStore.getState().clear();
  });

  it("closes the info card on the next click instead of dropping a new pin", () => {
    // Pre-arrange: a feature is already selected (info card visible).
    usePlaceStore.getState().setSelectedFeature({
      kind: "point",
      lat: 10,
      lon: 20,
    });

    render(<SelectedFeatureSync />);
    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 1, lat: 2 },
        point: { x: 0, y: 0 },
        timestamp: Date.now(),
      });
    });

    // Expect: feature cleared, NOT replaced with the new click coords.
    expect(usePlaceStore.getState().selectedFeature).toBeNull();
    // pendingClick is consumed (cleared) so other listeners don't fire.
    expect(useMapStore.getState().pendingClick).toBeNull();
  });

  it("closes an open side panel on the next click instead of dropping a pin", () => {
    useMapStore.getState().openLeftRail("directions");
    render(<SelectedFeatureSync />);
    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 1, lat: 2 },
        point: { x: 0, y: 0 },
        timestamp: Date.now(),
      });
    });

    expect(useMapStore.getState().leftRailTab).toBe("search");
    // No new feature was published — the click was consumed by the
    // panel-close step.
    expect(usePlaceStore.getState().selectedFeature).toBeNull();
    expect(useMapStore.getState().pendingClick).toBeNull();
  });
});

describe("usePlaceStore", () => {
  beforeEach(() => {
    usePlaceStore.getState().clearSelectedFeature();
  });

  it("setSelectedFeature + clearSelectedFeature flip the slice", () => {
    usePlaceStore.getState().setSelectedFeature({
      kind: "point",
      lat: 1,
      lon: 2,
    });
    expect(usePlaceStore.getState().selectedFeature).not.toBeNull();
    usePlaceStore.getState().clearSelectedFeature();
    expect(usePlaceStore.getState().selectedFeature).toBeNull();
  });
});
