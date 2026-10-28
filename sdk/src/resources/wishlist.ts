import type { Requester } from "../http";
import type { OpStatus, WishlistItem } from "../types";

/**
 * /api/v1/wishlist — JWT required, idempotent hearts.
 * NOTE: `add` returns the created item; `remove` addresses items by
 * PRODUCT id (the route param is the product id, not an item id).
 */
export class WishlistApi {
  constructor(private readonly request: Requester) {}

  list(): Promise<WishlistItem[]> {
    return this.request("GET", "/wishlist");
  }

  add(productId: string): Promise<WishlistItem> {
    return this.request("POST", "/wishlist", { body: { product_id: productId } });
  }

  remove(productId: string): Promise<OpStatus> {
    return this.request("DELETE", `/wishlist/${enc(productId)}`);
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}
