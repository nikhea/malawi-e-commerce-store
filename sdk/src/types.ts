/**
 * Wire-mirror types for the Malawi E-Commerce Store API.
 * Field names match the Go JSON tags exactly (snake_case).
 */

// ---------------------------------------------------------------------------
// Shared
// ---------------------------------------------------------------------------

export type Role = "admin" | "customer";

export type OrderStatus = "pending" | "paid" | "cancelled";
export type PaymentStatus = "pending" | "succeeded" | "failed" | "cancelled";

/** Standard success envelope payload for simple status acknowledgements. */
export type OpStatus = { status: string };

/** Standard error envelope detail. */
export type ErrorDetail = { code: string; message: string };
export type ErrorResponse = { error: ErrorDetail };

/**
 * Paging counters shipped beside list rows in the `meta` envelope.
 * `offset`/`limit` let a client continue from either paging style.
 */
export type PaginationMeta = {
  page: number;
  limit: number;
  offset: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
};

/** Raw paged envelope: `{ data: rows, meta: counters }`. */
export type Paginated<T> = { data: T[]; meta: PaginationMeta };

/** Batch outcome: only `"ok"` items count as succeeded. */
export type BulkSummary = { total: number; succeeded: number; failed: number };

/** One machine-readable per-item failure inside a 200 batch response. */
export type BulkItemError = { code: string; message: string };

/**
 * One item's outcome. Results are index-aligned with the request so a
 * client can match failures back to its input. `rolled_back` means an
 * atomic batch aborted and nothing was written.
 */
export type BulkResult<T> = {
  index: number;
  id?: string;
  status: "ok" | "error" | "rolled_back";
  data?: T;
  error?: BulkItemError;
  /** Idempotent customer writes: the thing was already there. */
  alreadyExists?: boolean;
  /** Idempotent customer writes: the thing was already gone. */
  alreadyRemoved?: boolean;
  /** dryRun only: the row would have been deleted. */
  wouldDelete?: boolean;
  /** dryRun only: what would cascade or block the delete. */
  blockers?: string[];
};

/** Standard batch payload: per-item results plus counters. */
export type BulkResponse<T> = { results: BulkResult<T>[]; summary: BulkSummary };

/** Payload for delete-style batch items (no entity to return). */
export type BulkEmpty = Record<string, never>;

// ---------------------------------------------------------------------------
// auth — /api/v1/auth (public, rate-limited 10/min/IP)
// ---------------------------------------------------------------------------

/** Returned by register / login / refresh (refresh rotates the pair). */
export type TokenPair = {
  token: string;
  expires_at: string;
  refresh_token: string;
  refresh_expires_at: string;
  user_id: string;
  email: string;
  name: string;
  role: Role;
};

export type RegisterInput = { email: string; name?: string; password: string };
export type LoginInput = { email: string; password: string };
export type RefreshInput = { refresh_token: string };
export type VerifyEmailInput = { email: string; code: string };
export type ForgotPasswordInput = { email: string };
export type ResetPasswordInput = { token: string; new_password: string };

// ---------------------------------------------------------------------------
// users — /api/v1/users (JWT)
// ---------------------------------------------------------------------------

export type User = {
  id: string;
  email: string;
  name: string;
  role: Role;
  email_verified: boolean;
  created_at: string;
};

export type SetRoleInput = { role: Role };

export type UserSort =
  | "newest"
  | "oldest"
  | "name_asc"
  | "name_desc"
  | "email_asc"
  | "email_desc";

/** Admin user directory filters. */
export type ListUsersParams = {
  /** Case-insensitive substring over email or name. */
  q?: string;
  role?: Role;
  email_verified?: boolean;
  /** RFC3339 timestamp or YYYY-MM-DD. */
  created_after?: string;
  created_before?: string;
  sort?: UserSort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  limit?: number;
  offset?: number;
};

export type UserListResponse = Paginated<User>;

// ---------------------------------------------------------------------------
// categories — /api/v1/categories
// ---------------------------------------------------------------------------

export type Category = {
  id: string;
  name: string;
  slug: string;
  description: string;
  parent_id: string | null;
  image_url: string;
  created_at: string;
};

export type CreateCategoryInput = {
  name: string;
  slug?: string;
  description?: string;
  parent_id?: string | null;
  image_url?: string;
};

export type UpdateCategoryInput = {
  name?: string;
  description?: string;
  parent_id?: string;
  clear_parent?: boolean;
};

