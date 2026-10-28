# @nikhea/malawi-store-sdk

TypeScript SDK for the **Malawi E-Commerce Store** Go API (`/api/v1`). Zero runtime
dependencies — plain `fetch` — so it runs in Node 18+, Next.js, Vite, and browsers.

- **Auth** — register, login, rotating refresh, email verification, password reset
- **Catalog** — categories, products (SPU), variants (SKU), reviews
- **Cart → Orders → Payments** — cart lines, checkout, Stripe PaymentIntents (MWK→USD)
- **Wishlist** — idempotent hearts
- **Admin** — categories/products/variants writes, inventory, user roles, review moderation
- **Malawi helpers** — MWK/tambala money math, the 28 districts by region

## Install

```bash
npm install @nikhea/malawi-store-sdk
```

## Quick start

```ts
import { MalawiStoreClient, formatMWK } from "@nikhea/malawi-store-sdk";

const store = new MalawiStoreClient({
  baseUrl: "https://api.mystore.mw", // default http://localhost:8080
  token: () => session.jwt,          // or string / client.setToken later
});

await store.health(); // { status: "ok" }
```

### Shopper flow (cart → checkout → Stripe.js)

```ts
const { token } = await store.auth.login({ email, password });
store.setToken(token);

await store.wishlist.add(productId);

const cart = await store.cart.add({ product_id, qty: 2 });
console.log(`Subtotal: ${formatMWK(cart.subtotal_cents)}`);

const order = await store.orders.checkout();
const payment = await store.payments.createIntent({ order_id: order.id });

// Hand payment.client_secret to Stripe.js confirmCardPayment.
// Poll store.orders.get(order.id) — the webhook settles pending → paid.
```

### Catalog filters and pagination

`list()` returns just the rows. `listPage()` returns the same rows plus the
API's `meta` counters, so a grid can render paging controls without a second
request.

```ts
const page = await store.products.listPage({
  q: "dress",                 // name, description or slug
  category_slug: "dresses",
  min_price_cents: 1_000_00,
  max_price_cents: 9_000_00,
  in_stock: true,
  size: "XL",                 // active variant name
  min_rating: 4,
  sort: "price_desc",
  page: 2,
  limit: 24,
});

page.data;       // Product[]
page.meta;       // { page, limit, offset, total, total_pages, has_next, has_prev }
```

Category listings take the same paging knobs plus `q`, `parent_id`, `root_only`,
`has_products` and `sort`:

```ts
const categories = await store.categories.listPage({
  has_products: true,
  sort: "product_count_desc",
  limit: 20,
});
```

`page` wins over `offset` when both are sent. The API defaults to 20 rows and
caps a page at 100.

Every variant (both `/products/:id` detail and `/products/:id/variants`) now
carries `available` (on-hand minus reservations) and `in_stock`, so size
pickers can disable sold-out options without extra calls.

### Prices are minor units (MWK tambala)

`12500` = **K125.00**. Helpers included:

```ts
import { mwk, toKwacha, formatMWK, districtsByRegion } from "@nikhea/malawi-store-sdk";

mkw === mwk(125);           // 12500
formatMWK(12500);           // "K125.00"
districtsByRegion("Southern").map(d => d.name); // Blantyre, Zomba, …
```

### Admin

```ts
store.products.adminCreate({ name: "Nkhata Bay Chilli", price_cents: 350000, currency: "MWK" });
store.variants.adminCreate(productId, { name: "500g", sku: "NB-CHILLI-500", price_cents: 350000 });
store.inventory.setStock({ product_id, qty: 40 });
const form = new FormData();
form.append("file", file); // ≤ 10 MiB
await store.products.adminUploadImage(productId, form); // 202, processed async via Cloudinary
```

### Errors

Every failure throws `ApiError` with the server's coded envelope:

```ts
try {
  await store.orders.get(orderId);
} catch (err) {
  if (err instanceof ApiError) {
    err.code;        // NOT_FOUND | VALIDATION_ERROR | UNAUTHORIZED | FORBIDDEN
                     // CONFLICT | RATE_LIMITED | INTERNAL | NETWORK | UNKNOWN
    err.status;      // HTTP status (0 for network failures)
    err.retryAfter;  // seconds, 429 only (auth: 10 req/min/IP)
    err.requestId;   // X-Request-ID, include it in bug reports
  }
}
```

### API surface ↔ SDK resources

| Resource | Routes covered |
|---|---|
| `store.auth` | `/auth/*` (register, login, refresh, logout, email verify, password reset) |
| `store.users` | `/users/me`, `/users/:id`, `/admin/users/*` |
| `store.categories` | `/categories`, `/categories/slug/:slug`, `/admin/categories/*` |
| `store.products` | `/products`, `/products/slug/:slug`, `/admin/products/*` |
| `store.variants` | `/products/:id/variants`, `/admin/variants/*` |
| `store.inventory` | `/admin/inventory/*` (stock, reserve, release, confirm) |
| `store.wishlist` | `/wishlist` (note: `remove()` takes the **product** id) |
| `store.cart` | `/cart`, `/cart/items/:id` (line ids) |
| `store.orders` | `/orders/checkout`, `/orders`, `/orders/:id`, `/orders/:id/cancel` |
| `store.payments` | `/payments/intents` |
| `store.reviews` | `/products/:id/reviews`, `/reviews/:id`, `/admin/reviews/:id` |
| `store.health` | `/health` (raw, no envelope) |

## Development

```bash
npm install
npm run typecheck   # tsc --noEmit
npm test            # vitest
npm run build       # tsup → dist (esm + cjs + d.ts)
```

Publishing is manual: `npm publish` runs typecheck → tests → build via `prepublishOnly`.

> The SDK mirrors the Go DTOs field-for-field (snake_case on the wire). When a
> module changes its `dto/` types, update `src/types.ts` in the same commit.
