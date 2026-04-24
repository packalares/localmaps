import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useBreakpoint, useIsMobile } from "./use-breakpoint";

/**
 * matchMedia is shimmed in `vitest.setup.ts` as a no-op that always
 * returns `matches: false`. For the breakpoint hook we need a shim
 * that honours the query string against a configurable `innerWidth` and
 * dispatches change events when the width moves across a threshold.
 */

interface Listener {
  query: string;
  cb: () => void;
}

let currentWidth = 1440;
let listeners: Listener[] = [];

function matches(query: string, width: number): boolean {
  const maxMatch = /\(max-width:\s*(\d+)px\)/.exec(query);
  const minMatch = /\(min-width:\s*(\d+)px\)/.exec(query);
  const min = minMatch ? parseInt(minMatch[1], 10) : 0;
  const max = maxMatch ? parseInt(maxMatch[1], 10) : Infinity;
  return width >= min && width <= max;
}

function makeMql(query: string): MediaQueryList {
  const mql = {
    get matches() {
      return matches(query, currentWidth);
    },
    media: query,
    onchange: null,
    addEventListener: (_: string, cb: () => void) => {
      listeners.push({ query, cb });
    },
    removeEventListener: (_: string, cb: () => void) => {
      listeners = listeners.filter((l) => l.cb !== cb);
    },
    addListener: (cb: () => void) => listeners.push({ query, cb }),
    removeListener: (cb: () => void) => {
      listeners = listeners.filter((l) => l.cb !== cb);
    },
    dispatchEvent: () => false,
  } as unknown as MediaQueryList;
  return mql;
}

function setWidth(width: number) {
  currentWidth = width;
  act(() => {
    // Fire a change for every registered listener; the hook recomputes.
    for (const l of listeners) l.cb();
  });
}

describe("useBreakpoint", () => {
  beforeEach(() => {
    currentWidth = 1440;
    listeners = [];
    (window as unknown as { matchMedia: (q: string) => MediaQueryList }).matchMedia =
      (query: string) => makeMql(query);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("starts null on the server tick and resolves to desktop at 1440px", () => {
    const { result } = renderHook(() => useBreakpoint());
    expect(result.current).toBe("desktop");
  });

  it("reports 'mobile' below 768", () => {
    currentWidth = 480;
    const { result } = renderHook(() => useBreakpoint());
    expect(result.current).toBe("mobile");
  });

  it("reports 'tablet' between 768 and 1023", () => {
    currentWidth = 900;
    const { result } = renderHook(() => useBreakpoint());
    expect(result.current).toBe("tablet");
  });

  it("transitions from desktop → tablet → mobile as the viewport narrows", () => {
    currentWidth = 1440;
    const { result } = renderHook(() => useBreakpoint());
    expect(result.current).toBe("desktop");
    setWidth(900);
    expect(result.current).toBe("tablet");
    setWidth(480);
    expect(result.current).toBe("mobile");
    setWidth(1440);
    expect(result.current).toBe("desktop");
  });
});

describe("useIsMobile", () => {
  beforeEach(() => {
    listeners = [];
    (window as unknown as { matchMedia: (q: string) => MediaQueryList }).matchMedia =
      (query: string) => makeMql(query);
  });

  it("is true below 768 and false above", () => {
    currentWidth = 400;
    const { result, rerender } = renderHook(() => useIsMobile());
    expect(result.current).toBe(true);
    setWidth(1200);
    rerender();
    expect(result.current).toBe(false);
  });
});
