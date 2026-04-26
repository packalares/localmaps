import { describe, expect, it } from "vitest";
import { decodeRoute, decodeURL } from "./decode";
import { encodeState, buildShareUrl } from "./encode";
import type { ShareableState } from "./types";
import type { MapViewport } from "@/lib/url-state";

const bucharest: MapViewport = {
  lat: 44.4268,
  lon: 26.1025,
  zoom: 15,
  bearing: 0,
  pitch: 0,
};

function absolute(path: string): string {
  return `https://maps.example${path}`;
}

describe("decodeURL", () => {
  it("decodes legacy Phase-3 URL (hash only)", () => {
    const d = decodeURL(absolute("/#15.00/44.4268/26.1025"));
    expect(d).not.toBeNull();
    expect(d?.viewport).toEqual(bucharest);
    expect(d?.activeRegion).toBeUndefined();
    expect(d?.route).toBeUndefined();
  });

  it("decodes hash with bearing + pitch", () => {
    const d = decodeURL(absolute("/#12.00/1.94/29.87/45.0/30.0"));
    expect(d?.viewport?.bearing).toBeCloseTo(45, 1);
    expect(d?.viewport?.pitch).toBeCloseTo(30, 1);
  });

  it("decodes the full query set", () => {
    const url = absolute(
      "/?r=europe-romania&tab=directions&poi=osm%3An%3A1&q=pizza&route=auto%7C26.10%2C44.43%3B28.04%2C45.65%7Cht#15/44/26",
    );
    const d = decodeURL(url)!;
    expect(d.activeRegion).toBe("europe-romania");
    expect(d.leftRailTab).toBe("directions");
    expect(d.selectedPoiId).toBe("osm:n:1");
    expect(d.searchQuery).toBe("pizza");
    expect(d.route?.mode).toBe("auto");
    expect(d.route?.waypoints).toEqual([
      { lng: 26.1, lat: 44.43 },
      { lng: 28.04, lat: 45.65 },
    ]);
    expect(d.route?.options).toEqual({
      avoidHighways: true,
      avoidTolls: true,
      avoidFerries: false,
    });
  });

  it("ignores unknown tab / invalid region", () => {
    const d = decodeURL(absolute("/?r=bad/slash&tab=nope#2/0/0"))!;
    expect(d.activeRegion).toBeUndefined();
    expect(d.leftRailTab).toBeUndefined();
  });

  it("returns an empty object for a bare path", () => {
    const d = decodeURL(absolute("/"));
    expect(d).toEqual({});
  });

  it("returns null for unparsable input", () => {
    expect(decodeURL("::::")).toBeNull();
  });

  it("accepts relative URL strings", () => {
    const d = decodeURL("/?tab=saved#2/0/0");
    expect(d?.leftRailTab).toBe("saved");
  });

  it("accepts a URL object", () => {
    const d = decodeURL(new URL("https://maps.example/?q=hello"));
    expect(d?.searchQuery).toBe("hello");
  });

  it("drops route with malformed coord", () => {
    expect(decodeRoute("auto|abc,44.43|")).toBeNull();
    expect(decodeRoute("auto|200,44|")).toBeNull();
    expect(decodeRoute("wat|1,2|")).toBeNull();
    expect(decodeRoute("")).toBeNull();
    expect(decodeRoute("auto||")).toBeNull();
  });

  it("decodes a single-waypoint route (route is being built)", () => {
    const r = decodeRoute("bicycle|5.12,45.77|f")!;
    expect(r.mode).toBe("bicycle");
    expect(r.waypoints).toHaveLength(1);
    expect(r.options.avoidFerries).toBe(true);
  });

  it("decodes the share-button's ?lat=&lon=&zoom=&place= shape (F2)", () => {
    const d = decodeURL(
      absolute("/?lat=44.4268&lon=26.1025&zoom=14&place=osm%3Anode%3A1"),
    )!;
    expect(d.viewport?.lat).toBeCloseTo(44.4268, 4);
    expect(d.viewport?.lon).toBeCloseTo(26.1025, 4);
    expect(d.viewport?.zoom).toBeCloseTo(14, 4);
    expect(d.selectedPoiId).toBe("osm:node:1");
  });

  it("share-button hash precedence: hash beats ?lat=&lon=", () => {
    const d = decodeURL(
      absolute("/?lat=10&lon=10&zoom=10#15.00/44.4268/26.1025"),
    )!;
    expect(d.viewport?.lat).toBeCloseTo(44.4268, 3);
  });

  it("rejects out-of-range share-button coords", () => {
    expect(decodeURL(absolute("/?lat=999&lon=10&zoom=15"))?.viewport).toBeUndefined();
  });
});

