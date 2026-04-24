"use client";

/**
 * Bottom-center attribution strip. The exact text comes from
 * `map.attribution` in the settings tree (default declared in
 * `docs/07-config-schema.md`). Phase-1 we render a static string
 * matching the documented default.
 */
export function Attribution({
  text = "© OpenStreetMap contributors, Overture Maps",
}: {
  text?: string;
}) {
  return (
    <div
      className="pointer-events-auto rounded-md bg-chrome-surface/90 px-2 py-1 text-xs text-muted-foreground shadow-chrome ring-1 ring-chrome-border"
      role="contentinfo"
      aria-label="Map attribution"
    >
      {text}
    </div>
  );
}
