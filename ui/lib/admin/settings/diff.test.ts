import { describe, expect, it } from "vitest";
import { deepEqual, diffSettings, flattenTree } from "./diff";

describe("deepEqual", () => {
  it("matches primitives and null", () => {
    expect(deepEqual(1, 1)).toBe(true);
    expect(deepEqual("a", "a")).toBe(true);
    expect(deepEqual(null, null)).toBe(true);
    expect(deepEqual(null, undefined)).toBe(false);
    expect(deepEqual(1, "1")).toBe(false);
  });
  it("matches arrays and objects deeply", () => {
    expect(deepEqual([1, 2, 3], [1, 2, 3])).toBe(true);
    expect(deepEqual([1, 2, 3], [1, 2])).toBe(false);
    expect(deepEqual({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(true);
    expect(deepEqual({ a: 1 }, { a: 2 })).toBe(false);
  });
});

describe("diffSettings", () => {
  it("returns only changed keys", () => {
    const a = { "map.style": "light", "map.maxZoom": 14 };
    const b = { "map.style": "dark", "map.maxZoom": 14 };
    expect(diffSettings(a, b)).toEqual({ "map.style": "dark" });
  });

  it("drops a key once the user reverts it", () => {
    const a = { "map.style": "light" };
    const b = { "map.style": "dark" };
    // Still dirty.
    expect(diffSettings(a, b)).toEqual({ "map.style": "dark" });
    // User reverts.
    expect(diffSettings(a, a)).toEqual({});
  });

  it("picks up new keys and deleted keys", () => {
    const a = { "x": 1 };
    const b = { "y": 2 };
    expect(diffSettings(a, b)).toEqual({ x: undefined, y: 2 });
  });
});

describe("flattenTree", () => {
  it("flattens nested maps into dotted paths", () => {
    const tree = {
      map: { style: "light", maxZoom: 14 },
      routing: { avoidTolls: false },
    };
    expect(flattenTree(tree)).toEqual({
      "map.style": "light",
      "map.maxZoom": 14,
      "routing.avoidTolls": false,
    });
  });

  it("treats map.defaultCenter as a leaf object", () => {
    const tree = {
      map: {
        defaultCenter: { lat: 0, lon: 0, zoom: 2 },
        style: "dark",
      },
    };
    const flat = flattenTree(tree);
    expect(flat["map.defaultCenter"]).toEqual({ lat: 0, lon: 0, zoom: 2 });
    expect(flat["map.style"]).toBe("dark");
  });

  it("keeps array leaves intact", () => {
    expect(flattenTree({ pois: { sources: ["overture", "osm"] } })).toEqual({
      "pois.sources": ["overture", "osm"],
    });
  });
});
