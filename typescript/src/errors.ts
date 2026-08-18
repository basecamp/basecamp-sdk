/**
 * Structured error types for the Basecamp SDK.
 *
 * Provides typed errors with error codes, hints, and exit codes
 * for CLI-friendly error handling.
 *
 * @example
 * ```ts
 * import { BasecampError, Errors } from "@37signals/basecamp";
 *
 * try {
 *   await client.todos.get(todoId);
 * } catch (err) {
 *   if (err instanceof BasecampError) {
 *     if (err.code === 'not_found') {
 *       console.log('Todo not found');
 *     } else if (err.retryable) {
 *       // Implement retry logic
 *     }
 *     process.exit(err.exitCode);
 *   }
 *   throw err;
 * }
 * ```
 */

/**
 * Maximum length for error messages to prevent information leakage and memory issues.
 */
const MAX_ERROR_MESSAGE_LENGTH = 500;

/**
 * Truncates a string to maxLen characters, appending "..." if truncated.
 * Exported for internal use in error messages that embed caller-supplied
 * values (e.g. URLs); not part of the public API surface.
 */
export function truncateErrorMessage(s: string, maxLen: number = MAX_ERROR_MESSAGE_LENGTH): string {
  if (s.length <= maxLen) return s;
  return s.slice(0, maxLen - 3) + "...";
}

/**
 * Error codes for categorizing Basecamp API errors.
 */
export type ErrorCode =
  | "auth_required"
  | "forbidden"
  | "not_found"
  | "rate_limit"
  | "validation"
  | "ambiguous"
  | "network"
  | "api_error"
  | "usage"
  | "limit_exceeded";

/**
 * Options for creating a BasecampError.
 */
export interface BasecampErrorOptions {
  /** User-friendly hint for resolving the error */
  hint?: string;
  /** HTTP status code that caused the error */
  httpStatus?: number;
  /** Whether the operation can be retried */
  retryable?: boolean;
  /** Original error that caused this error */
  cause?: Error;
  /** Number of seconds to wait before retrying (for rate limits) */
  retryAfter?: number;
  /** Request ID from the server for debugging */
  requestId?: string;
  /** Field-keyed validation messages from a 400/422 body, wrapped ({"errors": {field: [messages]}}) or bare ({field: [messages]}) */
  fieldErrors?: Record<string, string[]>;
}

/**
 * Exit codes for CLI applications, mapped from error codes.
 * Follows common Unix conventions where possible.
 */
const EXIT_CODES: Record<ErrorCode, number> = {
  usage: 1, // Usage error (invalid arguments, config)
  not_found: 2, // Not found
  auth_required: 3, // Authentication error
  forbidden: 4, // Permission denied
  rate_limit: 5, // Rate limited
  network: 6, // Network error
  api_error: 7, // API error
  ambiguous: 8, // Multiple matches found
  validation: 9, // Validation error (HTTP 400/422)
  limit_exceeded: 10, // Account limit reached (HTTP 507)
};

/**
 * Structured error class for Basecamp API errors.
 *
 * Extends the native Error class with additional metadata
 * useful for error handling, logging, and CLI exit codes.
 */
export class BasecampError extends Error {
  /** Error category code */
  readonly code: ErrorCode;

  /** User-friendly hint for resolving the error */
  readonly hint?: string;

  /** HTTP status code that caused the error */
  readonly httpStatus?: number;

  /** Whether the operation can be retried */
  readonly retryable: boolean;

  /** Number of seconds to wait before retrying (for rate limits) */
  readonly retryAfter?: number;

  /** Request ID from the server for debugging */
  readonly requestId?: string;

  /**
   * Field-keyed validation messages from a 422 body of the form
   * `{"errors": {"field": ["msg", ...]}}` — the Rails RecordInvalid rendering —
   * or the same map with no wrapper at all (`{"field": ["msg", ...]}`), which
   * some controllers emit. Undefined for every other error shape. The flattened
   * form is also folded into `message`; this slot preserves the raw, untruncated
   * per-field messages.
   */
  readonly fieldErrors?: Record<string, string[]>;

