import type { Requester } from "../http";
import type { AddCartItemInput, Cart, SetQtyInput } from "../types";

/**
 * /api/v1/cart — JWT required, one cart per user (created lazily on first
 * GET). Adding the same product+variant merges lines server-side.
 */
export class CartApi {
  constructor(private readonly request: Requester) {}

  get(): Promise<Cart> {
    return this.request("GET", "/cart");
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
