/* eslint-disable no-restricted-globals */
// LocalMaps service worker. Plain ES script (no bundler) so it can be
// shipped verbatim from /public without a build step. Kept in sync with
// `ui/lib/pwa/*` logic — the helpers there carry the unit tests; this
// file is the thin runtime wiring.
//
// Scope: /  (served from /sw.js so navigation + tile fetches are covered)
// Caches:
//   tiles-v1   — /api/tiles/{z}/{x}/{y}.pbf          (stale-while-revalidate, LRU)
//   shell-v1   — /, /_next/*, /offline.html          (stale-while-revalidate)
//   api-v1     — other /api/*                        (network-first 5s → cache)
//
// Message channel:
//   {type:'cachePurge'}          → wipe all our caches
//   {type:'cachePurge', key}     → wipe a single cache name
//   {type:'cacheStats'}          → reply with {tiles,shell,api} counts
//   {type:'cacheRegion', key}    → precache a list of tile URLs
//   {type:'configureTileBudget', bytes} → change LRU budget
//   {type:'skipWaiting'}         → trigger activation of a pending SW

const TILE_CACHE = "tiles-v1";
const SHELL_CACHE = "shell-v1";
const API_CACHE = "api-v1";
const KNOWN = [TILE_CACHE, SHELL_CACHE, API_CACHE];
const OFFLINE_URL = "/offline.html";
const NET_FIRST_TIMEOUT_MS = 5000;
let TILE_BUDGET_BYTES = 500 * 1024 * 1024;

const OFFLINE_HTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/><title>Offline — LocalMaps</title><style>html,body{margin:0;height:100%;font-family:system-ui,sans-serif;background:#0b1220;color:#e2e8f0;display:flex;align-items:center;justify-content:center}main{text-align:center;padding:2rem;max-width:32ch}h1{font-size:1.25rem;margin:0 0 .5rem}p{opacity:.8;margin:.25rem 0}</style></head><body><main><h1>You're offline</h1><p>LocalMaps needs a network connection to load new regions.</p><p>Previously-viewed tiles will still work.</p></main></body></html>`;

// ---------- install / activate ----------
self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const shell = await caches.open(SHELL_CACHE);
      await shell.put(
        OFFLINE_URL,
        new Response(OFFLINE_HTML, {
          headers: { "content-type": "text/html; charset=utf-8" },
        }),
      );
      await self.skipWaiting();
    })(),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const names = await caches.keys();
      await Promise.all(
        names.map((n) => (KNOWN.includes(n) ? null : caches.delete(n))),
      );
      await self.clients.claim();
    })(),
  );
});

// ---------- fetch routing ----------
self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  let url;
  try {
    url = new URL(req.url);
  } catch {
    return;
  }
  if (url.origin !== self.location.origin) return;

  if (isTileReq(url)) {
    event.respondWith(handleTile(req));
    return;
  }
  if (isShellReq(url, req)) {
    event.respondWith(handleShell(req));
    return;
  }
  if (url.pathname.startsWith("/api/")) {
    event.respondWith(handleApi(req));
    return;
  }
  // Else → passthrough (browser default).
});

function isTileReq(url) {
  return /^\/api\/tiles\/\d+\/\d+\/\d+\.pbf$/.test(url.pathname);
}

function isShellReq(url, req) {
  if (req.mode === "navigate") return true;
  if (url.pathname.startsWith("/_next/")) return true;
  if (url.pathname === "/" || url.pathname === "/manifest.webmanifest") {
    return true;
  }
  return false;
}

async function handleTile(request) {
  const cache = await caches.open(TILE_CACHE);
  try {
    return await swr(request, cache, async (req, res) => {
      await enforceLruBudget(cache, req, res).catch(() => {});
    });
  } catch {
    return new Response("", { status: 504, statusText: "offline" });
  }
}

async function handleShell(request) {
  const cache = await caches.open(SHELL_CACHE);
  try {
    return await swr(request, cache);
  } catch {
    if (request.mode === "navigate") {
      const fallback = await cache.match(OFFLINE_URL);
      if (fallback) return fallback;
    }
    return new Response("", { status: 504, statusText: "offline" });
  }
}

