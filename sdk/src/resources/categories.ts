import type { Requester } from "../http";
import type {
  Category,
  CreateCategoryInput,
  OpStatus,
  UpdateCategoryInput,
} from "../types";

/**
 * /api/v1/categories — reads public, writes admin-only.
 * Image upload is async (Cloudinary, 202 Accepted).
 */
export class CategoriesApi {
  constructor(private readonly request: Requester) {}

  list(): Promise<Category[]> {
    return this.request("GET", "/categories");
  }

  get(id: string): Promise<Category> {
    return this.request("GET", `/categories/${enc(id)}`);
  }

  getBySlug(slug: string): Promise<Category> {
    return this.request("GET", `/categories/slug/${enc(slug)}`);
  }

  adminCreate(input: CreateCategoryInput): Promise<Category> {
    return this.request("POST", "/admin/categories", { body: input });
  }

  adminUpdate(id: string, input: UpdateCategoryInput): Promise<Category> {
    return this.request("PATCH", `/admin/categories/${enc(id)}`, { body: input });
  }

  /** 204 No Content. */
  adminDelete(id: string): Promise<void> {
    return this.request("DELETE", `/admin/categories/${enc(id)}`);
  }

  /**
   * Multipart upload, field `file`, max 10 MiB. Async processing —
   * returns `{"status": "queued"}` when accepted.
   */
  adminUploadImage(id: string, form: FormData): Promise<OpStatus> {
    return this.request("POST", `/admin/categories/${enc(id)}/image`, { body: form });
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}
