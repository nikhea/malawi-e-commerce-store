import type { Requester } from "../http";
import type { CreatePaymentIntentInput, Payment } from "../types";

/**
 * /api/v1/payments — JWT required. Idempotent: a pending intent for the
 * same order is reused instead of duplicated. Webhooks settle orders
 * server-side; poll `orders.get(id)` for status transitions.
 */
export class PaymentsApi {
  constructor(private readonly request: Requester) {}

  createIntent(input: CreatePaymentIntentInput): Promise<Payment> {
    return this.request("POST", "/payments/intents", { body: input });
  }
}
