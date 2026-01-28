/**
 * Base service class for Basecamp API services.
 *
 * Provides shared functionality for all service classes including:
 * - Error handling with typed BasecampError
 * - Hooks integration for observability
 * - Request/response processing
 *
 * @example
 * ```ts
 * export class TodosService extends BaseService {
 *   async list(projectId: number, todolistId: number): Promise<Todo[]> {
 *     return this.request(
 *       { service: "Todos", operation: "List", resourceType: "todo", isMutation: false, projectId },
 *       () => this.client.GET("/buckets/{projectId}/todolists/{todolistId}/todos.json", {
 *         params: { path: { projectId, todolistId } },
 *       })
 *     );
 *   }
 * }
 * ```
 */

import type { BasecampHooks, OperationInfo, OperationResult } from "../hooks.js";
import { BasecampError, errorFromResponse } from "../errors.js";
import type { paths } from "../generated/schema.js";
import type createClient from "openapi-fetch";

/**
 * Raw client type from openapi-fetch.
 */
export type RawClient = ReturnType<typeof createClient<paths>>;

/**
 * Response type from openapi-fetch methods.
 */
export interface FetchResponse<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

/**
 * Abstract base class for all Basecamp API services.
 *
 * Services extend this class to inherit common functionality
 * for making API requests, handling errors, and integrating
 * with the hooks system.
 */
export abstract class BaseService {
  /** The underlying openapi-fetch client */
  protected readonly client: RawClient;

  /** Optional hooks for observability */
  protected readonly hooks?: BasecampHooks;

  constructor(client: RawClient, hooks?: BasecampHooks) {
    this.client = client;
    this.hooks = hooks;
  }

  /**
   * Executes an API request with error handling and hooks integration.
   *
   * @param info - Operation metadata for hooks
   * @param fn - The function that performs the actual API call
   * @returns The response data
   * @throws BasecampError on API errors
   */
  protected async request<T>(
    info: OperationInfo,
    fn: () => Promise<FetchResponse<T>>
  ): Promise<T> {
    const start = performance.now();
    let result: OperationResult = { durationMs: 0 };

    // Notify hooks of operation start
    this.hooks?.onOperationStart?.(info);

    try {
      const { data, error, response } = await fn();
      result.durationMs = Math.round(performance.now() - start);

      // Check for errors
      if (!response.ok || error) {
        const basecampError = await this.handleError(response, error);
        result.error = basecampError;
        throw basecampError;
      }

      // For void responses (204, etc.), return undefined as T
      if (response.status === 204 || data === undefined) {
        return undefined as T;
      }

      return data;
    } catch (err) {
      result.durationMs = Math.round(performance.now() - start);

      if (err instanceof BasecampError) {
        result.error = err;
      } else if (err instanceof Error) {
        result.error = err;
      }

      throw err;
    } finally {
      // Always notify hooks of operation end
      this.hooks?.onOperationEnd?.(info, result);
    }
  }

  /**
   * Converts an HTTP error response to a typed BasecampError.
   *
   * @param response - The HTTP response
   * @param error - Optional error object from openapi-fetch
   * @returns A BasecampError with appropriate code and metadata
   */
  protected async handleError(response: Response, error?: unknown): Promise<BasecampError> {
    // If already a BasecampError, just return it
    if (error instanceof BasecampError) {
      return error;
    }

    // Extract request ID from response headers if available
    const requestId = response.headers.get("X-Request-Id") ?? undefined;

    // Use the errorFromResponse helper to create the appropriate error
    return errorFromResponse(response, requestId);
  }
}
