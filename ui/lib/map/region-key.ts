/**
 * Region name <-> canonical-key helpers.
 *
 * Geofabrik names regions with slashes: `europe/romania`, `europe/germany/berlin`.
 * The filesystem + URL encoding prefers hyphens — so the canonical key used
 * inside the app (Zustand `activeRegion`, URL `?r=`, gateway query param) is
 * the hyphenated form: `europe-romania`.
 *
 * This module is the single source of truth for the conversion. Any other
 * module that needs to normalise a region identifier MUST use these helpers
 * rather than re-implement the transform.
 */

/** Convert a Geofabrik-style name (`europe/romania`) to hyphen-form. */
export function toCanonicalRegionKey(name: string): string {
  return name.trim().toLowerCase().replace(/\//g, "-");
}

/**
 * Convert a hyphenated canonical key back to a Geofabrik-style path. The
 * mapping is lossy when a display name itself contains hyphens; callers that
 * need the original `/`-form should store the source string instead.
 */
export function fromCanonicalRegionKey(key: string): string {
  return key.trim().toLowerCase().replace(/-/g, "/");
}

/** True if `value` is a plausible canonical region key. */
export function isCanonicalRegionKey(value: unknown): value is string {
  return (
    typeof value === "string" &&
    value.length > 0 &&
    /^[a-z0-9]+(-[a-z0-9]+)*$/.test(value)
  );
}
