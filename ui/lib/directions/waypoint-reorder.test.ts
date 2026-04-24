import { describe, expect, it } from "vitest";
import { moveDown, moveUp, reorder } from "./waypoint-reorder";

describe("reorder", () => {
  it("returns a copy when from === to", () => {
    const src = ["a", "b", "c"];
    const out = reorder(src, 1, 1);
    expect(out).toEqual(["a", "b", "c"]);
    expect(out).not.toBe(src);
  });

  it("moves an element forward", () => {
    expect(reorder(["a", "b", "c", "d"], 1, 3)).toEqual(["a", "c", "d", "b"]);
  });

  it("moves an element backward", () => {
    expect(reorder(["a", "b", "c", "d"], 3, 1)).toEqual(["a", "d", "b", "c"]);
  });

  it("clamps a too-high target index", () => {
    expect(reorder(["a", "b", "c"], 0, 99)).toEqual(["b", "c", "a"]);
  });

  it("throws for an invalid source index", () => {
    expect(() => reorder(["a"], 5, 0)).toThrow(/out of bounds/);
  });
});

describe("moveUp / moveDown", () => {
  it("moveUp at index 0 is a no-op copy", () => {
    const src = ["a", "b"];
    const out = moveUp(src, 0);
    expect(out).toEqual(["a", "b"]);
    expect(out).not.toBe(src);
  });

  it("moveDown at last index is a no-op copy", () => {
    const src = ["a", "b"];
    const out = moveDown(src, 1);
    expect(out).toEqual(["a", "b"]);
    expect(out).not.toBe(src);
  });

  it("moveUp swaps with previous", () => {
    expect(moveUp(["a", "b", "c"], 2)).toEqual(["a", "c", "b"]);
  });

  it("moveDown swaps with next", () => {
    expect(moveDown(["a", "b", "c"], 0)).toEqual(["b", "a", "c"]);
  });
});
