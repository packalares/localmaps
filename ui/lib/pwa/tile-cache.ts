/**
 * Tile-cache helpers: cache names, request keying, and LRU eviction.
 *
 * These helpers are intentionally framework-agnostic so both the service
 * worker (`public/sw.js`) and the tests can exercise the same primitives.
 * The helpers deal in plain Request/Response/Cache shapes so they work
 * under both the DOM and ServiceWorker globals.
 *
 * The cache is scoped with a version suffix so bumping the SW forces a
 * fresh cache without leaving the old one dangling — the SW's `activate`
 * step runs `deleteStaleCaches()` with the current version set.
 */
export const TILE_CACHE = "tiles-v1" as const;
export const SHELL_CACHE = "shell-v1" as const;
export const API_CACHE = "api-v1" as const;

export const KNOWN_CACHES = [TILE_CACHE, SHELL_CACHE, API_CACHE] as const;
export type KnownCache = (typeof KNOWN_CACHES)[number];

/**
 * Default max size for the tiles cache. Operators can override at runtime
 * via a `{type: 'configureTileBudget', bytes}` message to the SW.
 */
export const DEFAULT_TILE_BUDGET_BYTES = 500 * 1024 * 1024;

/**
 * Only vector tile endpoints flow through the cache. Metadata + styles are
 * handled by the generic API cache so this matcher stays laser-focused.
 */
const TILE_PATTERN = /^\/api\/tiles\/\d+\/\d+\/\d+\.pbf(?:\?.*)?$/;

export function isTileRequest(url: URL): boolean {
  return TILE_PATTERN.test(url.pathname + url.search);
}

/**
 * Returns a canonical cache key for a tile URL. Strips query strings the
 * SW layer doesn't care about so tiles hit even if upstream adds a nonce
 * or if a client appends `?v=...` to bust Next's rewrite cache.
 *
 * Note: Cache.match() uses the full Request URL by default; we store keys
 * as a pathname-only Request so we can LRU-order by a deterministic key.
 */
export function tileCacheKey(urlLike: string | URL): string {
  const u = typeof urlLike === "string" ? new URL(urlLike, SAFE_BASE) : urlLike;
  return u.pathname;
}

const SAFE_BASE = "http://localmaps.invalid";

/**
 * Minimal LRU record. We track the byte size so eviction can stay below a
 * configurable budget without a full pass over every Response.
 */
export interface LruEntry {
  key: string;
  size: number;
  insertedAt: number;
}

/** Adds an entry + evicts from the head until total size ≤ budget. */
export function applyLru(
  entries: LruEntry[],
  incoming: LruEntry,
  budgetBytes: number,
): { kept: LruEntry[]; evicted: LruEntry[] } {
  // Drop any prior entry with the same key so re-puts refresh recency.
  const filtered = entries.filter((e) => e.key !== incoming.key);
  const kept = [...filtered, incoming];
  const evicted: LruEntry[] = [];
  let total = kept.reduce((acc, e) => acc + e.size, 0);
  while (total > budgetBytes && kept.length > 0) {
    const head = kept.shift();
    if (!head) break;
    evicted.push(head);
    total -= head.size;
  }
  return { kept, evicted };
}

/** Response byte-size estimate. Prefers Content-Length, falls back to
 *  reading the clone. Returns 0 on any error so callers can still proceed. */
export async function responseSize(response: Response): Promise<number> {
  const header = response.headers.get("content-length");
  if (header && !Number.isNaN(Number(header))) return Number(header);
  try {
    const buf = await response.clone().arrayBuffer();
    return buf.byteLength;
  } catch {
    return 0;
  }
}

export interface CacheStats {
  tiles: number;
  shell: number;
  api: number;
}

/** Counts the number of entries in each known cache. Used for the
 *  `cacheStats` postMessage reply. */
export async function cacheStats(
  caches: CacheStorage,
): Promise<CacheStats> {
  const [tiles, shell, api] = await Promise.all([
    countCache(caches, TILE_CACHE),
    countCache(caches, SHELL_CACHE),
    countCache(caches, API_CACHE),
  ]);
  return { tiles, shell, api };
}

async function countCache(
  caches: CacheStorage,
  name: string,
): Promise<number> {
  const has = await caches.has(name);
  if (!has) return 0;
  const cache = await caches.open(name);
  const keys = await cache.keys();
  return keys.length;
}

/** Deletes caches that aren't in the current known set. */
export async function deleteStaleCaches(
  caches: CacheStorage,
  known: readonly string[] = KNOWN_CACHES,
): Promise<string[]> {
  const names = await caches.keys();
  const deleted: string[] = [];
  await Promise.all(
    names.map(async (n) => {
      if (!known.includes(n)) {
        await caches.delete(n);
        deleted.push(n);
      }
    }),
  );
  return deleted;
}
