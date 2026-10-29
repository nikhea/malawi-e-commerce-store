import type { Requester } from "../http";
import type { ListWishlistParams, OpStatus, WishlistItem, WishlistListResponse } from "../types";

/**
 * /api/v1/wishlist — JWT required, idempotent hearts.
 * NOTE: `add` returns the created item; `remove` addresses items by
 * PRODUCT id (the route param is the product id, not an item id).
 */
export class WishlistApi {
  constructor(private readonly request: Requester) {}

  /** Newest first (paging metadata is dropped). */
  list(params?: ListWishlistParams): Promise<WishlistItem[]> {
    return this.request("GET", "/wishlist", { query: params });
  }

  /** Same filters as `list`, plus the `meta` paging counters. */
  listPage(params?: ListWishlistParams): Promise<WishlistListResponse> {
    return this.request("GET", "/wishlist", { query: params, raw: true });
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
