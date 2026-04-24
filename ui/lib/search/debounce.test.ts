import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useDebouncedCallback, useDebouncedValue } from "./debounce";

describe("useDebouncedValue", () => {
  it("mirrors the initial value on mount", () => {
    const { result } = renderHook(() => useDebouncedValue("initial", 200));
    expect(result.current).toBe("initial");
  });

  it("waits for stability before updating", async () => {
    vi.useFakeTimers();
    try {
      const { result, rerender } = renderHook(
        ({ v }) => useDebouncedValue(v, 100),
        { initialProps: { v: "a" } },
      );

      rerender({ v: "b" });
      rerender({ v: "c" });

      // Not yet.
      expect(result.current).toBe("a");

      act(() => {
        vi.advanceTimersByTime(99);
      });
      expect(result.current).toBe("a");

      act(() => {
        vi.advanceTimersByTime(1);
      });
      expect(result.current).toBe("c");
    } finally {
      vi.useRealTimers();
    }
  });

  it("uses zero delay synchronously", () => {
    const { result, rerender } = renderHook(
      ({ v }) => useDebouncedValue(v, 0),
      { initialProps: { v: "a" } },
    );
    rerender({ v: "b" });
    expect(result.current).toBe("b");
  });
});

describe("useDebouncedCallback", () => {
  it("keeps a stable callback identity across renders", () => {
    const { result, rerender } = renderHook(
      ({ fn }) => useDebouncedCallback(fn, 100),
      { initialProps: { fn: () => {} } },
    );
    const first = result.current.callback;
    rerender({ fn: () => {} });
    expect(result.current.callback).toBe(first);
  });

  it("invokes the latest fn with the latest args after the delay", () => {
    vi.useFakeTimers();
    try {
      const seen: string[] = [];
      const fn = vi.fn((x: string) => {
        seen.push(x);
      });
      const { result } = renderHook(() => useDebouncedCallback(fn, 50));

      act(() => {
        result.current.callback("a");
        result.current.callback("b");
        result.current.callback("c");
      });

      expect(fn).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(50);
      });

      expect(fn).toHaveBeenCalledTimes(1);
      expect(seen).toEqual(["c"]);
    } finally {
      vi.useRealTimers();
    }
  });

  it("cancel() drops pending invocations", () => {
    vi.useFakeTimers();
    try {
      const fn = vi.fn();
      const { result } = renderHook(() => useDebouncedCallback(fn, 50));
      act(() => {
        result.current.callback();
        result.current.cancel();
        vi.advanceTimersByTime(100);
      });
      expect(fn).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it("flush() fires immediately with the latest args", () => {
    vi.useFakeTimers();
    try {
      const fn = vi.fn();
      const { result } = renderHook(() => useDebouncedCallback(fn, 50));
      act(() => {
        result.current.callback("x");
        result.current.flush();
      });
      expect(fn).toHaveBeenCalledTimes(1);
      expect(fn).toHaveBeenCalledWith("x");
    } finally {
      vi.useRealTimers();
    }
  });

  it("latest-fn semantics: later fn value wins without re-debouncing", () => {
    vi.useFakeTimers();
    try {
      const first = vi.fn();
      const second = vi.fn();
      const { result, rerender } = renderHook(
        ({ fn }) => useDebouncedCallback(fn, 50),
        { initialProps: { fn: first } },
      );

      act(() => {
        result.current.callback("q");
      });
      // Swap fn mid-flight; the fire should use the latest.
      rerender({ fn: second });
      act(() => {
        vi.advanceTimersByTime(50);
      });
      expect(first).not.toHaveBeenCalled();
      expect(second).toHaveBeenCalledWith("q");
    } finally {
      vi.useRealTimers();
    }
  });
});
