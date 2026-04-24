"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/**
 * Reusable Up/Down/Enter/Escape navigator for cmdk-style single-column
 * result lists. The hook tracks the "highlighted" index independently
 * of focus and dispatches callbacks when the user activates (Enter) or
 * dismisses (Escape).
 *
 * Arrow keys wrap at both ends, matching Google Maps + most command
 * palettes. Home/End jump to first / last.
 */

export interface UseKeyboardNavOptions<T> {
  /** Items currently visible in the list; index positions map 1:1. */
  items: readonly T[];
  /** Called when the user presses Enter on a highlighted item. */
  onSelect: (item: T, index: number) => void;
  /** Called on Escape; consumers typically blur the input and close. */
  onEscape?: () => void;
  /**
   * If true, the hook binds its keyboard listener to the window. Callers
   * with a local container can set `false` and use `handleKeyDown`
   * themselves.
   */
  boundToWindow?: boolean;
}

export interface KeyboardNavController<T> {
  highlightedIndex: number;
  setHighlightedIndex: (index: number) => void;
  /** Explicit handler for callers binding to a local element. */
  handleKeyDown: (event: KeyboardEvent | React.KeyboardEvent) => void;
  /** Invokes onSelect for the currently-highlighted item, if any. */
  activate: () => void;
  /** Returns the item at the current highlight (or undefined). */
  highlightedItem: T | undefined;
}

export function useKeyboardNav<T>(
  opts: UseKeyboardNavOptions<T>,
): KeyboardNavController<T> {
  const { items, onSelect, onEscape, boundToWindow = false } = opts;
  const [highlightedIndex, setHighlightedIndexState] = useState(0);
  const highlightRef = useRef(0);

  const setHighlightedIndex = useCallback((next: number) => {
    highlightRef.current = next;
    setHighlightedIndexState(next);
  }, []);

  // Ref to the latest items/callbacks so we can bind a stable window
  // handler without replaying the listener on every render.
  const stateRef = useRef({ items, onSelect, onEscape });
  stateRef.current = { items, onSelect, onEscape };

  // Clamp whenever the list shrinks / grows. Default to first item.
  useEffect(() => {
    if (items.length === 0) {
      setHighlightedIndex(0);
      return;
    }
    const current = highlightRef.current;
    if (current < 0) setHighlightedIndex(0);
    else if (current >= items.length) setHighlightedIndex(items.length - 1);
  }, [items.length, setHighlightedIndex]);

  const activate = useCallback(() => {
    const { items: list, onSelect: fn } = stateRef.current;
    const idx = highlightRef.current;
    const item = list[idx];
    if (item !== undefined) fn(item, idx);
  }, []);

  const handleKeyDown = useCallback(
    (event: KeyboardEvent | React.KeyboardEvent) => {
      const { items: list } = stateRef.current;
      if (list.length === 0 && event.key !== "Escape") return;

      switch (event.key) {
        case "ArrowDown": {
          event.preventDefault();
          const n = Math.max(list.length, 1);
          setHighlightedIndex((highlightRef.current + 1) % n);
          break;
        }
        case "ArrowUp": {
          event.preventDefault();
          const n = Math.max(list.length, 1);
          setHighlightedIndex((highlightRef.current - 1 + n) % n);
          break;
        }
        case "Home": {
          event.preventDefault();
          setHighlightedIndex(0);
          break;
        }
        case "End": {
          event.preventDefault();
          setHighlightedIndex(list.length - 1);
          break;
        }
        case "Enter": {
          if (list.length === 0) return;
          event.preventDefault();
          activate();
          break;
        }
        case "Escape": {
          event.preventDefault();
          stateRef.current.onEscape?.();
          break;
        }
      }
    },
    [activate, setHighlightedIndex],
  );

  useEffect(() => {
    if (!boundToWindow) return;
    const listener = (event: KeyboardEvent) => handleKeyDown(event);
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, [boundToWindow, handleKeyDown]);

  return {
    highlightedIndex,
    setHighlightedIndex,
    handleKeyDown,
    activate,
    highlightedItem: items[highlightedIndex],
  };
}
