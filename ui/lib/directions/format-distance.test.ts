import { describe, expect, it } from "vitest";
import { formatDistance } from "./format-distance";

describe("formatDistance — metric", () => {
  it("renders em-dash for invalid input", () => {
    expect(formatDistance(Number.NaN)).toBe("—");
    expect(formatDistance(-10)).toBe("—");
  });

  it("meters under 100 get nearest-metre precision", () => {
    expect(formatDistance(12)).toBe("12 m");
    expect(formatDistance(99)).toBe("99 m");
  });

  it("meters under 1000 get rounded to nearest 10", () => {
    expect(formatDistance(101)).toBe("100 m");
    expect(formatDistance(155)).toBe("160 m");
    expect(formatDistance(999)).toBe("1000 m");
  });

  it("kilometres below 10 get one decimal", () => {
    expect(formatDistance(1234)).toBe("1.2 km");
    expect(formatDistance(9999)).toBe("10.0 km");
  });

  it("kilometres at or above 10 are whole numbers", () => {
    expect(formatDistance(15600)).toBe("16 km");
  });
});

describe("formatDistance — imperial", () => {
  it("under 0.1 mi renders feet", () => {
    expect(formatDistance(30, { units: "imperial" })).toBe("98 ft");
  });

  it("miles below 10 get one decimal", () => {
    expect(formatDistance(5000, { units: "imperial" })).toBe("3.1 mi");
  });

  it("miles at or above 10 are whole", () => {
    expect(formatDistance(32000, { units: "imperial" })).toBe("20 mi");
  });
});
