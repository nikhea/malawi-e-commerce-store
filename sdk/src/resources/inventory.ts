import type { Requester } from "../http";
import type { OpStatus, ReserveInput, SetStockInput, Stock } from "../types";

/** Query for GET /admin/inventory/stock (product_id is required server-side). */
export type GetStockParams = { product_id: string; variant_id?: string };

/**
 * /api/v1/admin/inventory — admin only. Reserve/release/confirm are the same
 * hooks the checkout and payment flows call internally; all idempotent
 * except reserve (409 CONFLICT when a line is short).
 */
export class InventoryApi {
  constructor(private readonly request: Requester) {}

  getStock(params: GetStockParams): Promise<Stock> {
    return this.request("GET", "/admin/inventory/stock", {
      query: params as unknown as Record<string, string>,
    });
  }

  /** Idempotent absolute set of on-hand for a product or variant. */
  setStock(input: SetStockInput): Promise<Stock> {
    return this.request("POST", "/admin/inventory/stock", { body: input });
  }

  reserve(input: ReserveInput): Promise<OpStatus> {
    return this.request("POST", "/admin/inventory/reserve", { body: input });
  }

  release(orderRef: string): Promise<OpStatus> {
    return this.request("POST", "/admin/inventory/release", { body: { order_ref: orderRef } });
  }

  confirm(orderRef: string): Promise<OpStatus> {
    return this.request("POST", "/admin/inventory/confirm", { body: { order_ref: orderRef } });
  }
}