export type CategorySort =
  | "name_asc"
  | "name_desc"
  | "newest"
  | "oldest"
  | "product_count_asc"
  | "product_count_desc";

export type ListCategoriesParams = {
  /** Case-insensitive substring over name, description and slug. */
  q?: string;
  parent_id?: string;
  root_only?: boolean;
  /** true = categories with active products; false = empty only. */
  has_products?: boolean;
  sort?: CategorySort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  /** Page size (default 12, max 100). */
  limit?: number;
  offset?: number;
};

export type CategoryListResponse = Paginated<Category>;

// ---------------------------------------------------------------------------
// products / variants — /api/v1/products
// ---------------------------------------------------------------------------

export type ProductSort =
  | "name_asc"
  | "name_desc"
  | "price_asc"
  | "price_desc"
  | "newest"
  | "oldest"
  | "rating_asc"
  | "rating_desc"
  | "relevance";

/** Prices are USD cents (12500 = $125.00). */
export type ListProductsParams = {
  /**
   * Full-text search over name, description and slug. Word prefixes match
   * ("cook" finds "cooking"); the API falls back to substring search when
   * full-text finds nothing.
   */
  q?: string;
  /** Restrict to product ids (comma-separated; max MAX_BULK_ITEMS). */
  ids?: string;
  category_id?: string;
  category_slug?: string;
  min_price_cents?: number;
  max_price_cents?: number;
  /** true = in stock; false = out of stock only. */
  in_stock?: boolean;
  /** Active variant name, e.g. XS, M, XL. */
  size?: string;
  /** 0-5; products without reviews are excluded when set above 0. */
  min_rating?: number;
  /** RFC3339 timestamp or YYYY-MM-DD. */
  created_after?: string;
  created_before?: string;
  active_only?: boolean;
  sort?: ProductSort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  /** Page size (default 12, max 100). */
  limit?: number;
  offset?: number;
};

export type ProductListResponse = Paginated<Product>;

export type Product = {
  id: string;
  category_id: string | null;
  category_name: string;
  name: string;
  slug: string;
  description: string;
  price_cents: number;
  currency: string;
  image_url: string;
  is_active: boolean;
  created_at: string;
};

export type VariantView = {
  id: string;
  name: string;
  sku: string;
  price_cents: number;
  is_active: boolean;
  /** Sellable units: on-hand minus active reservations. */
  available: number;
  in_stock: boolean;
};

/** One gallery image. `is_primary` mirrors the product's `image_url`. */
export type ProductImage = {
  id: string;
  url: string;
  position: number;
  is_primary: boolean;
  created_at: string;
};

/** Returned by GET /products/:id and GET /products/slug/:slug. */
export type ProductDetail = {
  product: Product;
  variants: VariantView[];
  /** Uploads still queued are excluded. */
  images: ProductImage[];
};

/** 202 response from the admin multi-file upload. */
export type UploadImagesResult = {
  status: string;
  queued: number;
  failed: number;
};

export type CreateProductInput = {
  category_id?: string | null;
  name: string;
  slug?: string;
  description?: string;
  /** Absolute USD cents per base SKU (12500 = $125.00). */
  price_cents: number;
  currency?: string;
  image_url?: string;
};

export type UpdateProductInput = {
  category_id?: string;
  name?: string;
  description?: string;
  price_cents?: number;
  currency?: string;
  is_active?: boolean;
  clear_category?: boolean;
};

/** POST /admin/products/bulk. Cap MAX_BULK_ITEMS (default 50). */
export type BulkCreateProductsInput = {
  atomic?: boolean;
  dryRun?: boolean;
  items: CreateProductInput[];
};

export type BulkUpdateProductItem = {
  id: string;
  patch: UpdateProductInput;
};

/** PATCH /admin/products/bulk. Duplicate ids reject the whole request. */
export type BulkUpdateProductsInput = {
  atomic?: boolean;
  dryRun?: boolean;
  updates: BulkUpdateProductItem[];
};

/**
 * POST /admin/products/bulk-delete (capped at 25). Products referenced by
 * order history are blocked per item with `CONFLICT`.
 */
export type BulkDeleteProductsInput = {
  ids: string[];
  dryRun?: boolean;
  atomic?: boolean;
};

export type BulkCreateProductsResponse = BulkResponse<Product>;
export type BulkUpdateProductsResponse = BulkResponse<Product>;
export type BulkDeleteProductsResponse = BulkResponse<BulkEmpty>;

