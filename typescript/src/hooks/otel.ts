/**
 * OpenTelemetry hooks for the Basecamp SDK.
 *
 * Provides distributed tracing and metrics for API operations.
 * Requires @opentelemetry/api as an optional peer dependency.
 *
 * @example
 * ```ts
 * import { createBasecampClient, otelHooks } from "@37signals/basecamp";
 * import { trace, metrics } from "@opentelemetry/api";
 *
 * const tracer = trace.getTracer("my-app");
 * const meter = metrics.getMeter("my-app");
 *
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: "token",
 *   hooks: otelHooks({ tracer, meter }),
 * });
 *
 * // All SDK operations will now create spans and record metrics
 * const projects = await client.projects.list();
 * ```
 */

import type { BasecampHooks, OperationInfo, OperationResult, RequestInfo, RequestResult } from "../hooks.js";

// OpenTelemetry API types (optional peer dependency)
// We use the actual types from @opentelemetry/api when available
interface OtelSpan {
  setAttribute(key: string, value: string | number | boolean): this;
  setStatus(status: { code: number; message?: string }): this;
  recordException(exception: Error): this;
  end(): void;
}

interface OtelTracer {
  startSpan(name: string, options?: { attributes?: Record<string, string | number | boolean> }): OtelSpan;
}

interface OtelMeter {
  createHistogram(name: string, options?: { description?: string; unit?: string }): OtelHistogram;
  createCounter(name: string, options?: { description?: string }): OtelCounter;
}

interface OtelHistogram {
  record(value: number, attributes?: Record<string, string | number | boolean>): void;
}

interface OtelCounter {
  add(value: number, attributes?: Record<string, string | number | boolean>): void;
}

// Span status codes from OpenTelemetry
const SpanStatusCode = {
  UNSET: 0,
  OK: 1,
  ERROR: 2,
} as const;

/**
 * Options for OpenTelemetry hooks.
 */
export interface OtelHooksOptions {
  /**
   * OpenTelemetry tracer instance.
   * Create with: `trace.getTracer("your-app-name")`
   */
  tracer?: OtelTracer;

  /**
   * OpenTelemetry meter instance for metrics.
   * Create with: `metrics.getMeter("your-app-name")`
   */
  meter?: OtelMeter;

  /**
   * Whether to record request-level spans (in addition to operation spans).
   * Defaults to false for reduced noise.
   */
  recordRequestSpans?: boolean;

  /**
   * Prefix for span names.
   * Defaults to "basecamp"
   */
  spanPrefix?: string;

  /**
   * Prefix for metric names.
   * Defaults to "basecamp"
   */
  metricPrefix?: string;
}

/**
 * Internal state for tracking spans and request timing.
 */
interface OtelState {
  /** Active operation spans by key */
  operationSpans: Map<string, OtelSpan>;
  /** Active request spans by key */
  requestSpans: Map<string, OtelSpan>;
  /** Histogram for operation duration */
  operationDuration?: OtelHistogram;
  /** Histogram for request duration */
  requestDuration?: OtelHistogram;
  /** Counter for operations */
  operationCounter?: OtelCounter;
  /** Counter for errors */
  errorCounter?: OtelCounter;
  /** Counter for retries */
  retryCounter?: OtelCounter;
}

/** Counter for generating unique keys */
let keyCounter = 0;

/**
 * Creates a unique key for an operation.
 */
function operationKey(info: OperationInfo): string {
  return `${info.service}.${info.operation}:${info.resourceId ?? ""}:${++keyCounter}`;
}

/**
 * Creates a unique key for a request.
 */
function requestKey(info: RequestInfo): string {
  return `${info.method}:${info.url}:${info.attempt}:${++keyCounter}`;
}

/**
 * Creates OpenTelemetry hooks for distributed tracing and metrics.
 *
 * This function creates hooks that integrate with OpenTelemetry for:
 * - **Tracing**: Creates spans for SDK operations with relevant attributes
 * - **Metrics**: Records histograms for latency and counters for operations/errors
 *
 * Span attributes include:
 * - `basecamp.service`: Service name (e.g., "Todos", "Projects")
 * - `basecamp.operation`: Operation name (e.g., "List", "Create")
 * - `basecamp.resource_type`: Type of resource being accessed
 * - `basecamp.is_mutation`: Whether the operation modifies data
 * - `basecamp.project_id`: Project ID if applicable
 * - `basecamp.resource_id`: Resource ID if applicable
 *
 * Metrics recorded:
 * - `basecamp.operation.duration`: Histogram of operation durations (ms)
 * - `basecamp.request.duration`: Histogram of HTTP request durations (ms)
 * - `basecamp.operations.total`: Counter of total operations
 * - `basecamp.errors.total`: Counter of errors
 * - `basecamp.retries.total`: Counter of retry attempts
 *
 * @param options - Configuration options
 * @returns Hooks object for use with createBasecampClient
 *
 * @example
 * ```ts
 * import { createBasecampClient, otelHooks } from "@37signals/basecamp";
 * import { trace, metrics } from "@opentelemetry/api";
 *
 * // Set up OpenTelemetry first (SDK configuration not shown)
 * const tracer = trace.getTracer("my-app", "1.0.0");
 * const meter = metrics.getMeter("my-app", "1.0.0");
 *
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: "token",
 *   hooks: otelHooks({
 *     tracer,
 *     meter,
 *     spanPrefix: "basecamp",
 *     recordRequestSpans: true,
 *   }),
 * });
 *
 * // Operations will create spans and record metrics
 * const todos = await client.todos.list(projectId, todolistId);
 * ```
 */
