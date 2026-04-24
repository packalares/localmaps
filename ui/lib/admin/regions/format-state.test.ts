import { describe, expect, it } from "vitest";
import { formatRegionState } from "./format-state";

describe("formatRegionState", () => {
  it("maps ready to green chip + isReady", () => {
    const d = formatRegionState("ready");
    expect(d.label).toBe("Ready");
    expect(d.isReady).toBe(true);
    expect(d.inProgress).toBe(false);
    expect(d.chipClass).toMatch(/emerald/);
  });

  it("maps failed to red chip + isFailed", () => {
    const d = formatRegionState("failed");
    expect(d.isFailed).toBe(true);
    expect(d.isReady).toBe(false);
    expect(d.chipClass).toMatch(/destructive/);
  });

  it("marks processing_* states as in-progress with amber chip", () => {
    for (const s of [
      "downloading",
      "processing_tiles",
      "processing_routing",
      "processing_geocoding",
      "processing_poi",
    ] as const) {
      const d = formatRegionState(s);
      expect(d.inProgress).toBe(true);
      expect(d.chipClass).toMatch(/amber/);
    }
  });

  it("updating is in-progress but also ready (atomic swap keeps old live)", () => {
    const d = formatRegionState("updating");
    expect(d.inProgress).toBe(true);
    expect(d.isReady).toBe(true);
  });

  it("stage labels match the pipeline copy in the charter", () => {
    expect(formatRegionState("downloading").stage).toBe("Downloading pbf");
    expect(formatRegionState("processing_tiles").stage).toBe("Building tiles");
    expect(formatRegionState("processing_routing").stage).toBe(
      "Building routing",
    );
    expect(formatRegionState("processing_geocoding").stage).toBe(
      "Indexing geocoder",
    );
    expect(formatRegionState("processing_poi").stage).toBe("Fetching POIs");
  });
});
