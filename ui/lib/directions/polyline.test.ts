import { describe, expect, it } from "vitest";
import { boundsFromPoints, decodePolyline } from "./polyline";

describe("decodePolyline", () => {
  it("decodes the canonical precision-5 Google fixture", () => {
    // Classic example from the Google polyline algorithm reference.
    const pts = decodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@", 5);
    expect(pts.length).toBe(3);
    expect(pts[0].lat).toBeCloseTo(38.5, 5);
    expect(pts[0].lng).toBeCloseTo(-120.2, 5);
    expect(pts[1].lat).toBeCloseTo(40.7, 5);
    expect(pts[1].lng).toBeCloseTo(-120.95, 5);
    expect(pts[2].lat).toBeCloseTo(43.252, 5);
    expect(pts[2].lng).toBeCloseTo(-126.453, 5);
  });

  it("decodes precision-6 input (Valhalla default)", () => {
    // (1.0, 2.0) followed by (1.000001, 2.000001) at precision 6.
    const pts = decodePolyline("_c`|@_gayBAA", 6);
    expect(pts.length).toBe(2);
    expect(pts[0].lat).toBeCloseTo(1.0, 6);
    expect(pts[0].lng).toBeCloseTo(2.0, 6);
    expect(pts[1].lat).toBeCloseTo(1.000001, 6);
    expect(pts[1].lng).toBeCloseTo(2.000001, 6);
  });

  it("returns an empty array for an empty input", () => {
    expect(decodePolyline("")).toEqual([]);
  });
});

describe("boundsFromPoints", () => {
  it("returns null for empty input", () => {
    expect(boundsFromPoints([])).toBeNull();
  });

  it("computes min/max in [minLon, minLat, maxLon, maxLat] order", () => {
    const b = boundsFromPoints([
      { lng: 10, lat: 20 },
      { lng: -5, lat: 50 },
      { lng: 30, lat: 0 },
    ]);
    expect(b).not.toBeNull();
    expect(b!.bbox).toEqual([-5, 0, 30, 50]);
  });
});
