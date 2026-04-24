import { afterEach, describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useUrlViewport } from "./use-url-viewport";
import type { MapViewport } from "@/lib/url-state";

const fallback: MapViewport = {
  lat: 0,
  lon: 0,
  zoom: 2,
  bearing: 0,
  pitch: 0,
};

function setUrl(path: string) {
  window.history.replaceState(null, "", path);
}

describe("useUrlViewport", () => {
  afterEach(() => setUrl("/"));

  it("parses hash and returns fallback region when `r` is absent", () => {
    setUrl("/#15.00/44.4268/26.1025");
    const { result } = renderHook(() => useUrlViewport(fallback));
    expect(result.current.initial).toEqual({
      viewport: {
        lat: 44.4268,
        lon: 26.1025,
        zoom: 15,
        bearing: 0,
        pitch: 0,
      },
      region: null,
    });
  });

  it("parses the region query param (canonical hyphen-form)", () => {
    setUrl("/?r=europe-romania#14.00/45.75/21.22");
    const { result } = renderHook(() => useUrlViewport(fallback));
    expect(result.current.initial?.region).toBe("europe-romania");
    expect(result.current.initial?.viewport.zoom).toBe(14);
  });

  it("ignores invalid region keys (slashes, empty, whitespace)", () => {
    setUrl("/?r=europe/romania#10.00/45/21");
    const { result } = renderHook(() => useUrlViewport(fallback));
    expect(result.current.initial?.region).toBeNull();
  });

  it("commit writes hash AND preserves search params", () => {
    setUrl("/?foo=bar");
    const { result } = renderHook(() => useUrlViewport(fallback));
    act(() => {
      result.current.commit({
        viewport: { lat: 10, lon: 20, zoom: 4, bearing: 0, pitch: 0 },
        region: "europe-romania",
      });
    });
    expect(window.location.search).toContain("foo=bar");
    expect(window.location.search).toContain("r=europe-romania");
    expect(window.location.hash).toBe("#4.00/10.0000/20.0000");
  });

  it("commit removes the region param when set to null", () => {
    setUrl("/?r=europe-romania");
    const { result } = renderHook(() => useUrlViewport(fallback));
    act(() => {
      result.current.commit({
        viewport: fallback,
        region: null,
      });
    });
    expect(window.location.search).not.toContain("r=");
  });

  it("round-trips viewport + region", () => {
    setUrl("/?r=af-rwanda#12.34/1.9403/29.8739/45.0/30.0");
    const { result } = renderHook(() => useUrlViewport(fallback));
    const round = result.current.initial;
    expect(round?.region).toBe("af-rwanda");
    expect(round?.viewport.zoom).toBeCloseTo(12.34, 2);
    expect(round?.viewport.lat).toBeCloseTo(1.9403, 3);
    expect(round?.viewport.bearing).toBeCloseTo(45, 1);
    expect(round?.viewport.pitch).toBeCloseTo(30, 1);
  });
});
