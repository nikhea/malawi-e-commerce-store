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

describe("catalog filters and pagination", () => {
  const META = {
    page: 2,
    limit: 10,
    offset: 10,
    total: 41,
    total_pages: 5,
    has_next: true,
    has_prev: true,
  };

  it("serializes every product filter, keeping false values", async () => {
    const { calls, impl } = stubFetch(() => JSON_RESPONSE({ data: [] }));
    const store = new MalawiStoreClient({ fetchImpl: impl });
    await store.products.list({
      q: "dress",
      category_slug: "dresses",
      min_price_cents: 1000,
      max_price_cents: 5000,
      in_stock: true,
      size: "XL",
      min_rating: 4,
      created_after: "2026-01-01",
      created_before: "2026-12-31",
      active_only: false,
      sort: "price_desc",
      page: 2,
      limit: 10,
    });

    const url = new URL(calls[0].url);
    expect(url.pathname).toBe("/api/v1/products");
    expect(url.searchParams.get("q")).toBe("dress");
    expect(url.searchParams.get("category_slug")).toBe("dresses");
    expect(url.searchParams.get("min_price_cents")).toBe("1000");
    expect(url.searchParams.get("max_price_cents")).toBe("5000");
    expect(url.searchParams.get("in_stock")).toBe("true");
    expect(url.searchParams.get("size")).toBe("XL");
    expect(url.searchParams.get("min_rating")).toBe("4");
    expect(url.searchParams.get("created_after")).toBe("2026-01-01");
    expect(url.searchParams.get("created_before")).toBe("2026-12-31");
    expect(url.searchParams.get("active_only")).toBe("false");
    expect(url.searchParams.get("sort")).toBe("price_desc");
    expect(url.searchParams.get("page")).toBe("2");
    expect(url.searchParams.get("limit")).toBe("10");
  });

  it("listPage returns rows and meta from the paged envelope", async () => {
    const row = { id: "p1", name: "Wrap Dress" };
    const { impl } = stubFetch(() => JSON_RESPONSE({ data: [row], meta: META }));
    const store = new MalawiStoreClient({ fetchImpl: impl });
    await expect(store.products.listPage({ page: 2, limit: 10 })).resolves.toEqual({
      data: [row],
      meta: META,
    });
  });

  it("categories list passes filters and listPage exposes meta", async () => {
    const { calls, impl } = stubFetch(() =>
      JSON_RESPONSE({ data: [{ id: "c1", name: "Dresses" }], meta: META }),
    );
    const store = new MalawiStoreClient({ fetchImpl: impl });
    const page = await store.categories.listPage({
      q: "dress",
      root_only: true,
      has_products: true,
      sort: "product_count_desc",
      page: 2,
      limit: 10,
    });

    const url = new URL(calls[0].url);
    expect(url.searchParams.get("q")).toBe("dress");
    expect(url.searchParams.get("root_only")).toBe("true");
    expect(url.searchParams.get("has_products")).toBe("true");
    expect(url.searchParams.get("sort")).toBe("product_count_desc");
    expect(page.meta.total).toBe(41);
    expect(page.data).toHaveLength(1);
  });
});
