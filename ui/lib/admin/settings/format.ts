import type { SettingsSchemaNode } from "@/lib/api/schemas";

/**
 * Derive a human label from a dotted key. Splits on dots and camelCase
 * boundaries, title-cases each word. `map.defaultCenter` →
 * "Default Center". The top-level group is intentionally dropped from
 * the label — it's the group header in the UI.
 */
export function labelForKey(key: string): string {
  const parts = key.split(".");
  const leaf = parts[parts.length - 1];
  return humanise(leaf);
}

/** Humanise a camelCase or snake_case identifier. */
export function humanise(id: string): string {
  const spaced = id
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .trim();
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

/** Title for a top-level group ("map" → "Map"). */
export function labelForGroup(group: string): string {
  if (group === "pois") return "POIs";
  if (group === "rateLimit") return "Rate limit";
  if (group === "ui") return "UI";
  return humanise(group);
}

/** A concise hint line: description + unit (if any). */
export function hintForNode(node: SettingsSchemaNode): string {
  const bits: string[] = [];
  if (node.description) bits.push(node.description);
  if (node.unit) bits.push(`Unit: ${node.unit}`);
  if (node.minimum !== undefined && node.maximum !== undefined) {
    bits.push(`Range ${node.minimum}…${node.maximum}`);
  } else if (node.minimum !== undefined) {
    bits.push(`Min ${node.minimum}`);
  } else if (node.maximum !== undefined) {
    bits.push(`Max ${node.maximum}`);
  }
  return bits.join(" · ");
}

/** Group the flat node list by UIGroup, preserving input order. */
export function groupNodes(
  nodes: SettingsSchemaNode[],
): Array<{ group: string; nodes: SettingsSchemaNode[] }> {
  const map = new Map<string, SettingsSchemaNode[]>();
  for (const n of nodes) {
    const arr = map.get(n.uiGroup) ?? [];
    arr.push(n);
    map.set(n.uiGroup, arr);
  }
  return Array.from(map.entries()).map(([group, nodes]) => ({ group, nodes }));
}

/** Render any value to the text the textarea/json field shows. */
export function stringifyValue(v: unknown): string {
  if (v === null || v === undefined) return "";
  if (typeof v === "string") return v;
  return JSON.stringify(v, null, 2);
}

/** Try to parse a JSON string; return [ok, value] so callers can branch. */
export function tryParseJson(text: string): { ok: true; value: unknown } | { ok: false; error: string } {
  const trimmed = text.trim();
  if (trimmed === "") return { ok: true, value: null };
  try {
    return { ok: true, value: JSON.parse(trimmed) };
  } catch (err) {
    return { ok: false, error: (err as Error).message };
  }
}
