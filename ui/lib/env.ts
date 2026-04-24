/**
 * Runtime environment accessors.
 *
 * The gateway base URL is the only address the UI ever talks to. In
 * production the UI and gateway share an origin and the default empty
 * base ("") produces same-origin relative paths. In local dev the
 * developer sets NEXT_PUBLIC_GATEWAY_URL to the gateway's address and
 * Next's rewrites proxy `/api/*`, `/og/*`, `/embed` to it.
 */

/** Normalised gateway base URL with no trailing slash. */
export function gatewayBaseUrl(): string {
  const raw = process.env.NEXT_PUBLIC_GATEWAY_URL ?? "";
  return raw.replace(/\/$/, "");
}

/** Build a fully-qualified URL for an API path. */
export function apiUrl(path: string): string {
  const normalised = path.startsWith("/") ? path : `/${path}`;
  return `${gatewayBaseUrl()}${normalised}`;
}

/** Build a MapLibre style URL for a named style. */
export function styleUrl(name: "light" | "dark" | "print"): string {
  return apiUrl(`/api/styles/${name}.json`);
}
