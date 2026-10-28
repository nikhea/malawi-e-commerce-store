import { ApiError } from "./errors";

export type QueryParams = Record<
  string,
  string | number | boolean | undefined | null
>;

export interface RequestOptions {
  body?: unknown;
  query?: QueryParams;
  /**
   * Return the parsed JSON as-is instead of unwrapping the `{"data": …}`
   * envelope (used by GET /health, which has no envelope).
   */
  raw?: boolean;
  /**
   * Prepend the API prefix (default `/api/v1`). Set false for unversioned
   * routes like GET /health.
   */
  prefix?: boolean;
}

/** `method` + absolute-ish path (e.g. `/cart/items`) + options → payload. */
export type Requester = <T>(
  method: "GET" | "POST" | "PATCH" | "PUT" | "DELETE",
  path: string,
  options?: RequestOptions,
) => Promise<T>;

export interface HttpOptions {
  /** API origin, without a path suffix. Default `http://localhost:8080`. */
  baseUrl?: string;
  /**
   * Route prefix the server mounts module APIs under. Default `/api/v1`;
   * pass null to send paths verbatim.
   */
  apiPrefix?: string | null;
  /** JWT as a string, or a getter (re-evaluated per request). */
  token?: string | (() => string | undefined | null);
  /** Fetch implementation override (tests, polyfills). */
  fetchImpl?: typeof fetch;
  /** Abort a hanging request after this long. Default 15000. */
  timeoutMs?: number;
  /** Headers merged into every request. */
  defaultHeaders?: Record<string, string>;
  /** Strip a trailing slash from baseUrl. Default true. */
  trimBaseUrl?: boolean;
}

const DEFAULT_TIMEOUT_MS = 15_000;

export class Http {
  private readonly baseUrl: string;
  private readonly apiPrefix: string | null;
  private readonly token: string | (() => string | undefined | null) | undefined;
  private readonly fetchImpl: typeof fetch;
  private readonly timeoutMs: number;
  private readonly defaultHeaders: Record<string, string>;

  constructor(opts: HttpOptions = {}) {
    const trim = opts.trimBaseUrl !== false;
    this.baseUrl = trim ? trimSlashes(opts.baseUrl ?? "http://localhost:8080") : (opts.baseUrl ?? "");
    if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(this.baseUrl) && this.baseUrl !== "") {
      throw new TypeError(`baseUrl must be absolute, got "${opts.baseUrl}"`);
    }
    this.apiPrefix = opts.apiPrefix === null ? null : (opts.apiPrefix ?? "/api/v1");
    this.token = opts.token;
    this.fetchImpl = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);
    this.timeoutMs = opts.timeoutMs ?? DEFAULT_TIMEOUT_MS;
    this.defaultHeaders = opts.defaultHeaders ?? {};
  }

  /**
   * Update the stored bearer token. No-ops when the client was constructed
   * with a token getter (the getter owns the credential at that point).
   */
  setToken(token: string | undefined | null): void {
    if (typeof this.token === "function") return;
    Object.defineProperty(this, "token", { value: token ?? undefined, writable: false, configurable: true });
  }

  hasToken(): boolean {
    const t = this.resolveToken();
    return typeof t === "string" && t.length > 0;
  }

  private resolveToken(): string | undefined {
    const t = typeof this.token === "function" ? this.token() : this.token;
    return t ? t : undefined;
  }

  async request<T>(
    method: "GET" | "POST" | "PATCH" | "PUT" | "DELETE",
    path: string,
    options: RequestOptions = {},
  ): Promise<T> {
    const url = this.buildUrl(path, options.prefix !== false, options.query);
    const headers: Record<string, string> = { ...this.defaultHeaders, Accept: "application/json" };

    const token = this.resolveToken();
    if (token) headers.Authorization = `Bearer ${token}`;

    let body: FormData | string | undefined;
    if (options.body !== undefined) {
      if (options.body instanceof FormData) {
        body = options.body; // fetch sets the multipart boundary itself
      } else {
        headers["Content-Type"] = "application/json";
        body = JSON.stringify(options.body);
      }
    }

    let res: Response;
    try {
      res = await this.fetchImpl(url, {
        method,
        headers,
        body,
        signal: AbortSignal.timeout(this.timeoutMs),
      });
    } catch (err) {
      throw ApiError.fromPersistenceError(err);
    }

    if (res.status === 204) return undefined as T;

    const text = await res.text();
    let parsed: unknown;
    try {
      parsed = text ? (JSON.parse(text) as unknown) : undefined;
    } catch {
      parsed = undefined;
    }

    if (!res.ok) {
      throw ApiError.fromResponse(res.status, parsed, res.headers, text.slice(0, 500));
    }

    if (options.raw) return parsed as T;

    // Success envelope: {"data": <payload>}. Fall through to the parsed body
    // when there is no envelope key (defensive: GET /health).
    if (parsed !== null && typeof parsed === "object" && "data" in parsed) {
      return (parsed as { data: T }).data;
    }
    return parsed as T;
  }

  private buildUrl(path: string, prefixed: boolean, query?: QueryParams): string {
    let url = this.baseUrl;
    if (prefixed && this.apiPrefix) url += this.apiPrefix;
    url += path.startsWith("/") ? path : `/${path}`;
    if (query) {
      const search = new URLSearchParams();
      for (const [key, value] of Object.entries(query)) {
        if (value === undefined || value === null || value === "") continue;
        search.set(key, String(value));
      }
      const qs = search.toString();
      if (qs) url += (url.includes("?") ? "&" : "?") + qs;
    }
    return url;
  }
}

function trimSlashes(url: string): string {
  return url.replace(/\/+$/, "");
}
