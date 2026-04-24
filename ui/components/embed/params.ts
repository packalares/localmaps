/**
 * Embed query-string parser.
 *
 * The server-side handler in `server/internal/api/embed.go` does the
 * authoritative validation and returns a 400 on malformed input. This
 * module exists for the second validation pass inside the Next.js route:
 * the server handler may serve a redirect into the UI origin, and we want
 * the page to tolerate any value the gateway accepted.
 *
 * `/embed` query shape:
 *
 *   lat     number, WGS84 [-90, 90]
 *   lon     number, WGS84 [-180, 180]
 *   zoom    number, [0, 22]
 *   pin     string "lat,lon" or "lat,lon:label"
 *   style   enum   "light" | "dark" | "print"
 *   region  string canonical hyphenated key (e.g. "europe-romania")
 *
 * Anything malformed is silently dropped back to its default — the server
 * route rejects bad input before we get here, so reaching this parser with
 * a bad value means the operator bypassed the gateway.
 */

export interface EmbedPin {
  lat: number;
  lon: number;
  label?: string;
}

export interface ParsedEmbedParams {
  center: { lat: number; lon: number };
  zoom: number;
  style: "light" | "dark" | "print";
  region: string | null;
  pin: EmbedPin | null;
}

type SearchMap = Record<string, string | string[] | undefined>;

const DEFAULT_CENTER = { lat: 0, lon: 0 };
const DEFAULT_ZOOM = 2;
const DEFAULT_STYLE: ParsedEmbedParams["style"] = "light";

const CANONICAL_REGION_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/;
const ALLOWED_STYLES = new Set<ParsedEmbedParams["style"]>([
  "light",
  "dark",
  "print",
]);

function firstString(value: string | string[] | undefined): string | null {
  if (Array.isArray(value)) return value[0] ?? null;
  return value ?? null;
}

function parseFiniteNumber(
  raw: string | null,
  min: number,
  max: number,
): number | null {
  if (raw === null) return null;
  const n = Number.parseFloat(raw);
  if (!Number.isFinite(n)) return null;
  if (n < min || n > max) return null;
  return n;
}

/** Parse "lat,lon[:label]" into an `EmbedPin`, or `null` if malformed. */
export function parsePinParam(raw: string | null): EmbedPin | null {
  if (!raw) return null;
  const [coord, ...rest] = raw.split(":");
  const label = rest.length > 0 ? rest.join(":") : undefined;
  const parts = (coord ?? "").split(",");
  if (parts.length !== 2) return null;
  const lat = parseFiniteNumber(parts[0] ?? null, -90, 90);
  const lon = parseFiniteNumber(parts[1] ?? null, -180, 180);
  if (lat === null || lon === null) return null;
  if (label !== undefined) {
    if (label.length > 120) return null;
    // Reject control characters to match the server-side validator.
    for (const ch of label) {
      const code = ch.charCodeAt(0);
      if (code < 0x20 || code === 0x7f) return null;
    }
  }
  return label ? { lat, lon, label } : { lat, lon };
}

/** Parse the Next.js `searchParams` object into a typed, validated value. */
export function parseEmbedSearchParams(raw: SearchMap): ParsedEmbedParams {
  const lat = parseFiniteNumber(firstString(raw.lat), -90, 90);
  const lon = parseFiniteNumber(firstString(raw.lon), -180, 180);
  const zoom = parseFiniteNumber(firstString(raw.zoom), 0, 22);
  const styleRaw = firstString(raw.style);
  const regionRaw = firstString(raw.region);
  const pin = parsePinParam(firstString(raw.pin));

  const style: ParsedEmbedParams["style"] =
    styleRaw && ALLOWED_STYLES.has(styleRaw as ParsedEmbedParams["style"])
      ? (styleRaw as ParsedEmbedParams["style"])
      : DEFAULT_STYLE;

  const region =
    regionRaw && CANONICAL_REGION_RE.test(regionRaw) ? regionRaw : null;

  // If only one of lat/lon is supplied, treat both as missing — a half
  // coordinate is meaningless for a centre.
  const center =
    lat !== null && lon !== null ? { lat, lon } : DEFAULT_CENTER;

  return {
    center,
    zoom: zoom ?? DEFAULT_ZOOM,
    style,
    region,
    pin,
  };
}
