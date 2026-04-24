/**
 * Bottom-right attribution strip used inside the `/embed` viewer.
 *
 * Unlike the chrome Attribution used on the main page, this one:
 *
 *   - is a plain server component (no `"use client"` boilerplate) so we
 *     can SSR the embed page without hydration overhead;
 *   - inlines the OSM + Overture credit per docs/08-security.md — the
 *     text is licence-required and must stay visible in every iframe;
 *   - exposes no settings fetch (cookieless embed — no authenticated
 *     `/api/settings` call).
 *
 * The text default matches `map.attribution` in
 * `docs/07-config-schema.md`; callers can override it if the host ever
 * embeds content with an additional credit requirement.
 */
export interface EmbedAttributionProps {
  /** Override the rendered string; defaults to the documented value. */
  text?: string;
  /** Extra classes, for callers that tuck the strip inside a wider layout. */
  className?: string;
}

const DEFAULT_TEXT =
  "© OpenStreetMap contributors, Overture Maps";

export function EmbedAttribution({
  text = DEFAULT_TEXT,
  className,
}: EmbedAttributionProps) {
  return (
    <div
      role="contentinfo"
      aria-label="Map attribution"
      className={
        "pointer-events-auto rounded-md bg-chrome-surface/90 px-2 py-1 text-[11px] leading-tight text-muted-foreground shadow-chrome ring-1 ring-chrome-border" +
        (className ? " " + className : "")
      }
    >
      {text}
    </div>
  );
}

export default EmbedAttribution;
