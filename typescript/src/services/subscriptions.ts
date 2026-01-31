/**
 * Subscriptions service for the Basecamp API.
 *
 * Subscriptions control who receives notifications for a specific recording
 * (like a todo, message, or comment). Users can subscribe or unsubscribe
 * themselves, and you can batch update subscriptions for multiple users.
 *
 * @example
 * ```ts
 * const subscription = await client.subscriptions.get(projectId, recordingId);
 * await client.subscriptions.subscribe(projectId, recordingId);
 * await client.subscriptions.unsubscribe(projectId, recordingId);
 * await client.subscriptions.update(projectId, recordingId, {
 *   subscriptions: [userId1, userId2],
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A person reference (simplified).
 */
export interface PersonRef {
  id: number;
  name: string;
  email_address?: string;
  avatar_url?: string;
  admin?: boolean;
  owner?: boolean;
}

/**
 * Subscription state for a recording.
 */
export interface Subscription {
  /** Whether the current user is subscribed */
  subscribed: boolean;
  /** Number of subscribers */
  count: number;
  /** URL to manage subscriptions */
  url: string;
  /** List of subscribers */
  subscribers: PersonRef[];
}

/**
 * Request to update subscriptions.
 */
export interface UpdateSubscriptionRequest {
  /** Person IDs to subscribe to the recording (optional) */
  subscriptions?: number[];
  /** Person IDs to unsubscribe from the recording (optional) */
  unsubscriptions?: number[];
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing subscriptions in Basecamp.
 */
export class SubscriptionsService extends BaseService {
  /**
   * Gets the subscription information for a recording.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   * @returns The subscription information
   * @throws BasecampError with code "not_found" if recording doesn't exist
   *
   * @example
   * ```ts
   * const subscription = await client.subscriptions.get(projectId, todoId);
   * console.log(subscription.subscribed, subscription.count);
   * subscription.subscribers.forEach(p => console.log(p.name));
   * ```
   */
  async get(projectId: number, recordingId: number): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "Get",
        resourceType: "subscription",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: { path: { projectId, recordingId } },
        })
    );

    return response as unknown as Subscription;
  }

  /**
   * Subscribes the current user to the recording.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   * @returns The updated subscription information
   *
   * @example
   * ```ts
   * const subscription = await client.subscriptions.subscribe(projectId, todoId);
   * console.log("Now subscribed:", subscription.subscribed);
   * ```
   */
  async subscribe(projectId: number, recordingId: number): Promise<Subscription> {
    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "Subscribe",
        resourceType: "subscription",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: { path: { projectId, recordingId } },
        })
    );

    return response as unknown as Subscription;
  }

  /**
   * Unsubscribes the current user from the recording.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   *
   * @example
   * ```ts
   * await client.subscriptions.unsubscribe(projectId, todoId);
   * ```
   */
  async unsubscribe(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Subscriptions",
        operation: "Unsubscribe",
        resourceType: "subscription",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: { path: { projectId, recordingId } },
        })
    );
  }

  /**
   * Batch modifies subscriptions by adding or removing specific users.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   * @param req - Subscription update parameters
   * @returns The updated subscription information
   * @throws BasecampError with code "validation" if neither subscriptions nor unsubscriptions is provided
   *
   * @example
   * ```ts
   * const subscription = await client.subscriptions.update(projectId, todoId, {
   *   subscriptions: [userId1, userId2],
   *   unsubscriptions: [userId3],
   * });
   * ```
   */
  async update(
    projectId: number,
    recordingId: number,
    req: UpdateSubscriptionRequest
  ): Promise<Subscription> {
    if (
      (!req.subscriptions || req.subscriptions.length === 0) &&
      (!req.unsubscriptions || req.unsubscriptions.length === 0)
    ) {
      throw Errors.validation(
        "At least one of subscriptions or unsubscriptions must be specified"
      );
    }

    const response = await this.request(
      {
        service: "Subscriptions",
        operation: "Update",
        resourceType: "subscription",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/subscription.json", {
          params: { path: { projectId, recordingId } },
          body: {
            subscriptions: req.subscriptions,
            unsubscriptions: req.unsubscriptions,
          },
        })
    );

    return response as unknown as Subscription;
  }
}
