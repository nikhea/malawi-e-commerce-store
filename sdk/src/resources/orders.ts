import type { Requester } from "../http";
import type {
  AggregateOrdersParams,
  ListOrdersParams,
  Order,
  OrderAggregates,
  OrderListResponse,
} from "../types";

/**
 * /api/v1/orders — JWT required. Checkout takes NO body: it freezes the
 * current cart, reserves stock, and clears the cart atomically.
 */
export class OrdersApi {
  constructor(private readonly request: Requester) {}

  /** 201; creates a pending order from the current cart. */
  checkout(): Promise<Order> {
    return this.request("POST", "/orders/checkout");
  }

  /** Own orders, newest first (paging metadata is dropped). */
  list(params?: ListOrdersParams): Promise<Order[]> {
    return this.request("GET", "/orders", { query: params });
  }

  /** Same filters as `list`, plus the `meta` paging counters. */
  listPage(params?: ListOrdersParams): Promise<OrderListResponse> {
    return this.request("GET", "/orders", { query: params, raw: true });
  }

  /**
   * Order + line-item totals for the current customer. Counts cover every
   * matching row; `top_products` is the top 10 by units sold. Amounts are
   * grouped by currency.
   */
  aggregates(params?: AggregateOrdersParams): Promise<OrderAggregates> {
    return this.request("GET", "/orders/aggregates", { query: params });
  }

  /** Admin: all customers, or one via `params.user_id`. */
  adminAggregates(params?: AggregateOrdersParams): Promise<OrderAggregates> {
    return this.request("GET", "/admin/orders/aggregates", { query: params });
  }

  get(id: string): Promise<Order> {
    return this.request("GET", `/orders/${enc(id)}`);
  }

  /** Only pending orders cancel (409 otherwise); terminal states never move. */
  cancel(id: string): Promise<Order> {
    return this.request("POST", `/orders/${enc(id)}/cancel`);
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}