async function handleApi(request) {
  const cache = await caches.open(API_CACHE);
  try {
    return await networkFirst(request, cache, NET_FIRST_TIMEOUT_MS);
  } catch {
    return new Response(
      JSON.stringify({ code: "OFFLINE", message: "Offline; no cached copy" }),
      {
        status: 504,
        headers: { "content-type": "application/json" },
      },
    );
  }
}

// ---------- strategies ----------
async function swr(request, cache, onCached) {
  const cached = await cache.match(request);
  const refresh = fetch(request)
    .then(async (res) => {
      if (res && res.status >= 200 && res.status < 300) {
        await cache.put(request, res.clone());
        if (onCached) await onCached(request, res);
      }
      return res;
    })
    .catch(() => undefined);
  if (cached) {
    void refresh;
    return cached;
  }
  const fresh = await refresh;
  if (fresh) return fresh;
  throw new Error("no response");
}

async function networkFirst(request, cache, timeoutMs) {
  const controller = new AbortController();
  const timer = new Promise((resolve) =>
    setTimeout(() => resolve("timeout"), timeoutMs),
  );
  let netErr;
  const netP = fetch(new Request(request, { signal: controller.signal }))
    .then(async (res) => {
      if (res && res.status >= 200 && res.status < 300) {
        await cache.put(request, res.clone());
      }
      return res;
    })
    .catch((e) => {
      netErr = e;
      return "error";
    });
  const raced = await Promise.race([netP, timer]);
  if (raced instanceof Response) return raced;
  if (raced === "timeout") controller.abort();
  const cached = await cache.match(request);
  if (cached) return cached;
  if (netErr) throw netErr;
  throw new Error("network-first: no response");
}

// ---------- LRU (coarse-grained, recency-by-insertion) ----------
async function enforceLruBudget(cache) {
  const keys = await cache.keys();
  // Total size is expensive to compute — we approximate with a per-entry
  // average via a running estimate so eviction stays O(n). Target: drop
  // oldest 10% once total entries × 20KB (guessed tile size) > budget.
  const estimated = keys.length * 20 * 1024;
  if (estimated <= TILE_BUDGET_BYTES) return;
  const drop = Math.max(1, Math.floor(keys.length * 0.1));
  for (let i = 0; i < drop; i++) {
    await cache.delete(keys[i]).catch(() => {});
  }
}

// ---------- message channel ----------
self.addEventListener("message", (event) => {
  const data = event.data || {};
  if (data.type === "skipWaiting") {
    self.skipWaiting();
    return;
  }
  event.waitUntil(handleMessage(event));
});

async function handleMessage(event) {
  const data = event.data || {};
  const reply = (payload) => event.source && event.source.postMessage(payload);
  if (data.type === "cachePurge") {
    if (data.key && KNOWN.includes(data.key)) {
      await caches.delete(data.key);
    } else {
      await Promise.all(KNOWN.map((n) => caches.delete(n)));
    }
    reply({ type: "cachePurged", key: data.key ?? "all" });
    return;
  }
  if (data.type === "cacheStats") {
    const out = { tiles: 0, shell: 0, api: 0 };
    for (const [name, prop] of [
      [TILE_CACHE, "tiles"],
      [SHELL_CACHE, "shell"],
      [API_CACHE, "api"],
    ]) {
      if (await caches.has(name)) {
        const c = await caches.open(name);
        const k = await c.keys();
        out[prop] = k.length;
      }
    }
    reply({ type: "cacheStatsResult", stats: out });
    return;
  }
  if (data.type === "cacheRegion" && Array.isArray(data.urls)) {
    const cache = await caches.open(TILE_CACHE);
    const results = await Promise.allSettled(
      data.urls.map(async (u) => {
        const res = await fetch(u);
        if (res.ok) await cache.put(new Request(u), res.clone());
      }),
    );
    const ok = results.filter((r) => r.status === "fulfilled").length;
    reply({ type: "cacheRegionResult", ok, total: data.urls.length });
    return;
  }
  if (data.type === "configureTileBudget" && typeof data.bytes === "number") {
    TILE_BUDGET_BYTES = Math.max(1024 * 1024, data.bytes);
    reply({ type: "configured", budget: TILE_BUDGET_BYTES });
  }
}
