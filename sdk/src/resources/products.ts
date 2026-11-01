import type { Requester } from "../http";
import type {
  CreateProductInput,
  ListProductsParams,
  OpStatus,
  Product,
  ProductDetail,
  ProductImage,
  ProductListResponse,
  UpdateProductInput,
  UploadImagesResult,
  Variant,
  CreateVariantInput,
  UpdateVariantInput,
} from "../types";

/**
 * /api/v1/products — reads public, writes admin-only.
 */
export class ProductsApi {
  constructor(private readonly request: Requester) {}

  /**
   * Lists SKUs' parent SPUs without envelope metadata. Same filters and
   * paging as `listPage`; use this when you only need the rows.
   */
  list(params?: ListProductsParams): Promise<Product[]> {
    return this.request("GET", "/products", { query: params });
  }

  /**
   * Lists a catalog page plus pagination counters (`total`, `total_pages`,
   * `has_next`, …). Best for paginated storefront grids.
   */
  listPage(params?: ListProductsParams): Promise<ProductListResponse> {
    return this.request("GET", "/products", { query: params, raw: true });
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

  /**
   * Lists the gallery, including rows whose upload is still queued
   * (empty `url`). Admin only.
   */
  adminListImages(id: string): Promise<{ images: ProductImage[] }> {
    return this.request("GET", `/admin/products/${enc(id)}/images`);
  }

  /**
   * Multipart upload with repeated `files` entries (field name `files`),
   * max 10 MiB each / 20 per request. One River job per file: the gallery
   * fills in asynchronously (202).
   */
  adminUploadImages(id: string, form: FormData): Promise<UploadImagesResult> {
    return this.request("POST", `/admin/products/${enc(id)}/images`, { body: form });
  }

  /** 204 No Content. Deleting the primary promotes the next ready image. */
  adminDeleteImage(id: string, imageId: string): Promise<void> {
    return this.request("DELETE", `/admin/products/${enc(id)}/images/${enc(imageId)}`);
  }

  /** Points the product's primary `image_url` at one gallery row. */
  adminSetPrimaryImage(id: string, imageId: string): Promise<Product> {
    return this.request("PUT", `/admin/products/${enc(id)}/images/${enc(imageId)}/primary`);
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
