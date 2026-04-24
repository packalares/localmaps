import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useKeyboardNav } from "./keyboard-nav";

function keyEvent(key: string): KeyboardEvent {
  return new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true });
}

describe("useKeyboardNav", () => {
  it("defaults highlight to the first item", () => {
    const items = ["a", "b", "c"];
    const { result } = renderHook(() =>
      useKeyboardNav({ items, onSelect: () => {} }),
    );
    expect(result.current.highlightedIndex).toBe(0);
    expect(result.current.highlightedItem).toBe("a");
  });

  it("ArrowDown advances and wraps at the end", () => {
    const items = ["a", "b", "c"];
    const { result } = renderHook(() =>
      useKeyboardNav({ items, onSelect: () => {} }),
    );
    act(() => result.current.handleKeyDown(keyEvent("ArrowDown")));
    expect(result.current.highlightedIndex).toBe(1);
    act(() => result.current.handleKeyDown(keyEvent("ArrowDown")));
    expect(result.current.highlightedIndex).toBe(2);
    act(() => result.current.handleKeyDown(keyEvent("ArrowDown")));
    expect(result.current.highlightedIndex).toBe(0);
  });

  it("ArrowUp retreats and wraps at the start", () => {
    const items = ["a", "b", "c"];
    const { result } = renderHook(() =>
      useKeyboardNav({ items, onSelect: () => {} }),
    );
    act(() => result.current.handleKeyDown(keyEvent("ArrowUp")));
    expect(result.current.highlightedIndex).toBe(2);
  });

  it("Home/End jump to edges", () => {
    const items = ["a", "b", "c", "d"];
    const { result } = renderHook(() =>
      useKeyboardNav({ items, onSelect: () => {} }),
    );
    act(() => result.current.handleKeyDown(keyEvent("End")));
    expect(result.current.highlightedIndex).toBe(3);
    act(() => result.current.handleKeyDown(keyEvent("Home")));
    expect(result.current.highlightedIndex).toBe(0);
  });

  it("Enter invokes onSelect with the current item + index", () => {
    const onSelect = vi.fn();
    const items = ["a", "b", "c"];
    const { result } = renderHook(() => useKeyboardNav({ items, onSelect }));
    act(() => result.current.handleKeyDown(keyEvent("ArrowDown")));
    act(() => result.current.handleKeyDown(keyEvent("Enter")));
    expect(onSelect).toHaveBeenCalledWith("b", 1);
  });

  it("Escape calls onEscape", () => {
    const onEscape = vi.fn();
    const { result } = renderHook(() =>
      useKeyboardNav({ items: ["a"], onSelect: () => {}, onEscape }),
    );
    act(() => result.current.handleKeyDown(keyEvent("Escape")));
    expect(onEscape).toHaveBeenCalledTimes(1);
  });

  it("Ignores arrow keys with an empty list but still handles Escape", () => {
    const onEscape = vi.fn();
    const onSelect = vi.fn();
    const { result } = renderHook(() =>
      useKeyboardNav({ items: [], onSelect, onEscape }),
    );
    act(() => result.current.handleKeyDown(keyEvent("ArrowDown")));
    expect(result.current.highlightedIndex).toBe(0);
    expect(onSelect).not.toHaveBeenCalled();
    act(() => result.current.handleKeyDown(keyEvent("Escape")));
    expect(onEscape).toHaveBeenCalledTimes(1);
  });

  it("clamps highlight when the list shrinks", () => {
    const { result, rerender } = renderHook(
      ({ items }) => useKeyboardNav({ items, onSelect: () => {} }),
      { initialProps: { items: ["a", "b", "c"] as readonly string[] } },
    );
    act(() => result.current.handleKeyDown(keyEvent("End")));
    expect(result.current.highlightedIndex).toBe(2);
    rerender({ items: ["a"] });
    expect(result.current.highlightedIndex).toBe(0);
    expect(result.current.highlightedItem).toBe("a");
  });

  it("window binding fires the same handler", () => {
    const onSelect = vi.fn();
    renderHook(() =>
      useKeyboardNav({
        items: ["a", "b"],
        onSelect,
        boundToWindow: true,
      }),
    );
    act(() => {
      window.dispatchEvent(keyEvent("ArrowDown"));
      window.dispatchEvent(keyEvent("Enter"));
    });
    expect(onSelect).toHaveBeenCalledWith("b", 1);
  });
});
