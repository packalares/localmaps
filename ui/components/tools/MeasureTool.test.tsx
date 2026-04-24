import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { useMapStore } from "@/lib/state/map";
import { useActiveToolStore } from "@/lib/tools/active-tool";
import { useMeasureStore } from "@/lib/tools/measure-state";
import { haversineMetres } from "@/lib/tools/geometry";
import { MeasureTool } from "./MeasureTool";

/**
 * Build a minimal LayerBusMap-compatible stub with a real `on`/`off`
 * registry so the tool can attach its dblclick handler.
 */
function makeMap() {
  const sources = new Set<string>();
  const layers = new Set<string>();
  const events = new Map<string, Set<(...args: unknown[]) => void>>();
  return {
    getSource: (id: string) => (sources.has(id) ? { id } : undefined),
    getLayer: (id: string) => (layers.has(id) ? { id } : undefined),
    addSource: vi.fn((id: string) => {
      sources.add(id);
    }),
    addLayer: vi.fn((layer: { id: string }) => {
      layers.add(layer.id);
    }),
    removeSource: vi.fn((id: string) => {
      sources.delete(id);
    }),
    removeLayer: vi.fn((id: string) => {
      layers.delete(id);
    }),
    on: vi.fn((type: string, fn: (...args: unknown[]) => void) => {
      if (!events.has(type)) events.set(type, new Set());
      events.get(type)!.add(fn);
    }),
    off: vi.fn((type: string, fn: (...args: unknown[]) => void) => {
      events.get(type)?.delete(fn);
    }),
    __fire: (type: string) => {
      events.get(type)?.forEach((h) => h());
    },
    __layers: layers,
    __sources: sources,
  };
}

describe("<MeasureTool />", () => {
  beforeEach(() => {
    useActiveToolStore.setState({ active: null });
    useMeasureStore.getState().clear();
    useMapStore.getState().clear();
  });
  afterEach(() => {
    useActiveToolStore.setState({ active: null });
    useMeasureStore.getState().clear();
    useMapStore.getState().clear();
  });

  it("adds a point to the store when a pending click arrives while active", async () => {
    const map = makeMap();
    act(() => {
      useMapStore.setState({
        map: map as unknown as import("maplibre-gl").Map,
      });
      useActiveToolStore.getState().setActive("measure");
    });
    render(<MeasureTool />);

    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 10, lat: 20 },
        point: { x: 0, y: 0 },
        timestamp: Date.now(),
      });
    });

    expect(useMeasureStore.getState().points).toEqual([{ lng: 10, lat: 20 }]);
    // pendingClick should have been consumed.
    expect(useMapStore.getState().pendingClick).toBeNull();
  });

  it("sums haversine legs correctly after multiple clicks", async () => {
    const map = makeMap();
    act(() => {
      useMapStore.setState({
        map: map as unknown as import("maplibre-gl").Map,
      });
      useActiveToolStore.getState().setActive("measure");
    });
    render(<MeasureTool />);

    const a = { lng: 0, lat: 0 };
    const b = { lng: 0, lat: 1 };
    const c = { lng: 1, lat: 1 };

    for (const p of [a, b, c]) {
      act(() => {
        useMapStore.getState().setPendingClick({
          lngLat: p,
          point: { x: 0, y: 0 },
          timestamp: Date.now(),
        });
      });
    }

    const { points } = useMeasureStore.getState();
    expect(points).toHaveLength(3);

    const expected =
      haversineMetres(a, b) + haversineMetres(b, c);
    const sum = (() => {
      let total = 0;
      for (let i = 1; i < points.length; i++) {
        total += haversineMetres(points[i - 1]!, points[i]!);
      }
      return total;
    })();
    expect(sum).toBeCloseTo(expected, 3);
  });

  it("ignores clicks when the tool is not active", async () => {
    const map = makeMap();
    act(() => {
      useMapStore.setState({
        map: map as unknown as import("maplibre-gl").Map,
      });
    });
    render(<MeasureTool />);

    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 1, lat: 1 },
        point: { x: 0, y: 0 },
        timestamp: Date.now(),
      });
    });
    expect(useMeasureStore.getState().points).toEqual([]);
  });

  it("double-click on the map finalises the current measurement", async () => {
    const map = makeMap();
    act(() => {
      useMapStore.setState({
        map: map as unknown as import("maplibre-gl").Map,
      });
      useActiveToolStore.getState().setActive("measure");
    });
    render(<MeasureTool />);

    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 1, lat: 1 },
        point: { x: 0, y: 0 },
        timestamp: Date.now(),
      });
    });
    act(() => {
      useMapStore.getState().setPendingClick({
        lngLat: { lng: 2, lat: 2 },
        point: { x: 0, y: 0 },
        timestamp: Date.now(),
      });
    });
    act(() => {
      map.__fire("dblclick");
    });
    expect(useMeasureStore.getState().isFinalised).toBe(true);
  });
});
