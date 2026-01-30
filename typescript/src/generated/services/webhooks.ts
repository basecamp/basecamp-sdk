/**
 * Service for Webhooks operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Webhooks operations
 */
export class WebhooksService extends BaseService {

  /**
   * List all webhooks for a project
   */
  async list(projectId: number): Promise<components["schemas"]["ListWebhooksResponseContent"]> {
    const response = await this.request(
      {
        service: "Webhooks",
        operation: "ListWebhooks",
        resourceType: "webhook",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/webhooks.json", {
          params: {
            path: { projectId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new webhook for a project
   */
  async create(projectId: number, req: components["schemas"]["CreateWebhookRequestContent"]): Promise<components["schemas"]["CreateWebhookResponseContent"]> {
    const response = await this.request(
      {
        service: "Webhooks",
        operation: "CreateWebhook",
        resourceType: "webhook",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/webhooks.json", {
          params: {
            path: { projectId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a single webhook by id
   */
  async get(projectId: number, webhookId: number): Promise<components["schemas"]["GetWebhookResponseContent"]> {
    const response = await this.request(
      {
        service: "Webhooks",
        operation: "GetWebhook",
        resourceType: "webhook",
        isMutation: false,
        projectId,
        resourceId: webhookId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/webhooks/{webhookId}", {
          params: {
            path: { projectId, webhookId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing webhook
   */
  async update(projectId: number, webhookId: number, req: components["schemas"]["UpdateWebhookRequestContent"]): Promise<components["schemas"]["UpdateWebhookResponseContent"]> {
    const response = await this.request(
      {
        service: "Webhooks",
        operation: "UpdateWebhook",
        resourceType: "webhook",
        isMutation: true,
        projectId,
        resourceId: webhookId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/webhooks/{webhookId}", {
          params: {
            path: { projectId, webhookId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Delete a webhook
   */
  async delete(projectId: number, webhookId: number): Promise<void> {
    await this.request(
      {
        service: "Webhooks",
        operation: "DeleteWebhook",
        resourceType: "webhook",
        isMutation: true,
        projectId,
        resourceId: webhookId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/webhooks/{webhookId}", {
          params: {
            path: { projectId, webhookId },
          },
        })
    );
  }
}