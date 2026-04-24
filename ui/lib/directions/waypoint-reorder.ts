/**
 * Drag-reorder logic for waypoints. Pure function so we can unit-test
 * it without touching the DOM. The UI calls this on drop and on the
 * keyboard "move up" / "move down" actions.
 *
 * Returns a new array; never mutates.
 */

export function reorder<T>(list: readonly T[], from: number, to: number): T[] {
  if (from === to) return [...list];
  if (from < 0 || from >= list.length) {
    throw new RangeError(`from index out of bounds: ${from}`);
  }
  const clampedTo = Math.max(0, Math.min(list.length - 1, to));
  const next = [...list];
  const [moved] = next.splice(from, 1);
  next.splice(clampedTo, 0, moved);
  return next;
}

export function moveUp<T>(list: readonly T[], index: number): T[] {
  if (index <= 0) return [...list];
  return reorder(list, index, index - 1);
}

export function moveDown<T>(list: readonly T[], index: number): T[] {
  if (index >= list.length - 1) return [...list];
  return reorder(list, index, index + 1);
}
