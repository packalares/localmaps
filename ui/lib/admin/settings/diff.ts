import type { SettingsTree } from "@/lib/api/schemas";

/**
 * Compute the PATCH body as a flat dotted-key map. Only includes keys
 * whose value differs between `original` and `current`. Deep equality is
 * used so re-ordering object keys is ignored.
 */
export function diffSettings(
  original: Record<string, unknown>,
  current: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  const keys = new Set<string>([
    ...Object.keys(original),
    ...Object.keys(current),
  ]);
  for (const k of keys) {
    if (!deepEqual(original[k], current[k])) {
      out[k] = current[k];
    }
  }
  return out;
}

/** Nested-tree → flat dotted-key map. Known object-leaf keys stay intact. */
export function flattenTree(
  tree: SettingsTree,
  objectLeafKeys: ReadonlySet<string> = OBJECT_LEAF_KEYS,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  const walk = (prefix: string, node: Record<string, unknown>) => {
    for (const [k, v] of Object.entries(node)) {
      const key = prefix ? `${prefix}.${k}` : k;
      if (
        v !== null &&
        typeof v === "object" &&
        !Array.isArray(v) &&
        !objectLeafKeys.has(key)
      ) {
        walk(key, v as Record<string, unknown>);
      } else {
        out[key] = v;
      }
    }
  };
  walk("", tree as Record<string, unknown>);
  return out;
}

/** Keys the server writes as a single JSON row rather than a deep tree. */
export const OBJECT_LEAF_KEYS: ReadonlySet<string> = new Set([
  "map.defaultCenter",
]);

/** Small, tests-friendly deep-equality check for JSON-ish values. */
export function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (a === null || b === null) return a === b;
  if (typeof a !== typeof b) return false;
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (!deepEqual(a[i], b[i])) return false;
    }
    return true;
  }
  if (typeof a === "object" && typeof b === "object") {
    const ao = a as Record<string, unknown>;
    const bo = b as Record<string, unknown>;
    const ak = Object.keys(ao);
    const bk = Object.keys(bo);
    if (ak.length !== bk.length) return false;
    for (const k of ak) {
      if (!deepEqual(ao[k], bo[k])) return false;
    }
    return true;
  }
  return false;
}
