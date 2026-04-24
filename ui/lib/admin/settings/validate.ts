import type { SettingsSchemaNode } from "@/lib/api/schemas";

/**
 * Client-side mirror of server/internal/settingsschema.ValidateValue.
 * Produces the first user-facing error for a single field, or null if
 * the value is acceptable. UI forms call this on change to show inline
 * feedback; the PATCH handler re-validates before writing.
 */
export function validateValue(
  node: SettingsSchemaNode,
  value: unknown,
): string | null {
  switch (node.type) {
    case "boolean":
      if (typeof value !== "boolean") return "Expected true or false.";
      return null;
    case "string":
      if (typeof value !== "string") return "Expected text.";
      if (node.pattern) {
        try {
          const re = new RegExp(node.pattern);
          if (!re.test(value)) return `Must match ${node.pattern}.`;
        } catch {
          // An unparseable regex is a server bug; don't block the UI.
        }
      }
      return null;
    case "enum":
      if (typeof value !== "string") return "Expected a string.";
      if (!(node.enumValues ?? []).includes(value)) {
        return `Must be one of: ${(node.enumValues ?? []).join(", ")}.`;
      }
      return null;
    case "integer": {
      if (typeof value !== "number") return "Expected an integer.";
      if (!Number.isFinite(value)) return "Expected a finite number.";
      if (!Number.isInteger(value)) return "Expected an integer.";
      return rangeError(node, value);
    }
    case "number": {
      if (typeof value !== "number") return "Expected a number.";
      if (!Number.isFinite(value)) return "Expected a finite number.";
      return rangeError(node, value);
    }
    case "array": {
      if (!Array.isArray(value)) return "Expected a list.";
      if (node.itemType === "string") {
        for (let i = 0; i < value.length; i++) {
          if (typeof value[i] !== "string") {
            return `Item #${i + 1} must be text.`;
          }
        }
      } else if (node.itemType === "integer") {
        for (let i = 0; i < value.length; i++) {
          const v = value[i];
          if (typeof v !== "number" || !Number.isInteger(v)) {
            return `Item #${i + 1} must be an integer.`;
          }
        }
      } else if (node.itemType === "object") {
        for (let i = 0; i < value.length; i++) {
          if (
            value[i] === null ||
            typeof value[i] !== "object" ||
            Array.isArray(value[i])
          ) {
            return `Item #${i + 1} must be an object.`;
          }
        }
      }
      return null;
    }
    case "object":
      if (value === null || typeof value !== "object" || Array.isArray(value)) {
        return "Expected an object.";
      }
      return null;
    default:
      return null;
  }
}

function rangeError(
  node: SettingsSchemaNode,
  v: number,
): string | null {
  if (node.minimum !== undefined && v < node.minimum) {
    return `Must be ≥ ${node.minimum}.`;
  }
  if (node.maximum !== undefined && v > node.maximum) {
    return `Must be ≤ ${node.maximum}.`;
  }
  return null;
}

/**
 * Validate an entire draft (key → value) against the schema. Returns a
 * map of key → error message for every field that's currently invalid.
 */
export function validateAll(
  nodes: SettingsSchemaNode[],
  draft: Record<string, unknown>,
): Record<string, string> {
  const errors: Record<string, string> = {};
  for (const n of nodes) {
    if (!(n.key in draft)) continue;
    const err = validateValue(n, draft[n.key]);
    if (err) errors[n.key] = err;
  }
  return errors;
}
