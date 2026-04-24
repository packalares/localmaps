import { describe, expect, it } from "vitest";
import type { SettingsSchemaNode } from "@/lib/api/schemas";
import { validateAll, validateValue } from "./validate";

const node = (overrides: Partial<SettingsSchemaNode>): SettingsSchemaNode => ({
  key: "x",
  type: "string",
  uiGroup: "x",
  default: "",
  ...overrides,
});

describe("validateValue", () => {
  it("accepts valid enum and rejects others", () => {
    const n = node({ type: "enum", enumValues: ["light", "dark"] });
    expect(validateValue(n, "light")).toBeNull();
    expect(validateValue(n, "sepia")).toMatch(/Must be one of/);
    expect(validateValue(n, 1)).toMatch(/Expected a string/);
  });

  it("enforces integer, number, and range", () => {
    const i = node({ type: "integer", minimum: 0, maximum: 19 });
    expect(validateValue(i, 10)).toBeNull();
    expect(validateValue(i, 20)).toMatch(/≤ 19/);
    expect(validateValue(i, -1)).toMatch(/≥ 0/);
    expect(validateValue(i, 1.5)).toMatch(/integer/);
    expect(validateValue(i, "10")).toMatch(/integer/);

    const n = node({ type: "number", minimum: 0, maximum: 1 });
    expect(validateValue(n, 0.1)).toBeNull();
    expect(validateValue(n, Infinity)).toMatch(/finite/);
  });

  it("enforces pattern on strings", () => {
    const p = node({ type: "string", pattern: "^#[0-9a-fA-F]{6}$" });
    expect(validateValue(p, "#0ea5e9")).toBeNull();
    expect(validateValue(p, "sky")).toMatch(/Must match/);
  });

  it("validates array item type", () => {
    const a = node({ type: "array", itemType: "string" });
    expect(validateValue(a, ["a", "b"])).toBeNull();
    expect(validateValue(a, ["a", 2])).toMatch(/Item #2/);
    expect(validateValue(a, "oops")).toMatch(/list/);
  });

  it("rejects non-object for object fields", () => {
    const o = node({ type: "object" });
    expect(validateValue(o, { a: 1 })).toBeNull();
    expect(validateValue(o, [])).toMatch(/object/);
    expect(validateValue(o, "x")).toMatch(/object/);
  });
});

describe("validateAll", () => {
  it("aggregates per-key errors", () => {
    const nodes: SettingsSchemaNode[] = [
      node({ key: "a", type: "integer", minimum: 0, maximum: 10 }),
      node({ key: "b", type: "enum", enumValues: ["x", "y"] }),
    ];
    const errs = validateAll(nodes, { a: 99, b: "z" });
    expect(Object.keys(errs).sort()).toEqual(["a", "b"]);
  });
});
