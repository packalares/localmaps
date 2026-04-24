import { describe, expect, it } from "vitest";
import { parseEmbedSearchParams, parsePinParam } from "./params";

describe("parsePinParam", () => {
  it("parses 'lat,lon' without label", () => {
    expect(parsePinParam("44.43,26.10")).toEqual({ lat: 44.43, lon: 26.1 });
  });

  it("parses 'lat,lon:label'", () => {
    expect(parsePinParam("44.43,26.10:Bucharest")).toEqual({
      lat: 44.43,
      lon: 26.1,
      label: "Bucharest",
    });
  });

  it("preserves colons inside the label", () => {
    expect(parsePinParam("0,0:a:b:c")).toEqual({
      lat: 0,
      lon: 0,
      label: "a:b:c",
    });
  });

  it("rejects malformed coord", () => {
    expect(parsePinParam("garbage")).toBeNull();
    expect(parsePinParam("1")).toBeNull();
    expect(parsePinParam("NaN,10")).toBeNull();
  });

  it("rejects out-of-range coords", () => {
    expect(parsePinParam("91,0")).toBeNull();
    expect(parsePinParam("0,181")).toBeNull();
  });

  it("rejects labels with control characters", () => {
    expect(parsePinParam("0,0:")).toBeNull();
  });

  it("rejects labels over 120 chars", () => {
    expect(parsePinParam(`0,0:${"x".repeat(121)}`)).toBeNull();
  });

  it("returns null on empty/undefined", () => {
    expect(parsePinParam(null)).toBeNull();
    expect(parsePinParam("")).toBeNull();
  });
});

describe("parseEmbedSearchParams", () => {
  it("returns defaults when nothing is supplied", () => {
    const result = parseEmbedSearchParams({});
    expect(result.center).toEqual({ lat: 0, lon: 0 });
    expect(result.zoom).toBe(2);
    expect(result.style).toBe("light");
    expect(result.region).toBeNull();
    expect(result.pin).toBeNull();
  });

  it("accepts valid lat/lon/zoom", () => {
    const result = parseEmbedSearchParams({
      lat: "44.43",
      lon: "26.10",
      zoom: "12",
    });
    expect(result.center).toEqual({ lat: 44.43, lon: 26.1 });
    expect(result.zoom).toBe(12);
  });

  it("ignores half coordinates", () => {
    const result = parseEmbedSearchParams({ lat: "44.43" });
    // lat without lon = both dropped to default.
    expect(result.center).toEqual({ lat: 0, lon: 0 });
  });

  it("clamps out-of-range by dropping to default", () => {
    const result = parseEmbedSearchParams({
      lat: "999",
      lon: "0",
      zoom: "50",
    });
    expect(result.center).toEqual({ lat: 0, lon: 0 });
    expect(result.zoom).toBe(2);
  });

  it("validates style enum", () => {
    expect(parseEmbedSearchParams({ style: "dark" }).style).toBe("dark");
    expect(parseEmbedSearchParams({ style: "neon" }).style).toBe("light");
  });

  it("validates canonical region key", () => {
    expect(parseEmbedSearchParams({ region: "europe-romania" }).region).toBe(
      "europe-romania",
    );
    expect(parseEmbedSearchParams({ region: "Europe/Romania" }).region).toBeNull();
    expect(parseEmbedSearchParams({ region: "EUROPE" }).region).toBeNull();
  });

  it("parses the pin param", () => {
    const result = parseEmbedSearchParams({ pin: "1,2:Hello" });
    expect(result.pin).toEqual({ lat: 1, lon: 2, label: "Hello" });
  });

  it("treats array params by picking the first value", () => {
    const result = parseEmbedSearchParams({
      style: ["dark", "light"],
      lat: ["1", "2"],
      lon: ["3", "4"],
    });
    expect(result.style).toBe("dark");
    expect(result.center).toEqual({ lat: 1, lon: 3 });
  });
});
