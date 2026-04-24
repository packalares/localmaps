/**
 * Fetch strategies reused by the service worker. Each helper is written
 * to be driven by injectable `fetch` and `cache` seams so the tests can
 * exercise the semantics without a real SW global.
 *
 * Strategies covered:
 *  - staleWhileRevalidate: hand back the cached value immediately and
 *    kick off a background fetch that refreshes the cache.
 *  - networkFirst: race the network against a timeout; fall back to
 *    cache on timeout or network error.
 *
 * Both strategies tolerate a missing cache entry, a failed network, and
 * any combination of the two (the caller decides the offline fallback).
 */

export interface StrategyCache {
  match(request: Request): Promise<Response | undefined>;
  put(request: Request, response: Response): Promise<void>;
}

export interface StrategyContext {
  cache: StrategyCache;
  fetch: (request: Request) => Promise<Response>;
  /** Called with every (request, response) accepted into the cache. Lets
   *  the SW maintain an LRU / purge log out-of-band. */
  onCached?: (request: Request, response: Response) => void | Promise<void>;
  /** Logged + swallowed. */
  onError?: (err: unknown) => void;
}

/** Should we accept a response for caching? Successful 2xx + opaque only. */
export function isCacheable(response: Response): boolean {
  if (!response) return false;
  if (response.type === "opaque") return true;
  return response.status >= 200 && response.status < 300;
}

/**
 * Stale-while-revalidate:
 *  - If cached, return immediately and refresh in the background.
 *  - If not cached, await network; cache if successful; else reject.
 */
export async function staleWhileRevalidate(
  request: Request,
  ctx: StrategyContext,
): Promise<Response> {
  const cached = await ctx.cache.match(request);

  const refresh = ctx
    .fetch(request)
    .then(async (res) => {
      if (isCacheable(res)) {
        await ctx.cache.put(request, res.clone());
        await ctx.onCached?.(request, res);
      }
      return res;
    })
    .catch((err) => {
      ctx.onError?.(err);
      return undefined;
    });

  if (cached) {
    // Detach background refresh — we've already answered.
    void refresh;
    return cached;
  }
  const fresh = await refresh;
  if (fresh) return fresh;
  throw new Error("no cached or fresh response available");
}

export interface NetworkFirstOptions {
  /** Milliseconds before the network fetch is considered stalled. */
  timeoutMs: number;
}

/**
 * Network-first with a timeout:
 *  - Race fetch(request) against an abortable timer.
 *  - On success → cache + return.
 *  - On timeout or error → fall back to cached response if present.
 *  - If neither resolves, re-throw the network error.
 */
export async function networkFirst(
  request: Request,
  ctx: StrategyContext,
  opts: NetworkFirstOptions,
): Promise<Response> {
  const controller = new AbortController();
  const timer = new Promise<"timeout">((resolve) => {
    setTimeout(() => resolve("timeout"), opts.timeoutMs).unref?.();
  });

  let networkError: unknown;
  const networkPromise = ctx
    .fetch(new Request(request, { signal: controller.signal }))
    .then(async (res) => {
      if (isCacheable(res)) {
        await ctx.cache.put(request, res.clone());
        await ctx.onCached?.(request, res);
      }
      return res;
    })
    .catch((err) => {
      networkError = err;
      return "error" as const;
    });

  const raced = await Promise.race([networkPromise, timer]);
  if (raced instanceof Response) return raced;
  if (raced === "timeout") {
    controller.abort();
  }
  const cached = await ctx.cache.match(request);
  if (cached) return cached;
  // No cache and no network → let the caller decide the fallback.
  if (networkError) throw networkError;
  throw new Error("network-first: no response available");
}

/**
 * Convenience: cache-first (used for the offline fallback doc). Tries the
 * cache; on miss, falls through to the network; never writes back.
 */
export async function cacheOnly(
  request: Request,
  ctx: Pick<StrategyContext, "cache">,
): Promise<Response | undefined> {
  return ctx.cache.match(request);
}
