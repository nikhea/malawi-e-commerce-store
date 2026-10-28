import { afterEach, describe, expect, it, vi } from "vitest";
import { MalawiStoreClient } from "./client";
import { ApiError } from "./errors";

/** Minimal fetch stub: unresolved promises track calls for assertions. */
function stubFetch(handler: (url: string, init: RequestInit) => Response | Promise<Response>) {
  const calls: Array<{ url: string; init: RequestInit }> = [];
  const impl = vi.fn(async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return handler(url, init);
  });
  return { calls, impl: impl as unknown as typeof fetch };
}

const JSON_RESPONSE = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", "X-Request-ID": "req-1", ...headers },
  });

describe("MalawiStoreClient transport", () => {
  afterEach(() => vi.restoreAllMocks());

  it("unwraps the standard {data} envelope", async () => {
    const { impl } = stubFetch(() => JSON_RESPONSE({ data: { id: "u1", role: "customer" } }));
    const store = new MalawiStoreClient({ fetchImpl: impl });
    await expect(store.users.me()).resolves.toEqual({ id: "u1", role: "customer" });
  });

  it("sends Bearer token and JSON body", async () => {
    const { calls, impl } = stubFetch(() =>
      JSON_RESPONSE({ data: { token: "t", refresh_token: "r", role: "customer" } }),
    );
    const store = new MalawiStoreClient({ fetchImpl: impl, token: "jwt-abc" });
    await store.auth.login({ email: "a@b.mw", password: "pw" });

    const { url, init } = calls[0];
    expect(url).toBe("http://localhost:8080/api/v1/auth/login");
    expect(init.method).toBe("POST");
    expect((init.headers as Record<string, string>).Authorization).toBe("Bearer jwt-abc");
    expect((init.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
    expect(init.body).toBe(JSON.stringify({ email: "a@b.mw", password: "pw" }));
  });

  it("treats 204 as undefined", async () => {
    const { impl } = stubFetch(() => new Response(null, { status: 204 }));
    const store = new MalawiStoreClient({ fetchImpl: impl });
    await expect(store.cart.clear()).resolves.toBeUndefined();
  });

  it("reads /health raw with no envelope and no auth", async () => {
    const { calls, impl } = stubFetch(() => JSON_RESPONSE({ status: "ok" }));
    const store = new MalawiStoreClient({ fetchImpl: impl, token: "jwt" });
    await expect(store.health()).resolves.toEqual({ status: "ok" });
    expect(calls[0].url).toBe("http://localhost:8080/health");
  });

  it("serializes query params for product listing", async () => {
    const { calls, impl } = stubFetch(() => JSON_RESPONSE({ data: [] }));
    const store = new MalawiStoreClient({ fetchImpl: impl });
    await store.products.list({ category_id: "c1", limit: 50, active_only: undefined });
    expect(calls[0].url).toBe("http://localhost:8080/api/v1/products?category_id=c1&limit=50");
  });

  it("throws ApiError with server code and Retry-After on 429", async () => {
    const { impl } = stubFetch(() =>
      JSON_RESPONSE(
        { error: { code: "RATE_LIMITED", message: "too many requests" } },
        429,
        { "Retry-After": "59" },
      ),
    );
    const store = new MalawiStoreClient({ fetchImpl: impl });
    const err = (await store.auth.login({ email: "x@y.mw", password: "z" }).catch((e: unknown) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("RATE_LIMITED");
    expect(err.status).toBe(429);
    expect(err.retryAfter).toBe(59);
    expect(err.isRateLimited).toBe(true);
    expect(err.requestId).toBe("req-1");
  });

  it("falls back to a sensible message for non-JSON errors", async () => {
    const { impl } = stubFetch(() => new Response("oh no", { status: 502 }));
    const store = new MalawiStoreClient({ fetchImpl: impl });
    const err = (await store.cart.get().catch((e: unknown) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("UNKNOWN");
    expect(err.status).toBe(502);
  });

  it("maps transport failures to NETWORK ApiError", async () => {
    const { impl } = stubFetch(() => {
      throw new Error("connection refused");
    });
    const store = new MalawiStoreClient({ fetchImpl: impl });
    const err = (await store.cart.get().catch((e: unknown) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("NETWORK");
    expect(err.status).toBe(0);
  });
});

describe("MalawiStoreClient wiring", () => {
  it("exposes one API per module", () => {
    const store = new MalawiStoreClient({ fetchImpl: stubFetch(() => new Response()).impl });
    for (const name of ["auth", "users", "categories", "products", "variants", "inventory", "wishlist", "cart", "orders", "payments", "reviews"] as const) {
      expect(store[name], `store.${name}`).toBeTruthy();
    }
  });

  it("setToken swaps the credential", async () => {
    const { calls, impl } = stubFetch(() => JSON_RESPONSE({ data: {} }));
    const store = new MalawiStoreClient({ fetchImpl: impl });
    expect(store.hasToken()).toBe(false);
    store.setToken("t2");
    expect(store.hasToken()).toBe(true);
    await store.wishlist.list();
    expect((calls[0].init.headers as Record<string, string>).Authorization).toBe("Bearer t2");
  });
});