  /** Original error that caused this error (ES2022+) */
  declare readonly cause?: Error;

  constructor(code: ErrorCode, message: string, options?: BasecampErrorOptions) {
    super(message);
    this.name = "BasecampError";
    this.code = code;
    this.hint = options?.hint;
    this.httpStatus = options?.httpStatus;
    this.retryable = options?.retryable ?? false;
    this.retryAfter = options?.retryAfter;
    this.requestId = options?.requestId;
    this.fieldErrors = options?.fieldErrors;

    // Set cause if provided (ES2022+)
    if (options?.cause) {
      this.cause = options.cause;
    }

    // Maintain proper stack trace in V8 (Node.js, Chrome)
    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, BasecampError);
    }
  }

  /**
   * Exit code for CLI applications.
   * Different error types map to different exit codes.
   */
  get exitCode(): number {
    return EXIT_CODES[this.code];
  }

  /**
   * Returns a JSON-serializable representation of the error.
   */
  toJSON(): Record<string, unknown> {
    return {
      name: this.name,
      code: this.code,
      message: this.message,
      hint: this.hint,
      httpStatus: this.httpStatus,
      retryable: this.retryable,
      retryAfter: this.retryAfter,
      requestId: this.requestId,
      fieldErrors: this.fieldErrors,
    };
  }
}

/**
 * Factory functions for creating common error types.
 *
 * @example
 * ```ts
 * // Create an auth error
 * throw Errors.auth("Token expired");
 *
 * // Create a not found error
 * throw Errors.notFound("Todo", 12345);
 *
 * // Create a rate limit error with retry info
 * throw Errors.rateLimit(30);
 * ```
 */
export const Errors = {
  /**
   * Creates an authentication error (401).
   */
  auth: (hint?: string, cause?: Error): BasecampError =>
    new BasecampError("auth_required", "Authentication required", {
      hint: hint ?? "Check your access token or refresh it if expired",
      httpStatus: 401,
      cause,
    }),

  /**
   * Creates a forbidden error (403).
   */
  forbidden: (hint?: string, cause?: Error): BasecampError =>
    new BasecampError("forbidden", "Access denied", {
      hint: hint ?? "You do not have permission to access this resource",
      httpStatus: 403,
      cause,
    }),

  /**
   * Creates a not found error (404).
   */
  notFound: (resource: string, id?: number | string): BasecampError =>
    new BasecampError(
      "not_found",
      id ? `${resource} ${id} not found` : `${resource} not found`,
      { httpStatus: 404 }
    ),

  /**
   * Creates a rate limit error (429).
   */
  rateLimit: (retryAfter?: number, cause?: Error): BasecampError =>
    new BasecampError("rate_limit", "Rate limit exceeded", {
      retryable: true,
      httpStatus: 429,
      hint: retryAfter ? `Retry after ${retryAfter} seconds` : "Please slow down requests",
      retryAfter,
      cause,
    }),

  /**
   * Creates a validation error (400/422).
   */
  validation: (message: string, hint?: string): BasecampError =>
    new BasecampError("validation", message, {
      httpStatus: 400,
      hint,
    }),

  /**
   * Creates an ambiguous match error.
   */
  ambiguous: (resource: string, matches: string[]): BasecampError => {
    const hint = matches.length > 0 && matches.length <= 5
      ? `Did you mean: ${matches.join(", ")}`
      : "Be more specific";
    return new BasecampError("ambiguous", `Ambiguous ${resource}`, { hint });
  },

  /**
   * Creates a network error.
   */
  network: (message: string, cause?: Error): BasecampError =>
    new BasecampError("network", message, {
      retryable: true,
      hint: "Check your network connection",
      cause,
    }),

  /**
   * Creates a usage error — invalid arguments, missing configuration.
   */
  usage: (message: string, hint?: string): BasecampError =>
    new BasecampError("usage", message, { hint }),

  /**
   * Creates a generic API error.
   */
  apiError: (
    message: string,
    httpStatus?: number,
    options?: Pick<BasecampErrorOptions, "hint" | "retryable" | "requestId" | "cause">
  ): BasecampError =>
    new BasecampError("api_error", message, {
      httpStatus,
      ...options,
    }),
};