describe("round-trip encode <-> decode", () => {
  const cases: ShareableState[] = [
    { viewport: bucharest },
    { viewport: bucharest, activeRegion: "europe-romania" },
    { viewport: bucharest, leftRailTab: "saved" },
    { viewport: bucharest, leftRailTab: "saved", selectedPoiId: "osm:node:42" },
    { viewport: bucharest, searchQuery: "coffee & pastries" },
    {
      viewport: { ...bucharest, bearing: 42.3, pitch: 30 },
      activeRegion: "europe-romania",
    },
    {
      viewport: bucharest,
      route: {
        mode: "auto",
        waypoints: [
          { lng: 26.1025, lat: 44.4268 },
          { lng: 28.0395, lat: 45.6486 },
        ],
        options: {
          avoidHighways: false,
          avoidTolls: false,
          avoidFerries: false,
        },
      },
    },
    {
      viewport: bucharest,
      route: {
        mode: "pedestrian",
        waypoints: [
          { lng: 2.3522, lat: 48.8566 },
          { lng: 4.3517, lat: 50.8503 },
          { lng: 4.9041, lat: 52.3676 },
        ],
        options: {
          avoidHighways: true,
          avoidTolls: false,
          avoidFerries: true,
        },
      },
    },
    {
      viewport: bucharest,
      activeRegion: "europe-romania",
      leftRailTab: "directions",
      searchQuery: "home",
      route: {
        mode: "truck",
        waypoints: [{ lng: 10, lat: 20 }],
        options: {
          avoidHighways: true,
          avoidTolls: true,
          avoidFerries: true,
        },
      },
    },
    {
      viewport: { lat: 0, lon: 0, zoom: 2, bearing: 0, pitch: 0 },
      leftRailTab: "recents",
    },
    {
      viewport: { lat: -33.8688, lon: 151.2093, zoom: 13, bearing: 90, pitch: 60 },
      activeRegion: "australia-oceania-australia",
    },
  ];

  it.each(cases)("round-trips case #%#", (state) => {
    const encoded = encodeState(state);
    const url = buildShareUrl("https://maps.example", "/", encoded);
    const decoded = decodeURL(url)!;

    // Viewport round-trips at the precision of formatHash.
    expect(decoded.viewport?.zoom).toBeCloseTo(state.viewport.zoom, 1);
    expect(decoded.viewport?.lat).toBeCloseTo(state.viewport.lat, 3);
    expect(decoded.viewport?.lon).toBeCloseTo(state.viewport.lon, 3);

    if (state.activeRegion) expect(decoded.activeRegion).toBe(state.activeRegion);
    if (state.leftRailTab && state.leftRailTab !== "search") {
      expect(decoded.leftRailTab).toBe(state.leftRailTab);
    }
    if (state.selectedPoiId) expect(decoded.selectedPoiId).toBe(state.selectedPoiId);
    if (state.searchQuery) expect(decoded.searchQuery).toBe(state.searchQuery);
    if (state.route) {
      expect(decoded.route?.mode).toBe(state.route.mode);
      expect(decoded.route?.waypoints.length).toBe(state.route.waypoints.length);
      expect(decoded.route?.options).toEqual(state.route.options);
      decoded.route?.waypoints.forEach((wp, i) => {
        expect(wp.lng).toBeCloseTo(state.route!.waypoints[i].lng, 3);
        expect(wp.lat).toBeCloseTo(state.route!.waypoints[i].lat, 3);
      });
    }
  });
});

describe("size budget", () => {
  const noFlags = {
    avoidHighways: false,
    avoidTolls: false,
    avoidFerries: false,
  };

  it("normal state stays under 2000 chars", () => {
    const out = encodeState({
      viewport: bucharest,
      activeRegion: "europe-romania",
      leftRailTab: "directions",
      searchQuery: "coffee",
      route: {
        mode: "auto",
        waypoints: [
          { lng: 26.1, lat: 44.43 },
          { lng: 28.04, lat: 45.65 },
        ],
        options: { avoidHighways: true, avoidTolls: true, avoidFerries: false },
      },
    });
    expect(out.length).toBeLessThan(2000);
    expect(out.overBudget).toBe(false);
  });

  it("20 waypoints fits but 150 overflows the 2048-char budget", () => {
    const wps = (n: number) =>
      Array.from({ length: n }, (_, i) => ({
        lng: -179 + i * 0.12345,
        lat: -89 + i * 0.12345,
      }));
    const twenty = encodeState({
      viewport: bucharest,
      route: { mode: "auto", waypoints: wps(20), options: noFlags },
    });
    expect(twenty.length).toBeLessThan(2048);
    expect(twenty.overBudget).toBe(false);

    const bigger = encodeState({
      viewport: bucharest,
      route: { mode: "auto", waypoints: wps(150), options: noFlags },
    });
    expect(bigger.length).toBeGreaterThan(2048);
    expect(bigger.overBudget).toBe(true);
  });
});
