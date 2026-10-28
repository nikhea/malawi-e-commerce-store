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

// ---------------------------------------------------------------------------
// products / variants — /api/v1/products
// ---------------------------------------------------------------------------

/** Minor units: MWK tambala (12500 = K125.00). */
export type ListProductsParams = {
  category_id?: string;
  active_only?: boolean;
  limit?: number;
  offset?: number;
};

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
};

/** Returned by GET /products/:id and GET /products/slug/:slug. */
export type ProductDetail = { product: Product; variants: VariantView[] };

export type CreateProductInput = {
  category_id?: string | null;
  name: string;
  slug?: string;
  description?: string;
  /** Absolute minor units per base SKU (tambala). */
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

export type Variant = {
  id: string;
  product_id: string;
  name: string;
  sku: string;
  price_cents: number;
  is_active: boolean;
  created_at: string;
};

export type CreateVariantInput = {
  name: string;
  sku: string;
  /** Absolute minor units (tambala), not a price delta. */
  price_cents: number;
};

export type UpdateVariantInput = {
  name?: string;
  price_cents?: number;
  is_active?: boolean;
};

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

// ---------------------------------------------------------------------------
// payments — /api/v1/payments (JWT) + /api/v1/webhooks/stripe (public)
// ---------------------------------------------------------------------------

/** Feed `client_secret` to Stripe.js `confirmCardPayment`. */
export type Payment = {
  id: string;
  order_id: string;
  stripe_intent_id: string;
  client_secret: string;
  /** USD cents actually charged (MWK→USD conversion on the server). */
  amount_cents: number;
  currency: string;
  /** Source order amount in MWK tambala. */
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
};

export type CreateReviewInput = { rating: number; title?: string; body?: string };
export type UpdateReviewInput = { rating?: number; title?: string; body?: string };