/**
 * Creates a BasecampError from an HTTP response.
 * Useful for mapping API responses to typed errors.
 */
export async function errorFromResponse(
  response: Response,
  requestId?: string
): Promise<BasecampError> {
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    // Body is not JSON or empty, use status text
    body = undefined;
  }
  return errorFromParsedBody(response, body, requestId);
}

/**
 * Creates a BasecampError from an HTTP response whose body has already been
 * read and parsed (e.g. by openapi-fetch, which consumes the error body before
 * the service layer sees the response — re-reading it would throw and lose the
 * server's message).
 */
export function errorFromParsedBody(
  response: Response,
  body: unknown,
  requestId?: string
): BasecampError {
  const httpStatus = response.status;
  const retryAfter = parseRetryAfter(response.headers.get("Retry-After"));

  // Try to extract error message from the parsed body
  let message = response.statusText || "Request failed";
  let serverMessage: string | undefined;
  let hint: string | undefined;
  let fieldErrors: Record<string, string[]> | undefined;

  if (typeof body === "object" && body !== null) {
    if ("error" in body && typeof body.error === "string") {
      // Truncate error messages to prevent information leakage and unbounded memory growth
      serverMessage = truncateErrorMessage(body.error);
      message = serverMessage;
    } else if ("message" in body && typeof body.message === "string") {
      // SPEC §6 step 4: "message" is the fallback for APIs that use it
      // instead of "error".
      serverMessage = truncateErrorMessage(body.message);
      message = serverMessage;
    }
    if ("error_description" in body && typeof body.error_description === "string") {
      hint = truncateErrorMessage(body.error_description);
    }
    if (httpStatus === 400 || httpStatus === 422) {
      fieldErrors = parseFieldErrors(body);
      if (fieldErrors) {
        const flat = flattenFieldErrors(fieldErrors);
        // Appended in parentheses after a top-level message, standing alone
        // otherwise; truncated after flattening so the tail is capped too.
        message = truncateErrorMessage(serverMessage ? `${serverMessage} (${flat})` : flat);
      }
    }
  }

  switch (httpStatus) {
    case 401:
      return new BasecampError("auth_required", message, { httpStatus, hint, requestId });
    case 403:
      return new BasecampError("forbidden", message, { httpStatus, hint, requestId });
    case 404:
      return new BasecampError("not_found", message, { httpStatus, hint, requestId });
    case 429:
      return new BasecampError("rate_limit", message, {
        httpStatus,
        retryable: true,
        retryAfter,
        hint: retryAfter ? `Retry after ${retryAfter} seconds` : hint,
        requestId,
      });
    case 400:
    case 422:
      return new BasecampError("validation", message, { httpStatus, hint, requestId, fieldErrors });
    case 507:
      // A 5xx status carrying a client fact: the account is out of storage, or
      // at its webhook ceiling. Retrying cannot satisfy it, so this must be
      // decided before the 5xx catch-all below.
      return new BasecampError("limit_exceeded", message, {
        httpStatus,
        retryable: false,
        hint,
        requestId,
      });
    default:
      // 5xx errors are retryable
      const retryable = httpStatus >= 500 && httpStatus < 600;
      return new BasecampError("api_error", message, {
        httpStatus,
        retryable,
        hint,
        requestId,
      });
  }
}

/**
 * Extracts the field-keyed validation errors map from a parsed error body —
 * the Rails RecordInvalid rendering `{"errors": {"field": ["msg", ...]}}`.
 * Entries whose value is not an array are skipped, non-string elements are
 * dropped, and a map with no usable entries is treated as absent (undefined).
 *
 * A body with no `errors` key falls through to `parseBareFieldErrors` for the
 * unwrapped rendering.
 */
