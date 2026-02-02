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
   * @param recordingId - The recording ID
   * @returns The Subscription
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.subscriptions.get(123);
   * ```
   */
  async get(recordingId: number): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "GetSubscription",
        resourceType: "subscription",
        isMutation: false,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/recordings/{recordingId}/subscription.json", {
          params: {
            path: { recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Subscribe the current user to a recording
   * @param recordingId - The recording ID
   * @returns The Subscription
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.subscriptions.subscribe(123);
   * ```
   */
  async subscribe(recordingId: number): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "Subscribe",
        resourceType: "resource",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/subscription.json", {
          params: {
            path: { recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Update subscriptions by adding or removing specific users
   * @param recordingId - The recording ID
   * @param req - Subscription update parameters
   * @returns The Subscription
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.subscriptions.update(123, { });
   * ```
   */
  async update(recordingId: number, req: UpdateSubscriptionRequest): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "UpdateSubscription",
        resourceType: "subscription",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/recordings/{recordingId}/subscription.json", {
          params: {
            path: { recordingId },
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
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.subscriptions.unsubscribe(123);
   * ```
   */
  async unsubscribe(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Subscriptions",
        operation: "Unsubscribe",
        resourceType: "resource",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.DELETE("/recordings/{recordingId}/subscription.json", {
          params: {
            path: { recordingId },
          },
        })
    );
  }
}