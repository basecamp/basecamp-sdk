/**
 * Service for Subscriptions operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Subscriptions operations
 */
export class SubscriptionsService extends BaseService {

  /**
   * Get subscription information for a recording
   */
  async get(projectId: number, recordingId: number): Promise<components["schemas"]["GetSubscriptionResponseContent"]> {
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
   */
  async subscribe(projectId: number, recordingId: number): Promise<components["schemas"]["SubscribeResponseContent"]> {
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
   */
  async update(projectId: number, recordingId: number, req: components["schemas"]["UpdateSubscriptionRequestContent"]): Promise<components["schemas"]["UpdateSubscriptionResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Unsubscribe the current user from a recording
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