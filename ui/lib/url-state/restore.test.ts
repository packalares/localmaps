import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { applyDecoded, useRestoreOnMount } from "./restore";
import { useMapStore, DEFAULT_VIEWPORT } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import {
  useDirectionsStore,
  DEFAULT_OPTIONS,
  DEFAULT_WAYPOINTS,
} from "@/lib/state/directions";

function resetStores() {
  useMapStore.getState().clear();
  usePlaceStore.getState().clearSelectedFeature();
  useDirectionsStore.setState({
    waypoints: DEFAULT_WAYPOINTS(),
    mode: "auto",
    options: DEFAULT_OPTIONS,
    route: null,
    alternatives: [],
  });
}

function setUrl(path: string) {
  window.history.replaceState(null, "", path);
}

describe("applyDecoded", () => {
  beforeEach(() => resetStores());

  it("does nothing on an empty decode", () => {
    applyDecoded({});
    expect(useMapStore.getState().viewport).toEqual(DEFAULT_VIEWPORT);
    expect(useMapStore.getState().activeRegion).toBeNull();
    expect(useMapStore.getState().leftRailTab).toBe("search");
  });

  it("applies viewport, region, tab, poi id", () => {
    applyDecoded({
      viewport: { lat: 44, lon: 26, zoom: 10, bearing: 0, pitch: 0 },
      activeRegion: "europe-romania",
      leftRailTab: "saved",
      selectedPoiId: "osm:node:1",
    });
    const s = useMapStore.getState();
    expect(s.viewport.lat).toBe(44);
    expect(s.activeRegion).toBe("europe-romania");
    expect(s.leftRailTab).toBe("saved");
    // POI id is stamped onto the place store (canonical surface for
    // the bottom info card) — not the deleted `selectedPoi` slice.
    expect(usePlaceStore.getState().selectedFeature?.id).toBe("osm:node:1");
  });

  it("applies directions route + waypoints + options + mode", () => {
    applyDecoded({
      route: {
        mode: "bicycle",
        waypoints: [
          { lng: 26.1, lat: 44.43 },
          { lng: 28.04, lat: 45.65 },
        ],
        options: {
          avoidHighways: true,
          avoidTolls: false,
          avoidFerries: true,
        },
      },
    });
    const d = useDirectionsStore.getState();
    expect(d.mode).toBe("bicycle");
    expect(d.options).toEqual({
      avoidHighways: true,
      avoidTolls: false,
      avoidFerries: true,
    });
    expect(d.waypoints).toHaveLength(2);
    expect(d.waypoints[0].lngLat).toEqual({ lng: 26.1, lat: 44.43 });
    expect(d.waypoints[1].lngLat).toEqual({ lng: 28.04, lat: 45.65 });
  });

  it("pads a single-waypoint decode to the 2-waypoint UI shape", () => {
    applyDecoded({
      route: {
        mode: "auto",
        waypoints: [{ lng: 1, lat: 2 }],
        options: {
          avoidHighways: false,
          avoidTolls: false,
          avoidFerries: false,
        },
      },
    });
    const d = useDirectionsStore.getState();
    expect(d.waypoints).toHaveLength(2);
    expect(d.waypoints[0].lngLat).toEqual({ lng: 1, lat: 2 });
    expect(d.waypoints[1].lngLat).toBeNull();
  });
});

describe("useRestoreOnMount", () => {
  beforeEach(() => resetStores());
  afterEach(() => setUrl("/"));

  it("reads window.location once on mount", () => {
    setUrl("/?r=europe-romania&tab=saved#12.00/44.4/26.1");
    renderHook(() => useRestoreOnMount());
    const s = useMapStore.getState();
    expect(s.activeRegion).toBe("europe-romania");
    expect(s.leftRailTab).toBe("saved");
    expect(s.viewport.zoom).toBeCloseTo(12, 1);
  });

  it("is idempotent: re-renders do not reapply", () => {
    setUrl("/?tab=saved#2/0/0");
    const { rerender } = renderHook(() => useRestoreOnMount());
    // Reset the store mid-flight; a second effect tick should not clobber.
    useMapStore.getState().openLeftRail("search");
    rerender();
    expect(useMapStore.getState().leftRailTab).toBe("search");
  });

  it("is a no-op on an empty URL", () => {
    setUrl("/");
    renderHook(() => useRestoreOnMount());
    expect(useMapStore.getState().activeRegion).toBeNull();
  });
});
