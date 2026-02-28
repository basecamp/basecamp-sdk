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
 *   await client.todos.get(projectId, todoId);
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
 */
function truncateErrorMessage(s: string, maxLen: number = MAX_ERROR_MESSAGE_LENGTH): string {
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
  | "network"
  | "api_error"
  | "usage";

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
}

/**
 * Exit codes for CLI applications, mapped from error codes.
 * Follows common Unix conventions where possible.
 */
const EXIT_CODES: Record<ErrorCode, number> = {
  usage: 1, // Usage error (invalid arguments, config)
  validation: 1, // General error
  not_found: 2, // Not found
  auth_required: 3, // Authentication error
  forbidden: 4, // Permission denied
  rate_limit: 5, // Rate limited
  network: 6, // Network error
  api_error: 7, // API error
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
   * Creates a validation error (400).
   */
  validation: (message: string, hint?: string): BasecampError =>
    new BasecampError("validation", message, {
      httpStatus: 400,
      hint,
    }),

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
  const httpStatus = response.status;
  const retryAfter = parseRetryAfter(response.headers.get("Retry-After"));

  // Try to extract error message from response body
  let message = response.statusText || "Request failed";
  let hint: string | undefined;

  try {
    const body = await response.json();
    if (typeof body === "object" && body !== null) {
      if ("error" in body && typeof body.error === "string") {
        // Truncate error messages to prevent information leakage and unbounded memory growth
        message = truncateErrorMessage(body.error);
      }
      if ("error_description" in body && typeof body.error_description === "string") {
        hint = truncateErrorMessage(body.error_description);
      }
    }
  } catch {
    // Body is not JSON or empty, use status text
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
      return new BasecampError("validation", message, { httpStatus, hint, requestId });
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
 * Parses the Retry-After header value.
 * Supports both seconds (integer) and HTTP-date formats.
 */
function parseRetryAfter(value: string | null): number | undefined {
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
