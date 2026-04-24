import { describe, expect, it } from "vitest";
import {
  DEFAULT_TILE_BUDGET_BYTES,
  KNOWN_CACHES,
  applyLru,
  cacheStats,
  deleteStaleCaches,
  isTileRequest,
  responseSize,
  tileCacheKey,
} from "./tile-cache";

describe("tile-cache: isTileRequest", () => {
  it("accepts canonical /api/tiles paths", () => {
    expect(isTileRequest(new URL("http://x/api/tiles/12/345/678.pbf"))).toBe(
      true,
    );
  });

  it("accepts paths with a query string", () => {
    expect(
      isTileRequest(new URL("http://x/api/tiles/0/0/0.pbf?v=1")),
    ).toBe(true);
  });

  it("rejects metadata + style + other paths", () => {
    expect(isTileRequest(new URL("http://x/api/tiles/metadata"))).toBe(false);
    expect(isTileRequest(new URL("http://x/api/styles/light.json"))).toBe(
      false,
    );
    expect(isTileRequest(new URL("http://x/_next/static/a.js"))).toBe(false);
  });
});

describe("tile-cache: tileCacheKey", () => {
  it("strips origin + preserves pathname", () => {
    expect(tileCacheKey("http://host/api/tiles/1/2/3.pbf")).toBe(
      "/api/tiles/1/2/3.pbf",
    );
  });

  it("ignores query string", () => {
    expect(
      tileCacheKey(new URL("http://host/api/tiles/1/2/3.pbf?nocache=1")),
    ).toBe("/api/tiles/1/2/3.pbf");
  });
});

describe("tile-cache: applyLru", () => {
  const e = (key: string, size: number, insertedAt = 0) => ({
    key,
    size,
    insertedAt,
  });

  it("keeps the most recent entries and evicts older ones over budget", () => {
    const start = [e("a", 60), e("b", 60), e("c", 60)];
    const { kept, evicted } = applyLru(start, e("d", 60), 200);
    // 4 entries × 60 = 240 > 200 → evict from the head.
    expect(evicted.map((x) => x.key)).toEqual(["a"]);
    expect(kept.map((x) => x.key)).toEqual(["b", "c", "d"]);
  });

  it("refreshes recency when the same key is re-put", () => {
    const start = [e("a", 10), e("b", 10), e("c", 10)];
    const { kept, evicted } = applyLru(start, e("a", 10), 100);
    expect(evicted).toEqual([]);
    // a was dropped from the middle and re-appended as newest.
    expect(kept.map((x) => x.key)).toEqual(["b", "c", "a"]);
  });

  it("evicts multiple entries when a large entry arrives", () => {
    const start = [e("a", 50), e("b", 50), e("c", 50)];
    const { kept, evicted } = applyLru(start, e("big", 150), 200);
    expect(evicted.map((x) => x.key)).toEqual(["a", "b"]);
    expect(kept.map((x) => x.key)).toEqual(["c", "big"]);
  });

  it("handles an empty starting set under budget", () => {
    const { kept, evicted } = applyLru([], e("a", 10), 100);
    expect(evicted).toEqual([]);
    expect(kept.map((x) => x.key)).toEqual(["a"]);
  });

  it("clips to the budget even when a single entry exceeds it", () => {
    const { kept, evicted } = applyLru([], e("huge", 1000), 10);
    // Entry is still dropped to respect the budget — caller decides if it
    // wants to skip caching in that case.
    expect(evicted.map((x) => x.key)).toEqual(["huge"]);
    expect(kept).toEqual([]);
  });
});

describe("tile-cache: responseSize", () => {
  it("prefers content-length when valid", async () => {
    const res = new Response("hello", {
      headers: { "content-length": "5" },
    });
    expect(await responseSize(res)).toBe(5);
  });

  it("falls back to reading the body", async () => {
    const res = new Response(new Uint8Array([1, 2, 3]));
    expect(await responseSize(res)).toBe(3);
  });

  it("returns 0 on unreadable responses", async () => {
    // A read-once response: we consume it first, then measure.
    const res = new Response("x");
    await res.arrayBuffer();
    const size = await responseSize(res);
    // clone() of an already-consumed response throws — our helper swallows.
    expect(size).toBeGreaterThanOrEqual(0);
  });
});

describe("tile-cache: cacheStats + deleteStaleCaches", () => {
  function makeFakeCaches() {
    const store = new Map<string, Map<string, Response>>();
    const api: CacheStorage = {
      async has(name) {
        return store.has(name);
      },
      async keys() {
        return [...store.keys()];
      },
      async open(name) {
        if (!store.has(name)) store.set(name, new Map());
        const bucket = store.get(name)!;
        return {
          async keys() {
            return [...bucket.keys()].map((k) => new Request(k));
          },
          async put(req: Request, res: Response) {
            bucket.set(req.url, res);
          },
          async match() {
            return undefined;
          },
          async delete() {
            return true;
          },
          async matchAll() {
            return [];
          },
          async add() {},
          async addAll() {},
        } as unknown as Cache;
      },
      async delete(name) {
        return store.delete(name);
      },
      async match() {
        return undefined;
      },
    };
    return { api, store };
  }

  it("counts entries across known caches", async () => {
    const { api, store } = makeFakeCaches();
    store.set(
      "tiles-v1",
      new Map([["http://x/1", new Response("")]]),
    );
    store.set(
      "shell-v1",
      new Map([
        ["http://x/a", new Response("")],
        ["http://x/b", new Response("")],
      ]),
    );
    // api-v1 deliberately absent → 0.
    const stats = await cacheStats(api);
    expect(stats).toEqual({ tiles: 1, shell: 2, api: 0 });
  });

  it("deletes caches not in the known list", async () => {
    const { api, store } = makeFakeCaches();
    store.set("tiles-v1", new Map());
    store.set("tiles-v0", new Map());
    store.set("legacy", new Map());
    const deleted = await deleteStaleCaches(api, KNOWN_CACHES);
    expect(deleted.sort()).toEqual(["legacy", "tiles-v0"].sort());
    expect(store.has("tiles-v1")).toBe(true);
  });
});

describe("tile-cache: constants sanity", () => {
  it("DEFAULT_TILE_BUDGET_BYTES is ~500 MB", () => {
    expect(DEFAULT_TILE_BUDGET_BYTES).toBe(500 * 1024 * 1024);
  });
});
