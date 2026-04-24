import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";
import { apiRequest } from "./client";

// A minimal schema to validate against.
const OkSchema = z.object({ ok: z.boolean() });

afterEach(() => {
  vi.restoreAllMocks();
  // Reset any location mutation we made.
  if (typeof window !== "undefined") {
    window.history.replaceState({}, "", "/");
  }
});

describe("apiRequest", () => {
  it("returns parsed body on 200", async () => {
    global.fetch = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const out = await apiRequest({
      path: "/api/ping",
      schema: OkSchema,
    });
    expect(out).toEqual({ ok: true });
  });

  it("throws ApiError with ErrorResponse fields on 400", async () => {
    global.fetch = vi.fn(async () =>
      new Response(
        JSON.stringify({
          error: { code: "BAD_REQUEST", message: "nope", retryable: false },
          traceId: "abc",
        }),
        {
          status: 400,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    await expect(
      apiRequest({ path: "/api/x", schema: OkSchema }),
    ).rejects.toMatchObject({
      code: "BAD_REQUEST",
      message: "nope",
      status: 400,
    });
  });

  it("redirects to /login on 401", async () => {
    global.fetch = vi.fn(async () =>
      new Response("{}", { status: 401 }),
    );
    // JSDOM doesn't allow direct assignment to window.location.href; stub.
    const original = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: {
        ...original,
        pathname: "/admin/regions",
        search: "",
        href: "http://localhost/admin/regions",
      },
    });
    await expect(
      apiRequest({ path: "/api/x", schema: OkSchema }),
    ).rejects.toBeTruthy();
    expect(window.location.href).toContain("/login");
    // Restore.
    Object.defineProperty(window, "location", {
      writable: true,
      value: original,
    });
  });

  it("does NOT redirect on 401 when noAuthRedirect is set", async () => {
    global.fetch = vi.fn(async () =>
      new Response("{}", { status: 401 }),
    );
    const original = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: {
        ...original,
        pathname: "/admin/regions",
        search: "",
        href: "http://localhost/admin/regions",
      },
    });
    await expect(
      apiRequest({
        path: "/api/auth/me",
        schema: OkSchema,
        noAuthRedirect: true,
      }),
    ).rejects.toBeTruthy();
    expect(window.location.href).toBe("http://localhost/admin/regions");
    Object.defineProperty(window, "location", {
      writable: true,
      value: original,
    });
  });
});