export function otelHooks(options: OtelHooksOptions = {}): BasecampHooks {
  const {
    tracer,
    meter,
    recordRequestSpans = false,
    spanPrefix = "basecamp",
    metricPrefix = "basecamp",
  } = options;

  // Initialize state
  const state: OtelState = {
    operationSpans: new Map(),
    requestSpans: new Map(),
  };

  // Initialize metrics if meter is provided
  if (meter) {
    state.operationDuration = meter.createHistogram(`${metricPrefix}.operation.duration`, {
      description: "Duration of Basecamp SDK operations",
      unit: "ms",
    });

    state.requestDuration = meter.createHistogram(`${metricPrefix}.request.duration`, {
      description: "Duration of HTTP requests",
      unit: "ms",
    });

    state.operationCounter = meter.createCounter(`${metricPrefix}.operations.total`, {
      description: "Total number of Basecamp SDK operations",
    });

    state.errorCounter = meter.createCounter(`${metricPrefix}.errors.total`, {
      description: "Total number of errors",
    });

    state.retryCounter = meter.createCounter(`${metricPrefix}.retries.total`, {
      description: "Total number of retry attempts",
    });
  }

  return {
    onOperationStart(info: OperationInfo): void {
      // Record operation count (even without tracer)
      state.operationCounter?.add(1, {
        service: info.service,
        operation: info.operation,
        is_mutation: info.isMutation,
      });

      if (!tracer) return;

      const spanName = `${spanPrefix}.${info.service}.${info.operation}`;
      const attributes: Record<string, string | number | boolean> = {
        [`${spanPrefix}.service`]: info.service,
        [`${spanPrefix}.operation`]: info.operation,
        [`${spanPrefix}.resource_type`]: info.resourceType,
        [`${spanPrefix}.is_mutation`]: info.isMutation,
      };

      if (info.projectId) {
        attributes[`${spanPrefix}.project_id`] = info.projectId;
      }
      if (info.resourceId) {
        attributes[`${spanPrefix}.resource_id`] = info.resourceId;
      }

      const span = tracer.startSpan(spanName, { attributes });
      const key = operationKey(info);
      state.operationSpans.set(key, span);
    },

    onOperationEnd(info: OperationInfo, result: OperationResult): void {
      // Find and end the span
      if (tracer) {
        // Find the most recent span for this operation
        for (const [key, span] of state.operationSpans) {
          if (key.startsWith(`${info.service}.${info.operation}:`)) {
            if (result.error) {
              span.setStatus({ code: SpanStatusCode.ERROR, message: result.error.message });
              span.recordException(result.error);
              span.setAttribute("error", true);
              span.setAttribute("error.message", result.error.message);
            } else {
              span.setStatus({ code: SpanStatusCode.OK });
            }
            span.setAttribute("duration_ms", result.durationMs);
            span.end();
            state.operationSpans.delete(key);
            break;
          }
        }
      }

      // Record metrics
      const labels = {
        service: info.service,
        operation: info.operation,
        is_mutation: info.isMutation,
        success: !result.error,
      };

      state.operationDuration?.record(result.durationMs, labels);

      if (result.error) {
        state.errorCounter?.add(1, {
          service: info.service,
          operation: info.operation,
          error_type: result.error.name,
        });
      }
    },

    onRequestStart(info: RequestInfo): void {
      if (!tracer || !recordRequestSpans) return;

      const spanName = `${spanPrefix}.http.${info.method}`;
      const span = tracer.startSpan(spanName, {
        attributes: {
          "http.method": info.method,
          "http.url": info.url,
          "http.attempt": info.attempt,
        },
      });

      const key = requestKey(info);
      state.requestSpans.set(key, span);
    },

    onRequestEnd(info: RequestInfo, result: RequestResult): void {
      // Find and end the span
      if (tracer && recordRequestSpans) {
        for (const [key, span] of state.requestSpans) {
          if (key.startsWith(`${info.method}:${info.url}:${info.attempt}:`)) {
            span.setAttribute("http.status_code", result.statusCode);
            span.setAttribute("http.from_cache", result.fromCache);
            span.setAttribute("duration_ms", result.durationMs);

            if (result.error) {
              span.setStatus({ code: SpanStatusCode.ERROR, message: result.error.message });
              span.recordException(result.error);
            } else if (result.statusCode >= 400) {
              span.setStatus({ code: SpanStatusCode.ERROR, message: `HTTP ${result.statusCode}` });
            } else {
              span.setStatus({ code: SpanStatusCode.OK });
            }

            span.end();
            state.requestSpans.delete(key);
            break;
          }
        }
      }

      // Record request metrics
      state.requestDuration?.record(result.durationMs, {
        method: info.method,
        status_code: result.statusCode,
        from_cache: result.fromCache,
      });
    },

    onRetry(info: RequestInfo, attempt: number, error: Error, _delayMs: number): void {
      state.retryCounter?.add(1, {
        method: info.method,
        attempt,
        error_type: error.name,
      });
    },
  };
}
