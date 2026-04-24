import { describe, expect, it } from "vitest";
import {
  fromCanonicalRegionKey,
  isCanonicalRegionKey,
  toCanonicalRegionKey,
} from "./region-key";

describe("region-key", () => {
  it("slash-form → canonical hyphen-form", () => {
    expect(toCanonicalRegionKey("europe/romania")).toBe("europe-romania");
    expect(toCanonicalRegionKey("EUROPE/Germany/Berlin")).toBe(
      "europe-germany-berlin",
    );
  });

  it("canonical hyphen-form → slash-form", () => {
    expect(fromCanonicalRegionKey("europe-romania")).toBe("europe/romania");
  });

  it("canonical-key validator accepts the happy path", () => {
    expect(isCanonicalRegionKey("europe-romania")).toBe(true);
    expect(isCanonicalRegionKey("us")).toBe(true);
    expect(isCanonicalRegionKey("af-rwanda")).toBe(true);
  });

  it("canonical-key validator rejects spaces, slashes, and empty", () => {
    expect(isCanonicalRegionKey("europe/romania")).toBe(false);
    expect(isCanonicalRegionKey("")).toBe(false);
    expect(isCanonicalRegionKey(" europe-romania ")).toBe(false);
    expect(isCanonicalRegionKey(null)).toBe(false);
    expect(isCanonicalRegionKey(42)).toBe(false);
  });
});
