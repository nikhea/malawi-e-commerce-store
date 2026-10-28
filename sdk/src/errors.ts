import type { ErrorResponse } from "./types";

export type ErrorCode =
  | "NOT_FOUND" //        404
  | "VALIDATION_ERROR" // 400
  | "UNAUTHORIZED" //     401
  | "FORBIDDEN" //        403
  | "CONFLICT" //         409
  | "RATE_LIMITED" //     429
  | "INTERNAL" //         500
  | "NETWORK" //          request never reached the API
  | "UNKNOWN"; //         non-JSON / unparseable error body

const STATUS_OVERRIDES: Record<number, ErrorCode> = {
  400: "VALIDATION_ERROR",
  401: "UNAUTHORIZED",
  403: "FORBIDDEN",
  404: "NOT_FOUND",
  409: "CONFLICT",
  429: "RATE_LIMITED",
  500: "INTERNAL",
};

function messageFor(status: number): string {
  switch (STATUS_OVERRIDES[status]) {
    case "VALIDATION_ERROR":
      return "The request was invalid";
    case "UNAUTHORIZED":
      return "Authentication required";
    case "FORBIDDEN":
      return "You do not have permission for this action";
    case "NOT_FOUND":
      return "Resource not found";
    case "CONFLICT":
      return "The request conflicts with existing state";
    case "RATE_LIMITED":
      return "Too many requests";
    default:
      return `Request failed with status ${status}`;
  }
}

/**
 * Raised for every non-2xx API response, transport failures
 * (`code: "NETWORK"`), and unparseable bodies (`code: "UNKNOWN"`).
 */
export class ApiError extends Error {
  /** HTTP status, or 0 for transport-level (NETWORK) failures. */
  readonly status: number;
  /** Server error code from the standardized envelope (pkg/apperr). */
  readonly code: ErrorCode;
  /** Seconds until the rate-limit window resets (429 only, from Retry-After). */
  readonly retryAfter?: number;
  /** Server-assigned X-Request-ID, echoed on every response. */
  readonly requestId?: string;

  constructor(
    code: ErrorCode,
    message: string,
    opts: { status?: number; retryAfter?: number; requestId?: string; cause?: unknown } = {},
  ) {
    super(message, { cause: opts.cause });
    this.name = "ApiError";
    this.code = code;
    this.status = opts.status ?? 0;
    this.retryAfter = opts.retryAfter;
    this.requestId = opts.requestId;
  }

  /** True when the server rate-limited the request (auth 10/min, webhooks 120/min). */
  get isRateLimited(): boolean {
    return this.code === "RATE_LIMITED";
  }

  /** True when the JWT is missing/expired/invalidated (rotated refresh reuse). */
  get isUnauthorized(): boolean {
    return this.code === "UNAUTHORIZED";
  }

  /** True when a required resource (product, order, review) does not exist. */
  get isNotFound(): boolean {
    return this.code === "NOT_FOUND";
  }

  static fromPersistenceError(err: unknown): ApiError {
    return new ApiError("NETWORK", err instanceof Error ? err.message : String(err), { status: 0, cause: err });
  }

  /** Build from a non-2xx response pair (status + parsed-or-undefined body). */
  static fromResponse(
    status: number,
    body: unknown,
    headers: Headers | null,
    fallback: string,
  ): ApiError {
    let code: ErrorCode = STATUS_OVERRIDES[status] ?? (normalize(body)?.code as ErrorCode) ?? "UNKNOWN";
    let message = fallback || messageFor(status);
    const detail = normalize(body);
    if (detail?.message) message = detail.message;
    return new ApiError(code, message, {
      status,
      retryAfter: parseRetryAfter(headers),
      requestId: headers?.get("x-request-id") ?? undefined,
    });
  }
}

function parseRetryAfter(headers: Headers | null): number | undefined {
  const raw = headers?.get("retry-after");
  if (!raw) return undefined;
  const seconds = Number.parseFloat(raw);
  return Number.isFinite(seconds) ? seconds : undefined;
}

function normalize(body: unknown): ErrorResponse["error"] | undefined {
  if (body === null || typeof body !== "object" || !("error" in body)) return undefined;
  const err = (body as { error: unknown }).error;
  if (err === null || typeof err !== "object") return undefined;
  const detail = err as Partial<ErrorResponse["error"]>;
  return {
    code: typeof detail.code === "string" ? detail.code : "UNKNOWN",
    message: typeof detail.message === "string" ? detail.message : "",
  };
}