export type Variant = {
  id: string;
  product_id: string;
  name: string;
  sku: string;
  price_cents: number;
  is_active: boolean;
  /** Sellable units: on-hand minus active reservations. */
  available: number;
  in_stock: boolean;
  created_at: string;
};

export type CreateVariantInput = {
  name: string;
  sku: string;
  /** Absolute USD cents, not a price delta. */
  price_cents: number;
};

export type UpdateVariantInput = {
  name?: string;
  price_cents?: number;
  is_active?: boolean;
};

/**
 * POST /products/{id}/variants/bulk. Cap MAX_BULK_ITEMS (default 50).
 * The product must exist (whole-request 404); duplicate SKUs reject the
 * whole batch.
 */
export type BulkCreateVariantsInput = {
  atomic?: boolean;
  dryRun?: boolean;
  items: CreateVariantInput[];
};

export type BulkUpdateVariantItem = {
  id: string;
  patch: UpdateVariantInput;
};

/** PATCH /admin/variants/bulk. Duplicate ids reject the whole request. */
export type BulkUpdateVariantsInput = {
  atomic?: boolean;
  dryRun?: boolean;
  updates: BulkUpdateVariantItem[];
};

export type BulkCreateVariantsResponse = BulkResponse<Variant>;
export type BulkUpdateVariantsResponse = BulkResponse<Variant>;

// ---------------------------------------------------------------------------
// inventory — /api/v1/admin/inventory (admin only)
// ---------------------------------------------------------------------------

export type Stock = {
  product_id: string;
  variant_id: string | null;
  on_hand: number;
  reserved: number;
  /** on_hand - reserved */
  available: number;
};

export type SetStockInput = {
  product_id: string;
  variant_id?: string | null;
  qty: number;
};

export type StockLine = { product_id: string; variant_id?: string | null; qty: number };

export type ReserveInput = {
  order_ref: string;
  lines: StockLine[];
  ttl_seconds?: number;
};

// ---------------------------------------------------------------------------
// wishlist — /api/v1/wishlist (JWT)
// ---------------------------------------------------------------------------

export type ProductSummary = {
  id: string;
  name: string;
  slug: string;
  price_cents: number;
  currency: string;
  image_url: string;
};

/**
 * Wishlist item. Note: routes address items by PRODUCT id (DELETE /wishlist/:id
 * takes the product id, matching the wishlist handler's "Product ID" param).
 */
export type WishlistItem = {
  product_id: string;
  product: ProductSummary;
  added_at: string;
};

export type AddWishlistInput = { product_id: string };

export type WishlistSort =
  | "newest"
  | "oldest"
  | "name_asc"
  | "name_desc"
  | "price_asc"
  | "price_desc";

export type ListWishlistParams = {
  /** Case-insensitive substring over product name and slug. */
  q?: string;
  category_id?: string;
  sort?: WishlistSort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  limit?: number;
  offset?: number;
};

export type WishlistListResponse = Paginated<WishlistItem>;

/** POST /wishlist/bulk. Field is camelCase per the API spec. */
export type BulkAddWishlistInput = {
  productIds: string[];
  atomic?: boolean;
};

/** POST /wishlist/bulk-delete. Ids are product ids, not wishlist row ids. */
export type BulkRemoveWishlistInput = {
  ids: string[];
  dryRun?: boolean;
  atomic?: boolean;
};

export type BulkAddWishlistResponse = BulkResponse<WishlistItem>;
export type BulkRemoveWishlistResponse = BulkResponse<BulkEmpty>;

// ---------------------------------------------------------------------------
// cart — /api/v1/cart (JWT)
// ---------------------------------------------------------------------------

export type CartLine = {
  id: string;
  product_id: string;
  variant_id: string | null;
  name: string;
  sku: string;
  unit_price_cents: number;
  currency: string;
  qty: number;
  line_total_cents: number;
};

export type Cart = {
  id: string;
  user_id: string;
  items: CartLine[];
  subtotal_cents: number;
  currency: string;
  updated_at: string;
};

export type AddCartItemInput = {
  product_id: string;
  /** Omit/undefined for the base product price. */
  variant_id?: string | null;
  qty: number;
};

export type SetQtyInput = { qty: number };

/**
 * POST /cart/items/bulk. Capped at 25 lines; duplicate product/variant
 * lines reject the whole request.
 */
