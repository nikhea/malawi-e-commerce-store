import type { Requester } from "../http";
import type { ListOrdersParams, Order, OrderListResponse } from "../types";

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
