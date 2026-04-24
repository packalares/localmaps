"use client";

import { useEffect, useState } from "react";

/**
 * Named viewport bands — spec-locked to match Tailwind's `md` and `lg`
 * breakpoints, which the chrome uses elsewhere:
 *
 *   - `mobile`   : width < 768
 *   - `tablet`   : 768 <= width < 1024
 *   - `desktop`  : 1024 <= width
 *
 * Kept here as constants so tests + sibling modules can reference them
 * without duplicating the number.
 */
export const BREAKPOINT_MOBILE_MAX = 767;
export const BREAKPOINT_TABLET_MIN = 768;
export const BREAKPOINT_TABLET_MAX = 1023;
export const BREAKPOINT_DESKTOP_MIN = 1024;

export type Breakpoint = "mobile" | "tablet" | "desktop";

const MOBILE_QUERY = `(max-width: ${BREAKPOINT_MOBILE_MAX}px)`;
const TABLET_QUERY = `(min-width: ${BREAKPOINT_TABLET_MIN}px) and (max-width: ${BREAKPOINT_TABLET_MAX}px)`;

/**
 * SSR-safe media-query hook. Returns `null` on the server (and during
 * the first hydration tick) so consumers can fall through to a neutral
 * render. Once mounted, returns the current breakpoint band and keeps
 * it in sync with `window.matchMedia` change events.
 *
 * Stable-ref by design: the returned value is `"mobile" | "tablet" |
 * "desktop" | null`, not a new object, so downstream `useEffect`
 * dependency arrays never re-run on every render.
 */
export function useBreakpoint(): Breakpoint | null {
  const [breakpoint, setBreakpoint] = useState<Breakpoint | null>(null);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }
    const mobile = window.matchMedia(MOBILE_QUERY);
    const tablet = window.matchMedia(TABLET_QUERY);

    const compute = (): Breakpoint => {
      if (mobile.matches) return "mobile";
      if (tablet.matches) return "tablet";
      return "desktop";
    };

    setBreakpoint(compute());

    const onChange = () => setBreakpoint(compute());

    // Safari <14 uses the deprecated addListener/removeListener API.
    if (typeof mobile.addEventListener === "function") {
      mobile.addEventListener("change", onChange);
      tablet.addEventListener("change", onChange);
      return () => {
        mobile.removeEventListener("change", onChange);
        tablet.removeEventListener("change", onChange);
      };
    }
    mobile.addListener(onChange);
    tablet.addListener(onChange);
    return () => {
      mobile.removeListener(onChange);
      tablet.removeListener(onChange);
    };
  }, []);

  return breakpoint;
}

/** Convenience predicate used by chrome components that only care about mobile. */
export function useIsMobile(): boolean {
  return useBreakpoint() === "mobile";
}
