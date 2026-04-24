import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { routeToKml } from "./kml";

const golden = readFileSync(
  join(__dirname, "testdata", "simple.kml"),
  "utf-8",
);

describe("routeToKml", () => {
  it("matches the golden fixture for a canned input", () => {
    const out = routeToKml({
      routeId: "abc-123",
      polylines: ["_c`|@_gayBAA"],
      waypoints: [
        { lat: 1.0, lon: 2.0, name: "Start" },
        { lat: 1.000001, lon: 2.000001, name: "End" },
      ],
      appVersion: "0.1.0-test",
    });
    expect(out).toBe(golden);
  });

  it("declares the correct KML namespace", () => {
    const out = routeToKml({
      routeId: "x",
      polylines: [],
    });
    expect(out).toMatch(/xmlns="http:\/\/www\.opengis\.net\/kml\/2\.2"/);
  });
});