export type BulkAddCartItemsInput = {
  atomic?: boolean;
  items: AddCartItemInput[];
};

/**
 * Bulk envelope for a batch add plus the full cart after the batch, so the
 * client never has to re-fetch.
 */
export type BulkAddCartResponse = BulkResponse<CartLine> & { cart: Cart };

export type CartSort =
  | "oldest"
  | "newest"
  | "name_asc"
  | "name_desc"
  | "price_asc"
  | "price_desc"
  | "qty_asc"
  | "qty_desc";

export type ListCartParams = {
  /** Case-insensitive substring over line name and SKU. */
  q?: string;
  product_id?: string;
  variant_id?: string;
  sort?: CartSort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  limit?: number;
  offset?: number;
};

/**
 * Paged cart envelope. `data` is the cart with its current items page;
 * `subtotal_cents` always covers the whole cart. `meta` counts lines.
 */
export type CartResponse = { data: Cart; meta: PaginationMeta };

// ---------------------------------------------------------------------------
// orders — /api/v1/orders (JWT)
// ---------------------------------------------------------------------------

export type OrderLine = {
  product_id: string;
  variant_id: string | null;
  name: string;
  sku: string;
  unit_price_cents: number;
  qty: number;
  line_total_cents: number;
};

export type Order = {
  id: string;
  user_id: string;
  status: OrderStatus;
  subtotal_cents: number;
  currency: string;
  lines: OrderLine[];
  created_at: string;
};

export type OrderSort = "newest" | "oldest" | "total_asc" | "total_desc";

export type ListOrdersParams = {
  status?: OrderStatus;
  currency?: string;
  /** RFC3339 or YYYY-MM-DD. */
  created_after?: string;
  /** RFC3339 or YYYY-MM-DD; a date-only value includes the whole day. */
  created_before?: string;
  min_total_cents?: number;
  max_total_cents?: number;
  sort?: OrderSort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  limit?: number;
  offset?: number;
};

export type OrderListResponse = Paginated<Order>;

// ---------------------------------------------------------------------------
// payments — /api/v1/payments (JWT) + /api/v1/webhooks/stripe (public)
// ---------------------------------------------------------------------------

/** Feed `client_secret` to Stripe.js `confirmCardPayment`. */
export type Payment = {
  id: string;
  order_id: string;
  stripe_intent_id: string;
  client_secret: string;
  /** USD cents actually charged; legacy MWK orders convert server-side. */
  amount_cents: number;
  currency: string;
  /** Source order total in its own minor units (USD cents, or tambala for legacy MWK orders). */
  order_amount_cents: number;
  status: PaymentStatus;
  created_at: string;
};

export type CreatePaymentIntentInput = { order_id: string };

// ---------------------------------------------------------------------------
// reviews — /api/v1/products/:id/reviews (public read, JWT write)
// ---------------------------------------------------------------------------

export type Review = {
  id: string;
  user_id: string;
  author: string;
  product_id: string;
  rating: number;
  title: string;
  body: string;
  created_at: string;
};

export type ReviewSummary = {
  average: number;
  count: number;
  reviews: Review[];
  /** Counters for the filtered review page (average/count are unfiltered). */
  meta: PaginationMeta;
};

export type ReviewSort = "newest" | "oldest" | "highest" | "lowest";

export type ListReviewsParams = {
  /** Case-insensitive substring over title, body and author. */
  q?: string;
  /** Only this rating (1-5). */
  rating?: number;
  sort?: ReviewSort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  limit?: number;
  offset?: number;
};

export type CreateReviewInput = { rating: number; title?: string; body?: string };
export type UpdateReviewInput = { rating?: number; title?: string; body?: string };

/** Catalog context attached to one of the caller's own reviews. */
export type MyReviewProduct = {
  id: string;
  name: string;
  slug: string;
  image_url?: string;
};

/** One row of GET /reviews/me: the review plus its product context. */
export type MyReview = {
  id: string;
  user_id: string;
  product_id: string;
  rating: number;
  title: string;
  body: string;
  created_at: string;
  updated_at: string;
  product: MyReviewProduct;
};

export type ListMyReviewsParams = {
  sort?: ReviewSort;
  /** 1-based page. Wins over `offset` when both are sent. */
  page?: number;
  limit?: number;
  offset?: number;
};

/** `{ data: rows, meta: counters }` — same shape as other paged reads. */
export type MyReviewsResponse = Paginated<MyReview>;
