import { describe, expect, it } from "vitest";
import {
  formatMeasureArea,
  formatMeasureDistance,
  haversineMetres,
  polygonAreaMetres,
  polylineDistanceMetres,
} from "./geometry";

describe("haversineMetres", () => {
  it("returns 0 for identical points", () => {
    expect(haversineMetres({ lng: 10, lat: 20 }, { lng: 10, lat: 20 })).toBe(0);
  });

  it("matches a known ~111 km per degree of latitude at the equator", () => {
    const d = haversineMetres({ lng: 0, lat: 0 }, { lng: 0, lat: 1 });
    expect(d).toBeGreaterThan(110_000);
    expect(d).toBeLessThan(112_000);
  });

  it("handles antimeridian crossing (179 → -179 ≈ 2°)", () => {
    const near = haversineMetres({ lng: 179, lat: 0 }, { lng: -179, lat: 0 });
    const farSameHemisphere = haversineMetres(
      { lng: -179, lat: 0 },
      { lng: 179, lat: 0 },
    );
    // Both directions should yield the short-way (≈ 2°) distance.
    expect(near).toBeLessThan(300_000);
    expect(farSameHemisphere).toBeCloseTo(near, 0);
  });

  it("Bucharest → Budapest matches a known ~640 km great-circle", () => {
    const d = haversineMetres(
      { lng: 26.1025, lat: 44.4268 },
      { lng: 19.0402, lat: 47.4979 },
    );
    expect(d).toBeGreaterThan(630_000);
    expect(d).toBeLessThan(660_000);
  });
});

describe("polylineDistanceMetres", () => {
  it("returns 0 for fewer than two points", () => {
    expect(polylineDistanceMetres([])).toBe(0);
    expect(polylineDistanceMetres([{ lng: 1, lat: 2 }])).toBe(0);
  });

  it("sums haversine legs across an L-shaped path", () => {
    const pts = [
      { lng: 0, lat: 0 },
      { lng: 0, lat: 1 },
      { lng: 1, lat: 1 },
    ];
    const expected =
      haversineMetres(pts[0], pts[1]) + haversineMetres(pts[1], pts[2]);
    expect(polylineDistanceMetres(pts)).toBeCloseTo(expected, 3);
  });
});

describe("polygonAreaMetres", () => {
  it("returns 0 for degenerate 2-point polygon", () => {
    expect(
      polygonAreaMetres([
        { lng: 0, lat: 0 },
        { lng: 1, lat: 0 },
      ]),
    ).toBe(0);
  });

  it("computes ~12.3 billion m² for a 1°x1° box near the equator", () => {
    const ring = [
      { lng: 0, lat: 0 },
      { lng: 1, lat: 0 },
      { lng: 1, lat: 1 },
      { lng: 0, lat: 1 },
    ];
    const a = polygonAreaMetres(ring);
    // A 1×1 degree square near the equator is ≈ 1.23e10 m² (12,300 km²).
    expect(a).toBeGreaterThan(1.22e10);
    expect(a).toBeLessThan(1.24e10);
  });

  it("is winding-order independent", () => {
    const cw = [
      { lng: 0, lat: 0 },
      { lng: 0, lat: 1 },
      { lng: 1, lat: 1 },
      { lng: 1, lat: 0 },
    ];
    const ccw = [...cw].reverse();
    expect(polygonAreaMetres(cw)).toBeCloseTo(polygonAreaMetres(ccw), 0);
  });
});

describe("formatMeasureDistance", () => {
  it("short metric values show whole metres", () => {
    expect(formatMeasureDistance(42)).toBe("42 m");
  });

  it("≥ 1 km shows dual unit metric/imperial", () => {
    const s = formatMeasureDistance(12_300);
    expect(s).toContain("12.3 km");
    expect(s).toContain("mi");
  });

  it("imperial short values choose feet vs miles", () => {
    expect(formatMeasureDistance(100, "imperial")).toMatch(/ft/);
    expect(formatMeasureDistance(400, "imperial")).toMatch(/mi$/);
  });

  it("invalid input → em-dash", () => {
    expect(formatMeasureDistance(Number.NaN)).toBe("—");
    expect(formatMeasureDistance(-1)).toBe("—");
  });
});

describe("formatMeasureArea", () => {
  it("rolls m² → ha → km² at the right thresholds", () => {
    expect(formatMeasureArea(500)).toBe("500 m²");
    expect(formatMeasureArea(50_000)).toMatch(/ha$/);
    expect(formatMeasureArea(5_000_000)).toMatch(/km²/);
  });

  it("imperial rolls ft² → acres → mi²", () => {
    expect(formatMeasureArea(500, "imperial")).toMatch(/ft²$/);
    expect(formatMeasureArea(50_000, "imperial")).toMatch(/ac$/);
    expect(formatMeasureArea(5_000_000, "imperial")).toMatch(/mi²/);
  });
});
