import type { Requester } from "../http";
import type {
  CreateProductInput,
  ListProductsParams,
  OpStatus,
  Product,
  ProductDetail,
  UpdateProductInput,
  Variant,
  CreateVariantInput,
  UpdateVariantInput,
} from "../types";

/**
 * /api/v1/products — reads public, writes admin-only.
 */
export class ProductsApi {
  constructor(private readonly request: Requester) {}

  /** Lists SKUs' parent SPUs; call `variants.listFor(productId)` for SKU data. */
  list(params?: ListProductsParams): Promise<Product[]> {
    return this.request("GET", "/products", { query: params });
  }

  get(id: string): Promise<ProductDetail> {
    return this.request("GET", `/products/${enc(id)}`);
  }

  getBySlug(slug: string): Promise<ProductDetail> {
    return this.request("GET", `/products/slug/${enc(slug)}`);
  }

  adminCreate(input: CreateProductInput): Promise<Product> {
    return this.request("POST", "/admin/products", { body: input });
  }

  adminUpdate(id: string, input: UpdateProductInput): Promise<Product> {
    return this.request("PATCH", `/admin/products/${enc(id)}`, { body: input });
  }

  /** 204 No Content. */
  adminDelete(id: string): Promise<void> {
    return this.request("DELETE", `/admin/products/${enc(id)}`);
  }

  /** Multipart upload, field `file`, max 10 MiB. Async (202, Cloudinary). */
  adminUploadImage(id: string, form: FormData): Promise<OpStatus> {
    return this.request("POST", `/admin/products/${enc(id)}/image`, { body: form });
  }
}

/**
 * SKU-level variants. Create nests under the product; update/delete are
 * admin routes keyed by the variant id (`sku`/`product_id` are immutable).
 */
export class VariantsApi {
  constructor(private readonly request: Requester) {}

  listFor(productId: string): Promise<Variant[]> {
    return this.request("GET", `/products/${enc(productId)}/variants`);
  }

  adminCreate(productId: string, input: CreateVariantInput): Promise<Variant> {
    return this.request("POST", `/products/${enc(productId)}/variants`, { body: input });
  }

  adminUpdate(id: string, input: UpdateVariantInput): Promise<Variant> {
    return this.request("PATCH", `/admin/variants/${enc(id)}`, { body: input });
  }

  /** 204 No Content. */
  adminDelete(id: string): Promise<void> {
    return this.request("DELETE", `/admin/variants/${enc(id)}`);
  }
}

function enc(value: string): string {
  return encodeURIComponent(value);
}
