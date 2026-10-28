import { Http, type HttpOptions, type Requester } from "./http";
import { AuthApi } from "./resources/auth";
import { UsersApi } from "./resources/users";
import { CategoriesApi } from "./resources/categories";
import { ProductsApi, VariantsApi } from "./resources/products";
import { InventoryApi } from "./resources/inventory";
import { WishlistApi } from "./resources/wishlist";
import { CartApi } from "./resources/cart";
import { OrdersApi } from "./resources/orders";
import { PaymentsApi } from "./resources/payments";
import { ReviewsApi } from "./resources/reviews";

export type { HttpOptions } from "./http";

/**
 * Typed client for the Malawi E-Commerce Store API (`/api/v1`).
 *
 * @example
 * const store = new MalawiStoreClient({
 *   baseUrl: "https://api.mystore.mw",
 *   token: () => session.jwt,
 * });
 *
 * await store.auth.login({ email, password });
 * await store.cart.add({ product_id, qty: 2 });
 * const order = await store.orders.checkout();
 * const payment = await store.payments.createIntent({ order_id: order.id });
 * // then confirm with Stripe.js using payment.client_secret
 */
export class MalawiStoreClient {
  private readonly http: Http;
  private readonly requester: Requester;

  readonly auth: AuthApi;
  readonly users: UsersApi;
  readonly categories: CategoriesApi;
  readonly products: ProductsApi;
  readonly variants: VariantsApi;
  readonly inventory: InventoryApi;
  readonly wishlist: WishlistApi;
  readonly cart: CartApi;
  readonly orders: OrdersApi;
  readonly payments: PaymentsApi;
  readonly reviews: ReviewsApi;

  constructor(options: HttpOptions = {}) {
    this.http = new Http(options);
    this.requester = (method, path, opts) => this.http.request(method, path, opts);

    this.auth = new AuthApi(this.requester);
    this.users = new UsersApi(this.requester);
    this.categories = new CategoriesApi(this.requester);
    this.products = new ProductsApi(this.requester);
    this.variants = new VariantsApi(this.requester);
    this.inventory = new InventoryApi(this.requester);
    this.wishlist = new WishlistApi(this.requester);
    this.cart = new CartApi(this.requester);
    this.orders = new OrdersApi(this.requester);
    this.payments = new PaymentsApi(this.requester);
    this.reviews = new ReviewsApi(this.requester);
  }

  /** GET /health — liveness probe (raw body, no auth, no envelope, unversioned). */
  health(): Promise<{ status: string }> {
    return this.requester("GET", "/health", { raw: true, prefix: false }) as Promise<{ status: string }>;
  }

  /**
   * Swap the stored JWT (e.g. after login/refresh). No-ops when the client
   * was constructed with a token getter.
   */
  setToken(token: string | undefined | null): void {
    this.http.setToken(token);
  }

  hasToken(): boolean {
    return this.http.hasToken();
  }
}
