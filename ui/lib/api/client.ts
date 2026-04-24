import { z } from "zod";
import { apiUrl } from "@/lib/env";
import { ErrorResponseSchema } from "./schemas";

/**
 * Thin fetch wrapper used by every React-Query hook in the UI.
 *
 * Responsibilities:
 * - Prepend the gateway base URL.
 * - Generate a client-side trace id and forward it as X-Trace-Id.
 * - Parse JSON, validate against a zod schema.
 * - Map non-2xx responses into `ApiError` with traceId + retryable flag.
 *
 * Note on generated types: Agent C produces `../contracts/ts/api.d.ts`
 * via `openapi-typescript`. We use `import type` so the build does not
 * fail if that file is not yet present.
 */

// Note: the OpenAPI-generated types live at `@/contracts/api`
// (produced by `npm run typegen`). Consumers that want strongly-typed
// request/response shapes can `import type { components } from
// "@/contracts/api"` directly; this client validates at the zod layer
// and deliberately avoids coupling its internals to generated types so
// the UI still compiles before Agent C produces them.

export interface ApiErrorPayload {
  code: string;
  message: string;
  retryable: boolean;
  traceId: string;
}

export class ApiError extends Error {
  readonly code: string;
  readonly retryable: boolean;
  readonly traceId: string;
  readonly status: number;

  constructor(status: number, payload: ApiErrorPayload) {
    super(payload.message);
    this.name = "ApiError";
    this.status = status;
    this.code = payload.code;
    this.retryable = payload.retryable;
    this.traceId = payload.traceId;
  }
}

function newTraceId(): string {
  // 96 bits of randomness is plenty for a client-side correlation id.
  const buf = new Uint8Array(12);
  if (typeof crypto !== "undefined" && "getRandomValues" in crypto) {
    crypto.getRandomValues(buf);
  } else {
    for (let i = 0; i < buf.length; i++) {
      buf[i] = Math.floor(Math.random() * 256);
    }
  }
  return Array.from(buf, (b) => b.toString(16).padStart(2, "0")).join("");
}

export interface RequestOptions<TSchema extends z.ZodTypeAny> {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  path: string;
  query?: Record<string, string | number | boolean | undefined | null>;
  body?: unknown;
  schema: TSchema;
  signal?: AbortSignal;
  /** Extra request headers (e.g. Accept overrides). */
  headers?: Record<string, string>;
}

function buildUrl(
  path: string,
  query?: RequestOptions<z.ZodTypeAny>["query"],
): string {
  const base = apiUrl(path);
  if (!query) return base;
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null) continue;
    params.set(k, String(v));
  }
  const qs = params.toString();
  return qs.length > 0 ? `${base}?${qs}` : base;
}

// redirectTo401LoginPath lets callers opt out of the automatic
// "bounce to /login on 401" behaviour (e.g. /api/auth/me itself).
export interface RequestOptionsExtras {
  /** Skip the 401 → /login?rd=<path> redirect. Default false. */
  noAuthRedirect?: boolean;
}

/**
 * Redirect the browser to /login?rd=<current-path> the first time the
 * server returns 401. Safe to call repeatedly; subsequent invocations
 * no-op until the page reloads.
 */
let redirectingToLogin = false;
export function redirectToLogin(reason: "401" | "expired" = "401"): void {
  if (redirectingToLogin) return;
  if (typeof window === "undefined") return;
  redirectingToLogin = true;
  const here = window.location.pathname + window.location.search;
  const rd = here && here !== "/login" ? `?rd=${encodeURIComponent(here)}` : "";
  // Avoid an infinite loop when the 401 happens on the login page itself.
  if (window.location.pathname.startsWith("/login")) {
    redirectingToLogin = false;
    return;
  }
  window.location.href = `/login${rd}`;
}

export async function apiRequest<TSchema extends z.ZodTypeAny>(
  options: RequestOptions<TSchema> & RequestOptionsExtras,
): Promise<z.infer<TSchema>> {
  const traceId = newTraceId();
  const headers: Record<string, string> = {
    Accept: "application/json",
    "X-Trace-Id": traceId,
    ...options.headers,
  };
  const init: RequestInit = {
    method: options.method ?? "GET",
    headers,
    signal: options.signal,
    // Include the session cookie on same-origin requests.
    credentials: "include",
  };
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(options.body);
  }

  const url = buildUrl(options.path, options.query);
  const response = await fetch(url, init);

  if (!response.ok) {
    let payload: ApiErrorPayload = {
      code: "INTERNAL",
      message: response.statusText || "Request failed",
      retryable: response.status >= 500,
      traceId,
    };
    try {
      const json = await response.json();
      const parsed = ErrorResponseSchema.safeParse(json);
      if (parsed.success) {
        payload = {
          code: parsed.data.error.code,
          message: parsed.data.error.message,
          retryable: parsed.data.error.retryable,
          traceId: parsed.data.traceId || traceId,
        };
      }
    } catch {
      // Non-JSON error body. Keep the defaults.
    }
    if (response.status === 401 && !options.noAuthRedirect) {
      redirectToLogin("401");
    }
    throw new ApiError(response.status, payload);
  }

  const json = await response.json();
  const parsed = options.schema.safeParse(json);
  if (!parsed.success) {
    throw new ApiError(response.status, {
      code: "SCHEMA_MISMATCH",
      message: `Response failed schema validation: ${parsed.error.message}`,
      retryable: false,
      traceId,
    });
  }
  return parsed.data;
}
