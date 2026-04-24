import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { routeToGpx } from "./gpx";

const golden = readFileSync(
  join(__dirname, "testdata", "simple.gpx"),
  "utf-8",
);

describe("routeToGpx", () => {
  it("matches the golden fixture for a canned input", () => {
    const out = routeToGpx({
      routeId: "abc-123",
      polylines: ["_c`|@_gayBAA"],
      waypoints: [
        { lat: 1.0, lon: 2.0, name: "Start" },
        { lat: 1.000001, lon: 2.000001, name: "End" },
      ],
      appVersion: "0.1.0-test",
      nowIso: "2026-01-01T00:00:00.000Z",
    });
    expect(out).toBe(golden);
  });

  it("renders a well-formed XML prologue", () => {
    const out = routeToGpx({
      routeId: "x",
      polylines: [""],
      nowIso: "2026-01-01T00:00:00.000Z",
    });
    expect(out.startsWith('<?xml version="1.0" encoding="UTF-8"?>\n')).toBe(
      true,
    );
    expect(out).toMatch(/creator="LocalMaps dev"/);
  });

  it("escapes special characters in names", () => {
    const out = routeToGpx({
      routeId: "x",
      polylines: [],
      trackName: 'A&B "quote" <x>',
      nowIso: "2026-01-01T00:00:00.000Z",
    });
    expect(out).toContain("A&amp;B &quot;quote&quot; &lt;x&gt;");
  });
});
