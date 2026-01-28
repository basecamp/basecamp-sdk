/**
 * Observability hooks system for the Basecamp SDK.
 *
 * Provides a way to observe API operations and HTTP requests
 * without modifying the core client logic. Hooks compile away
 * when unused (no overhead).
 *
 * @example
 * ```ts
 * import { createBasecampClient, consoleHooks } from "@basecamp/sdk";
 *
 * // Simple console logging
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: "token",
 *   hooks: consoleHooks(),
 * });
 *
 * // Custom metrics
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: "token",
 *   hooks: {
 *     onOperationEnd: (info, result) => {
 *       metrics.recordLatency(info.service, info.operation, result.durationMs);
 *       if (result.error) {
 *         metrics.recordError(info.service, info.operation);
 *       }
 *     },
 *   },
 * });
 * ```
 */

/**
 * Information about a high-level service operation.
 */
export interface OperationInfo {
  /** Service name (e.g., "Todos", "Projects") */
  service: string;
  /** Operation name (e.g., "List", "Get", "Create") */
  operation: string;
  /** Type of resource being accessed */
  resourceType: string;
  /** Whether this operation modifies data */
  isMutation: boolean;
  /** Project ID if the operation is scoped to a project */
  projectId?: number;
  /** Resource ID if the operation targets a specific resource */
  resourceId?: number;
}

/**
 * Information about an HTTP request.
 */
export interface RequestInfo {
  /** HTTP method */
  method: string;
  /** Full request URL */
  url: string;
  /** Current attempt number (1-based) */
  attempt: number;
}

/**
 * Result of an HTTP request.
 */
export interface RequestResult {
  /** HTTP status code */
  statusCode: number;
  /** Request duration in milliseconds */
  durationMs: number;
  /** Whether the response was served from cache */
  fromCache: boolean;
  /** Error if the request failed */
  error?: Error;
}

/**
 * Result of an operation.
 */
export interface OperationResult {
  /** Error if the operation failed */
  error?: Error;
  /** Operation duration in milliseconds */
  durationMs: number;
}

/**
 * Hooks interface for observing SDK operations.
 * All hooks are optional - implement only what you need.
 */
export interface BasecampHooks {
  /**
   * Called when a service operation starts.
   * Use for logging, tracing, or metrics.
   */
  onOperationStart?(info: OperationInfo): void;

  /**
   * Called when a service operation completes (success or failure).
   * Always called, even if onOperationStart threw.
   */
  onOperationEnd?(info: OperationInfo, result: OperationResult): void;

  /**
   * Called when an HTTP request starts.
   * Called for each attempt (including retries).
   */
  onRequestStart?(info: RequestInfo): void;

  /**
   * Called when an HTTP request completes.
   * Called for each attempt (including retries).
   */
  onRequestEnd?(info: RequestInfo, result: RequestResult): void;

  /**
   * Called before a retry attempt.
   * Use for logging retry behavior or implementing custom backoff.
   */
  onRetry?(info: RequestInfo, attempt: number, error: Error, delayMs: number): void;
}

/**
 * Combines multiple hooks into a single hooks object.
 * All hooks are called in order. Errors in hooks are caught and logged.
 *
 * @example
 * ```ts
 * const hooks = chainHooks(
 *   consoleHooks(),
 *   metricsHooks(),
 *   sentryHooks(),
 * );
 * ```
 */
export function chainHooks(...hooks: BasecampHooks[]): BasecampHooks {
  // Filter out undefined/null hooks
  const activeHooks = hooks.filter(Boolean);

  if (activeHooks.length === 0) {
    return {};
  }

  if (activeHooks.length === 1) {
    return activeHooks[0]!;
  }

  return {
    onOperationStart: (info) => {
      for (const h of activeHooks) {
        try {
          h.onOperationStart?.(info);
        } catch (err) {
          console.error("Hook onOperationStart error:", err);
        }
      }
    },

    onOperationEnd: (info, result) => {
      for (const h of activeHooks) {
        try {
          h.onOperationEnd?.(info, result);
        } catch (err) {
          console.error("Hook onOperationEnd error:", err);
        }
      }
    },

    onRequestStart: (info) => {
      for (const h of activeHooks) {
        try {
          h.onRequestStart?.(info);
        } catch (err) {
          console.error("Hook onRequestStart error:", err);
        }
      }
    },

    onRequestEnd: (info, result) => {
      for (const h of activeHooks) {
        try {
          h.onRequestEnd?.(info, result);
        } catch (err) {
          console.error("Hook onRequestEnd error:", err);
        }
      }
    },

    onRetry: (info, attempt, error, delayMs) => {
      for (const h of activeHooks) {
        try {
          h.onRetry?.(info, attempt, error, delayMs);
        } catch (err) {
          console.error("Hook onRetry error:", err);
        }
      }
    },
  };
}

