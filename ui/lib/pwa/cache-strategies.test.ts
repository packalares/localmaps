import { describe, expect, it, vi } from "vitest";
import {
  cacheOnly,
  isCacheable,
  networkFirst,
  staleWhileRevalidate,
} from "./cache-strategies";

function makeCache(initial: Record<string, Response> = {}) {
  const store = new Map<string, Response>(Object.entries(initial));
  return {
    store,
    cache: {
      async match(req: Request) {
        return store.get(req.url);
      },
      async put(req: Request, res: Response) {
        store.set(req.url, res);
      },
    },
  };
}

const URL_A = "http://host/a";

describe("isCacheable", () => {
  it("accepts 2xx", () => {
    expect(isCacheable(new Response("x", { status: 200 }))).toBe(true);
    // 204 = no body; construct without body to satisfy spec.
    expect(isCacheable(new Response(null, { status: 204 }))).toBe(true);
  });
  it("rejects 4xx/5xx", () => {
    expect(isCacheable(new Response("x", { status: 404 }))).toBe(false);
    expect(isCacheable(new Response("x", { status: 500 }))).toBe(false);
  });
});

describe("staleWhileRevalidate", () => {
  it("returns the cached response immediately and refreshes in the background", async () => {
    const { cache, store } = makeCache({
      [URL_A]: new Response("stale", { status: 200 }),
    });
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(new Response("fresh", { status: 200 }));
    const onCached = vi.fn();

    const res = await staleWhileRevalidate(new Request(URL_A), {
      cache,
      fetch: fetchSpy,
      onCached,
    });
    expect(await res.text()).toBe("stale");
    expect(fetchSpy).toHaveBeenCalledOnce();

    // Wait a microtask round so the background put resolves.
    await new Promise((r) => setTimeout(r, 0));
    expect(await store.get(URL_A)!.text()).toBe("fresh");
    expect(onCached).toHaveBeenCalled();
  });

  it("awaits the network when there is no cache entry", async () => {
    const { cache } = makeCache();
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(new Response("fresh", { status: 200 }));
    const res = await staleWhileRevalidate(new Request(URL_A), {
      cache,
      fetch: fetchSpy,
    });
    expect(await res.text()).toBe("fresh");
  });

  it("does not cache non-2xx responses", async () => {
    const { cache, store } = makeCache();
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(new Response("no", { status: 500 }));
    const res = await staleWhileRevalidate(new Request(URL_A), {
      cache,
      fetch: fetchSpy,
    });
    expect(res.status).toBe(500);
    expect(store.has(URL_A)).toBe(false);
  });

  it("throws if neither cache nor network are available", async () => {
    const { cache } = makeCache();
    const fetchSpy = vi.fn().mockRejectedValue(new Error("boom"));
    const onError = vi.fn();
    await expect(
      staleWhileRevalidate(new Request(URL_A), {
        cache,
        fetch: fetchSpy,
        onError,
      }),
    ).rejects.toThrow(/no cached or fresh/);
    expect(onError).toHaveBeenCalled();
  });

  it("does not throw when network fails but cache hit exists", async () => {
    const { cache } = makeCache({
      [URL_A]: new Response("stale", { status: 200 }),
    });
    const fetchSpy = vi.fn().mockRejectedValue(new Error("net"));
    const onError = vi.fn();
    const res = await staleWhileRevalidate(new Request(URL_A), {
      cache,
      fetch: fetchSpy,
      onError,
    });
    expect(await res.text()).toBe("stale");
    // Allow the background rejection to flush.
    await new Promise((r) => setTimeout(r, 0));
    expect(onError).toHaveBeenCalled();
  });
});

describe("networkFirst", () => {
  it("returns a fast network response and caches it", async () => {
    const { cache, store } = makeCache();
    const fetchSpy = vi
      .fn()
      .mockResolvedValue(new Response("fresh", { status: 200 }));
    const res = await networkFirst(
      new Request(URL_A),
      { cache, fetch: fetchSpy },
      { timeoutMs: 1000 },
    );
    expect(await res.text()).toBe("fresh");
    expect(await store.get(URL_A)!.text()).toBe("fresh");
  });

  it("falls back to cache on network error", async () => {
    const { cache } = makeCache({
      [URL_A]: new Response("stale", { status: 200 }),
    });
    const fetchSpy = vi.fn().mockRejectedValue(new Error("offline"));
    const res = await networkFirst(
      new Request(URL_A),
      { cache, fetch: fetchSpy },
      { timeoutMs: 1000 },
    );
    expect(await res.text()).toBe("stale");
  });

  it("falls back to cache on timeout", async () => {
    const { cache } = makeCache({
      [URL_A]: new Response("stale", { status: 200 }),
    });
    const fetchSpy = vi.fn(
      () => new Promise<Response>(() => {}), // never resolves
    );
    const res = await networkFirst(
      new Request(URL_A),
      { cache, fetch: fetchSpy },
      { timeoutMs: 10 },
    );
    expect(await res.text()).toBe("stale");
  });

  it("rethrows the network error when nothing is cached", async () => {
    const { cache } = makeCache();
    const err = new Error("dead");
    const fetchSpy = vi.fn().mockRejectedValue(err);
    await expect(
      networkFirst(
        new Request(URL_A),
        { cache, fetch: fetchSpy },
        { timeoutMs: 1000 },
      ),
    ).rejects.toThrow("dead");
  });

  it("throws a generic error on timeout with empty cache", async () => {
    const { cache } = makeCache();
    const fetchSpy = vi.fn(() => new Promise<Response>(() => {}));
    await expect(
      networkFirst(
        new Request(URL_A),
        { cache, fetch: fetchSpy },
        { timeoutMs: 5 },
      ),
    ).rejects.toThrow();
  });
});

describe("cacheOnly", () => {
  it("returns a cached response when available", async () => {
    const { cache } = makeCache({
      [URL_A]: new Response("c", { status: 200 }),
    });
    const res = await cacheOnly(new Request(URL_A), { cache });
    expect(res).toBeDefined();
    expect(await res!.text()).toBe("c");
  });

  it("returns undefined on a miss", async () => {
    const { cache } = makeCache();
    expect(await cacheOnly(new Request(URL_A), { cache })).toBeUndefined();
  });
});
