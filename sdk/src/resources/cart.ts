import type { Requester } from "../http";
import type { AddCartItemInput, Cart, CartResponse, ListCartParams, SetQtyInput } from "../types";

/**
 * /api/v1/cart — JWT required, one cart per user (created lazily on first
 * GET). Adding the same product+variant merges lines server-side.
 */
export class CartApi {
  constructor(private readonly request: Requester) {}

  /** Cart with its first/paged item set (paging metadata is dropped). */
  get(params?: ListCartParams): Promise<Cart> {
    return this.request("GET", "/cart", { query: params });
  }

  /** Same as `get`, plus the `meta` paging counters. */
  getPage(params?: ListCartParams): Promise<CartResponse> {
    return this.request("GET", "/cart", { query: params, raw: true });
  }

  add(input: AddCartItemInput): Promise<Cart> {
    return this.request("POST", "/cart/items", { body: input });
  }

  /** qty 0 removes the line. Path id is the CART LINE id (`CartLine.id`). */
  setQty(lineId: string, input: SetQtyInput): Promise<Cart> {
    return this.request("PATCH", `/cart/items/${enc(lineId)}`, { body: input });
  }

  removeItem(lineId: string): Promise<Cart> {
    return this.request("DELETE", `/cart/items/${enc(lineId)}`);
  }

  /** 204 No Content. */
  clear(): Promise<void> {
    return this.request("DELETE", "/cart");
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}
