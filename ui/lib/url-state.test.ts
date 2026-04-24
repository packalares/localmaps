import { describe, expect, it } from "vitest";
import {
  formatHash,
  parseHash,
  parseHashOr,
  type MapViewport,
} from "./url-state";

const bucharest: MapViewport = {
  lat: 44.4268,
  lon: 26.1025,
  zoom: 15,
  bearing: 0,
  pitch: 0,
};

describe("url-state", () => {
  it("round-trips a simple viewport", () => {
    const hash = formatHash(bucharest);
    expect(hash).toBe("15.00/44.4268/26.1025");
    expect(parseHash(hash)).toEqual({
      lat: 44.4268,
      lon: 26.1025,
      zoom: 15,
      bearing: 0,
      pitch: 0,
    });
  });

  it("round-trips bearing + pitch when non-zero", () => {
    const rotated: MapViewport = { ...bucharest, bearing: 42.3, pitch: 30 };
    const hash = formatHash(rotated);
    expect(hash).toBe("15.00/44.4268/26.1025/42.3/30.0");
    const parsed = parseHash(hash);
    expect(parsed?.bearing).toBeCloseTo(42.3, 1);
    expect(parsed?.pitch).toBeCloseTo(30, 1);
  });

  it("accepts an optional leading #", () => {
    expect(parseHash("#15/44.4268/26.1025")).not.toBeNull();
  });

  it("returns null for malformed input", () => {
    expect(parseHash("")).toBeNull();
    expect(parseHash("not-a-hash")).toBeNull();
    expect(parseHash("15/44.4268")).toBeNull();
    expect(parseHash("abc/44.4/26.1")).toBeNull();
  });

  it("clamps out-of-range values", () => {
    const parsed = parseHash("25/95/-200/400/120");
    expect(parsed).not.toBeNull();
    expect(parsed!.zoom).toBe(22);
    expect(parsed!.lat).toBe(90);
    expect(parsed!.lon).toBe(-180);
    // bearing is normalised mod 360: 400 -> 40
    expect(parsed!.bearing).toBeCloseTo(40, 1);
    expect(parsed!.pitch).toBe(85);
  });

  it("parseHashOr falls back on garbage", () => {
    expect(parseHashOr("garbage", bucharest)).toEqual(bucharest);
  });

  it("normalises negative bearings", () => {
    const parsed = parseHash("10/0/0/-90/0");
    expect(parsed?.bearing).toBeCloseTo(270, 1);
  });
});
