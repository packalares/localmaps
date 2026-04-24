import { describe, expect, it } from "vitest";
import { encodeRoute, encodeState, buildShareUrl } from "./encode";
import type { ShareableState, ShareRoute } from "./types";
import type { MapViewport } from "@/lib/url-state";

const viewport: MapViewport = {
  lat: 44.4268,
  lon: 26.1025,
  zoom: 15,
  bearing: 0,
  pitch: 0,
};

describe("encodeState", () => {
  it("emits just the hash for a bare viewport", () => {
    const out = encodeState({ viewport });
    expect(out.hash).toBe("15.00/44.4268/26.1025");
    expect(out.query).toBe("");
    expect(out.overBudget).toBe(false);
  });

  it("adds the region query param when canonical", () => {
    const out = encodeState({ viewport, activeRegion: "europe-romania" });
    expect(out.query).toBe("?r=europe-romania");
  });

  it("drops a non-canonical region (slashes, whitespace)", () => {
    const out = encodeState({ viewport, activeRegion: "europe/romania" });
    expect(out.query).toBe("");
  });

  it("omits the default `search` tab to keep URLs short", () => {
    const out = encodeState({ viewport, leftRailTab: "search" });
    expect(out.query).toBe("");
  });

  it("emits non-default tabs", () => {
    const out = encodeState({ viewport, leftRailTab: "directions" });
    expect(out.query).toContain("tab=directions");
  });

  it("adds poi, q and route when present", () => {
    const state: ShareableState = {
      viewport,
      selectedPoiId: "osm:node:12345",
      searchQuery: "pizza near me",
      route: {
        mode: "auto",
        waypoints: [
          { lng: 26.1025, lat: 44.4268 },
          { lng: 28.0395, lat: 45.6486 },
        ],
        options: {
          avoidHighways: true,
          avoidTolls: true,
          avoidFerries: false,
        },
      },
    };
    const out = encodeState(state);
    expect(out.query).toContain("poi=osm%3Anode%3A12345");
    expect(out.query).toContain("q=pizza+near+me");
    expect(out.query).toContain("route=auto%7C26.1025%2C44.4268%3B28.0395%2C45.6486%7Cht");
  });

  it("is deterministic (same input -> same output)", () => {
    const state: ShareableState = {
      viewport,
      activeRegion: "europe-romania",
      leftRailTab: "directions",
    };
    expect(encodeState(state)).toEqual(encodeState(state));
  });

  it("computes overBudget when the payload exceeds 2048 chars", () => {
    const many: ShareRoute = {
      mode: "auto",
      waypoints: Array.from({ length: 150 }, (_, i) => ({
        lng: -179 + i * 0.12345,
        lat: -89 + i * 0.12345,
      })),
      options: {
        avoidHighways: false,
        avoidTolls: false,
        avoidFerries: false,
      },
    };
    const out = encodeState({ viewport, route: many });
    expect(out.overBudget).toBe(true);
  });

  it("rejects routes with out-of-range coords in encodeRoute", () => {
    expect(
      encodeRoute({
        mode: "auto",
        waypoints: [{ lng: 500, lat: 44 }],
        options: {
          avoidHighways: false,
          avoidTolls: false,
          avoidFerries: false,
        },
      }),
    ).toBeNull();
  });

  it("rejects empty waypoints array", () => {
    expect(
      encodeRoute({
        mode: "auto",
        waypoints: [],
        options: {
          avoidHighways: false,
          avoidTolls: false,
          avoidFerries: false,
        },
      }),
    ).toBeNull();
  });

  it("emits flag combinations correctly (h only, t only, all three, none)", () => {
    const base: ShareRoute = {
      mode: "bicycle",
      waypoints: [{ lng: 1, lat: 2 }],
      options: {
        avoidHighways: false,
        avoidTolls: false,
        avoidFerries: false,
      },
    };
    expect(encodeRoute(base)).toMatch(/\|$/);
    expect(
      encodeRoute({ ...base, options: { ...base.options, avoidHighways: true } }),
    ).toMatch(/\|h$/);
    expect(
      encodeRoute({ ...base, options: { ...base.options, avoidTolls: true } }),
    ).toMatch(/\|t$/);
    expect(
      encodeRoute({
        ...base,
        options: { avoidHighways: true, avoidTolls: true, avoidFerries: true },
      }),
    ).toMatch(/\|htf$/);
  });

  it("buildShareUrl composes origin + path + query + hash correctly", () => {
    const encoded = encodeState({
      viewport,
      activeRegion: "europe-romania",
    });
    expect(buildShareUrl("https://maps.example", "/", encoded)).toBe(
      "https://maps.example/?r=europe-romania#15.00/44.4268/26.1025",
    );
  });

  it("skips empty search query", () => {
    const out = encodeState({ viewport, searchQuery: "   " });
    expect(out.query).toBe("");
  });

  it("skips null route", () => {
    const out = encodeState({ viewport, route: null });
    expect(out.query).toBe("");
  });
});
