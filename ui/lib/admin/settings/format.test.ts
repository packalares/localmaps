import { describe, expect, it } from "vitest";
import {
  groupNodes,
  hintForNode,
  labelForGroup,
  labelForKey,
  tryParseJson,
} from "./format";
import type { SettingsSchemaNode } from "@/lib/api/schemas";

describe("labelForKey / labelForGroup", () => {
  it("humanises camelCase leaves", () => {
    expect(labelForKey("map.defaultCenter")).toBe("Default Center");
    expect(labelForKey("routing.truck.heightMeters")).toBe("Height Meters");
  });
  it("uses curated group labels where relevant", () => {
    expect(labelForGroup("pois")).toBe("POIs");
    expect(labelForGroup("rateLimit")).toBe("Rate limit");
    expect(labelForGroup("ui")).toBe("UI");
    expect(labelForGroup("map")).toBe("Map");
  });
});

describe("hintForNode", () => {
  const node = (o: Partial<SettingsSchemaNode>): SettingsSchemaNode => ({
    key: "x",
    type: "integer",
    uiGroup: "x",
    default: 0,
    ...o,
  });
  it("joins description with unit and range", () => {
    expect(
      hintForNode(
        node({ description: "Max zoom.", unit: "zoom", minimum: 0, maximum: 19 }),
      ),
    ).toBe("Max zoom. · Unit: zoom · Range 0…19");
  });
  it("degrades gracefully with no metadata", () => {
    expect(hintForNode(node({}))).toBe("");
  });
});

describe("tryParseJson", () => {
  it("accepts valid JSON and blank", () => {
    expect(tryParseJson("{\"a\":1}")).toEqual({ ok: true, value: { a: 1 } });
    expect(tryParseJson("  ")).toEqual({ ok: true, value: null });
  });
  it("reports parse errors", () => {
    const r = tryParseJson("{bad");
    expect(r.ok).toBe(false);
  });
});

describe("groupNodes", () => {
  it("groups by uiGroup preserving order", () => {
    const nodes: SettingsSchemaNode[] = [
      { key: "a.x", type: "string", uiGroup: "a", default: "" },
      { key: "b.y", type: "string", uiGroup: "b", default: "" },
      { key: "a.z", type: "string", uiGroup: "a", default: "" },
    ];
    const got = groupNodes(nodes);
    expect(got.map((g) => g.group)).toEqual(["a", "b"]);
    expect(got[0].nodes.map((n) => n.key)).toEqual(["a.x", "a.z"]);
  });
});
