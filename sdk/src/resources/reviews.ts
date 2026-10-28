import type { Requester } from "../http";
import type { CreateReviewInput, Review, ReviewSummary, UpdateReviewInput } from "../types";

/**
 * Product reviews. `:productId` scopes create/list; `:reviewId` scopes
 * owner-checked update/delete. One review per user per product (409 on
 * duplicate). Writes never block the catalog read path server-side.
 */
export class ReviewsApi {
  constructor(private readonly request: Requester) {}

  summary(productId: string): Promise<ReviewSummary> {
    return this.request("GET", `/products/${enc(productId)}/reviews`);
  }

  create(productId: string, input: CreateReviewInput): Promise<Review> {
    return this.request("POST", `/products/${enc(productId)}/reviews`, { body: input });
  }

  updateMine(reviewId: string, input: UpdateReviewInput): Promise<Review> {
    return this.request("PATCH", `/reviews/${enc(reviewId)}`, { body: input });
  }

  deleteMine(reviewId: string): Promise<void> {
    return this.request("DELETE", `/reviews/${enc(reviewId)}`);
  }

  adminDelete(reviewId: string): Promise<void> {
    return this.request("DELETE", `/admin/reviews/${enc(reviewId)}`);
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}