/**
 * Options for console logging hooks.
 */
export interface ConsoleHooksOptions {
  /** Whether to log operation events (default: true) */
  logOperations?: boolean;
  /** Whether to log request events (default: false) */
  logRequests?: boolean;
  /** Whether to log retry events (default: true) */
  logRetries?: boolean;
  /** Minimum duration in ms to log (default: 0) */
  minDurationMs?: number;
  /** Custom logger (default: console) */
  logger?: Pick<Console, "log" | "warn" | "error">;
}

/**
 * Creates hooks that log to the console.
 * Useful for debugging and development.
 *
 * @example
 * ```ts
 * // Basic usage
 * const hooks = consoleHooks();
 *
 * // With custom options
 * const hooks = consoleHooks({
 *   logRequests: true,
 *   minDurationMs: 100,
 * });
 * ```
 */
export function consoleHooks(options: ConsoleHooksOptions = {}): BasecampHooks {
  const {
    logOperations = true,
    logRequests = false,
    logRetries = true,
    minDurationMs = 0,
    logger = console,
  } = options;

  return {
    onOperationStart: logOperations
      ? (info) => {
          const mutation = info.isMutation ? " [mutation]" : "";
          const resource = info.resourceId ? ` #${info.resourceId}` : "";
          const project = info.projectId ? ` (project: ${info.projectId})` : "";
          logger.log(`[Basecamp] ${info.service}.${info.operation}${resource}${project}${mutation}`);
        }
      : undefined,

    onOperationEnd: logOperations
      ? (info, result) => {
          if (result.durationMs < minDurationMs) return;

          const duration = `${result.durationMs}ms`;
          if (result.error) {
            logger.error(
              `[Basecamp] ${info.service}.${info.operation} failed (${duration}):`,
              result.error.message
            );
          } else {
            logger.log(`[Basecamp] ${info.service}.${info.operation} completed (${duration})`);
          }
        }
      : undefined,

    onRequestStart: logRequests
      ? (info) => {
          const retry = info.attempt > 1 ? ` (attempt ${info.attempt})` : "";
          logger.log(`[Basecamp] -> ${info.method} ${info.url}${retry}`);
        }
      : undefined,

    onRequestEnd: logRequests
      ? (info, result) => {
          if (result.durationMs < minDurationMs) return;

          const cache = result.fromCache ? " (cached)" : "";
          const status = result.error ? "error" : result.statusCode;
          logger.log(`[Basecamp] <- ${info.method} ${info.url} ${status} (${result.durationMs}ms)${cache}`);
        }
      : undefined,

    onRetry: logRetries
      ? (info, attempt, error, delayMs) => {
          logger.warn(
            `[Basecamp] Retrying ${info.method} ${info.url} (attempt ${attempt + 1}, waiting ${delayMs}ms): ${error.message}`
          );
        }
      : undefined,
  };
}

/**
 * Creates a no-op hooks object.
 * Useful as a default when hooks are optional.
 */
export function noopHooks(): BasecampHooks {
  return {};
}

/**
 * Internal helper to safely invoke a hook.
 * Returns silently if the hook throws.
 */
export function safeInvoke<K extends keyof BasecampHooks>(
  hooks: BasecampHooks | undefined,
  hookName: K,
  ...args: Parameters<NonNullable<BasecampHooks[K]>>
): void {
  if (!hooks) return;

  const hook = hooks[hookName];
  if (!hook) return;

  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (hook as (...a: unknown[]) => void)(...args);
  } catch (err) {
    console.error(`Hook ${hookName} error:`, err);
  }
}
