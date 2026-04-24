import { EmbedMap } from "@/components/embed/EmbedMap";
import { EmbedAttribution } from "@/components/embed/EmbedAttribution";
import {
  parseEmbedSearchParams,
  type EmbedPin,
} from "@/components/embed/params";

/**
 * `/embed` — the third-party-iframe viewer.
 *
 * This is a server component: it reads the query string (`lat`, `lon`,
 * `zoom`, `pin`, `style`, `region`) once on the server, validates each
 * value, and passes a typed props object into the client-only
 * `<EmbedMap>` component. We do NOT round-trip through the map store's
 * URL-hash binding — embed URLs are expected to be rendered verbatim by
 * the host page, and any in-iframe panning is persisted to the hash
 * locally without affecting the enclosing document.
 *
 * The matching gateway route at `GET /embed` (see
 * `server/internal/api/embed.go`) applies the CSP + security headers
 * required by `docs/08-security.md`; this page renders the viewer HTML.
 */
type SearchParamValue = string | string[] | undefined;

export interface EmbedPageProps {
  // Next 15 passes this as a Promise; support both shapes for testability.
  searchParams?:
    | Record<string, SearchParamValue>
    | Promise<Record<string, SearchParamValue>>;
}

export default async function EmbedPage({ searchParams }: EmbedPageProps) {
  const resolved =
    (searchParams instanceof Promise ? await searchParams : searchParams) ??
    {};
  const parsed = parseEmbedSearchParams(resolved);

  const pin: EmbedPin | null = parsed.pin ?? null;

  return (
    <main
      className="relative h-dvh w-screen overflow-hidden bg-background text-foreground"
      data-testid="embed-root"
    >
      <EmbedMap
        center={parsed.center}
        zoom={parsed.zoom}
        styleName={parsed.style}
        region={parsed.region}
        pin={pin}
      />

      {/* Attribution is mandatory under docs/08 and the OSM licence. */}
      <div className="pointer-events-none absolute inset-x-0 bottom-0 flex justify-end p-2">
        <EmbedAttribution />
      </div>

      {/* "Open in LocalMaps" escape hatch — target="_top" breaks out of
          the iframe so the user lands on the full viewer. */}
      <div className="pointer-events-none absolute left-0 bottom-0 flex p-2">
        <a
          className="pointer-events-auto rounded-md bg-chrome-surface/90 px-2 py-1 text-xs font-medium text-foreground shadow-chrome ring-1 ring-chrome-border hover:bg-muted"
          target="_top"
          rel="noopener"
          href={buildOpenInAppHref(parsed)}
        >
          Open in LocalMaps
        </a>
      </div>
    </main>
  );
}

/**
 * Construct the "Open in LocalMaps" link. Uses the main viewer's URL-hash
 * format so deep-link round-tripping is symmetric with the standalone
 * page. Region, when present, is carried via the `?r=` query param that
 * `use-url-viewport.ts` honours.
 */
function buildOpenInAppHref(
  parsed: ReturnType<typeof parseEmbedSearchParams>,
): string {
  const { zoom, center, region } = parsed;
  const hash = `#${zoom.toFixed(2)}/${center.lat.toFixed(4)}/${center.lon.toFixed(4)}`;
  const qs = region ? `?r=${encodeURIComponent(region)}` : "";
  return `/${qs}${hash}`;
}

// Ensure server rendering — no secret env access, no dynamic data fetch,
// just pure query-string parsing.
export const dynamic = "force-static";
