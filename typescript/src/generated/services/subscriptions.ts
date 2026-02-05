/**
 * Subscriptions service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Subscription entity from the Basecamp API. */
export type Subscription = components["schemas"]["Subscription"];

/**
 * Request parameters for update.
 */
export interface UpdateSubscriptionRequest {
  /** Subscriptions */
  subscriptions?: number[];
  /** Unsubscriptions */
  unsubscriptions?: number[];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Subscriptions operations.
 */
export class SubscriptionsService extends BaseService {

  /**
   * Get subscription information for a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns The Subscription
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.subscriptions.get(123, 123);
   * ```
   */
  async get(projectId: number, recordingId: number): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "GetSubscription",
        resourceType: "subscription",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Subscribe the current user to a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns The Subscription
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.subscriptions.subscribe(123, 123);
   * ```
   */
  async subscribe(projectId: number, recordingId: number): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "Subscribe",
        resourceType: "resource",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Update subscriptions by adding or removing specific users
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @param req - Subscription update parameters
   * @returns The Subscription
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.subscriptions.update(123, 123, { });
   * ```
   */
  async update(projectId: number, recordingId: number, req: UpdateSubscriptionRequest): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "UpdateSubscription",
        resourceType: "subscription",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: {
            path: { projectId, recordingId },
          },
          body: {
            subscriptions: req.subscriptions,
            unsubscriptions: req.unsubscriptions,
          },
        })
    );
    return response;
  }

  /**
   * Unsubscribe the current user from a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.subscriptions.unsubscribe(123, 123);
   * ```
   */
  async unsubscribe(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Subscriptions",
        operation: "Unsubscribe",
        resourceType: "resource",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
  }
}