function parseFieldErrors(body: object): Record<string, string[]> | undefined {
  const errors = (body as { errors?: unknown }).errors;
  if (typeof errors !== "object" || errors === null || Array.isArray(errors)) {
    return parseBareFieldErrors(body);
  }
  // Null prototype so an untrusted field name like "__proto__" becomes an
  // ordinary own property instead of invoking the legacy prototype setter
  // (which would both drop the entry and mutate the map's prototype).
  const fieldErrors: Record<string, string[]> = Object.create(null);
  let found = false;
  for (const [field, value] of Object.entries(errors)) {
    if (!Array.isArray(value)) continue;
    const messages = value.filter((m): m is string => typeof m === "string");
    if (messages.length === 0) continue;
    fieldErrors[field] = messages;
    found = true;
  }
  return found ? fieldErrors : undefined;
}

/**
 * Extracts an unwrapped field map — the `render json: @webhook.errors`
 * rendering, where the whole body is `{"field": ["msg", ...]}`. The gate is
 * all-or-nothing by design (SPEC §6 step 2): with no `errors` key to declare
 * intent, only shape distinguishes a field map from any other JSON object, so
 * a single non-conforming member means this is not one. Returns undefined
 * unless every member is a non-empty array of non-empty strings.
 */
function parseBareFieldErrors(body: object): Record<string, string[]> | undefined {
  if (Array.isArray(body)) return undefined;
  // Only "errors" is structurally reserved (it belongs to step 1). "error" and
  // "message" are not excluded by name: a flat body carries them as strings,
  // which the shape gate below already rejects.
  if ("errors" in body) return undefined;

  const entries = Object.entries(body);
  if (entries.length === 0) return undefined;

  // Null prototype, for the same reason as the wrapped map: field names are
  // data, and "__proto__" must land as an ordinary own property.
  const fieldErrors: Record<string, string[]> = Object.create(null);
  for (const [field, value] of entries) {
    if (!Array.isArray(value) || value.length === 0) return undefined;
    if (!value.every((m) => typeof m === "string" && m.length > 0)) return undefined;
    fieldErrors[field] = value as string[];
  }
  return fieldErrors;
}

/**
 * Flattens a field-keyed errors map as "field: msg1; msg2, other: msg" —
 * fields sorted lexicographically, a field's messages joined with "; ",
 * fields joined with ", ". This shape is shared by all six SDKs; change it
 * everywhere or nowhere.
 */
function flattenFieldErrors(fieldErrors: Record<string, string[]>): string {
  return Object.entries(fieldErrors)
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([field, messages]) => `${field}: ${messages.join("; ")}`)
    .join(", ");
}

/**
 * Parses the Retry-After header value (SPEC §6, "Retry-After Parsing
 * Algorithm"): integer seconds when > 0, else an RFC 7231 HTTP-date reduced to
 * `max(0, date - now())` seconds when that is > 0, else `undefined` — which
 * means "no server-directed delay", and every caller falls through to the
 * backoff formula.
 *
 * This is the SDK's ONE implementation of that algorithm. The retry loop
 * (`retry.ts`) and the multipart upload loop (`services/base.ts`) each carried
 * their own `parseInt` copy until #564; both were missing the date branch, and
 * one of them honoured `Retry-After: 0` as a zero-millisecond delay, which
 * collapsed the backoff outright. Exported for those two callers rather than
 * re-derived — it is deliberately NOT re-exported from `index.ts`, so this
 * stays off the package's public surface.
 */
export function parseRetryAfter(value: string | null): number | undefined {
  if (!value) return undefined;

  // Try parsing as integer (seconds)
  const seconds = parseInt(value, 10);
  if (!isNaN(seconds) && seconds > 0) {
    return seconds;
  }

  // Try parsing as HTTP-date
  const date = Date.parse(value);
  if (!isNaN(date)) {
    const diffMs = date - Date.now();
    if (diffMs > 0) {
      return Math.ceil(diffMs / 1000);
    }
  }

  return undefined;
}

/**
 * Type guard to check if an error is a BasecampError.
 */
export function isBasecampError(error: unknown): error is BasecampError {
  return error instanceof BasecampError;
}

/**
 * Type guard to check if an error is a specific type of BasecampError.
 */
export function isErrorCode(error: unknown, code: ErrorCode): error is BasecampError {
  return isBasecampError(error) && error.code === code;
}
