"use client";

import { useEffect, useRef, useState } from "react";

/**
 * Stable-ref debounce utilities for the search stack.
 *
 * Two flavours:
 *
 * - `useDebouncedValue(value, delayMs)` — React state + effect that
 *   mirrors `value` after it has been stable for `delayMs`. Returns the
 *   latest stable value. Ideal for deriving query strings from a
 *   controlled input.
 *
 * - `useDebouncedCallback(fn, delayMs)` — returns a memo-stable callback
 *   that debounces calls to `fn`. The ref shim keeps `fn` latest-value
 *   so downstream callers don't need to re-subscribe on every render.
 *
 * Both cancel pending timers on unmount.
 */

/** Mirrors `value` after `delayMs` of stability; defaults to 300ms. */
export function useDebouncedValue<T>(value: T, delayMs = 300): T {
  const [debounced, setDebounced] = useState<T>(value);

  useEffect(() => {
    if (delayMs <= 0) {
      setDebounced(value);
      return;
    }
    const handle = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(handle);
  }, [value, delayMs]);

  return debounced;
}

/**
 * Returns a stable-identity callback that debounces invocations of
 * `fn` by `delayMs`. The returned callback forwards the latest arguments
 * and resolves to `undefined` (fire-and-forget).
 *
 * The implementation uses a ref to track `fn` so the returned callback
 * identity stays stable across renders — consumers can list it as a
 * dep without triggering re-debounces.
 */
export function useDebouncedCallback<A extends unknown[]>(
  fn: (...args: A) => void,
  delayMs = 300,
): {
  callback: (...args: A) => void;
  /** Cancels any pending invocation. */
  cancel: () => void;
  /** Flushes any pending invocation immediately (latest args). */
  flush: () => void;
} {
  const fnRef = useRef(fn);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingArgsRef = useRef<A | null>(null);

  // Keep the callback ref in sync with the newest fn on every render so
  // consumers don't need to re-create the debouncer when `fn` changes.
  useEffect(() => {
    fnRef.current = fn;
  }, [fn]);

  useEffect(() => {
    return () => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      pendingArgsRef.current = null;
    };
  }, []);

  const ref = useRef<{
    callback: (...args: A) => void;
    cancel: () => void;
    flush: () => void;
  } | null>(null);

  if (ref.current === null) {
    ref.current = {
      callback: (...args: A) => {
        pendingArgsRef.current = args;
        if (timerRef.current !== null) clearTimeout(timerRef.current);
        if (delayMs <= 0) {
          const a = pendingArgsRef.current;
          pendingArgsRef.current = null;
          if (a) fnRef.current(...a);
          return;
        }
        timerRef.current = setTimeout(() => {
          timerRef.current = null;
          const a = pendingArgsRef.current;
          pendingArgsRef.current = null;
          if (a) fnRef.current(...a);
        }, delayMs);
      },
      cancel: () => {
        if (timerRef.current !== null) {
          clearTimeout(timerRef.current);
          timerRef.current = null;
        }
        pendingArgsRef.current = null;
      },
      flush: () => {
        if (timerRef.current !== null) {
          clearTimeout(timerRef.current);
          timerRef.current = null;
        }
        const a = pendingArgsRef.current;
        pendingArgsRef.current = null;
        if (a) fnRef.current(...a);
      },
    };
  }

  return ref.current;
}
