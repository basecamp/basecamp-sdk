/**
 * Webhooks service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Webhook entity from the Basecamp API. */
export type Webhook = components["schemas"]["Webhook"];

/**
 * Request parameters for create.
 */
export interface CreateWebhookRequest {
  /** payload url */
  payloadUrl: string;
  /** types */
  types: string[];
  /** active */
  active?: boolean;
}

/**
 * Request parameters for update.
 */
export interface UpdateWebhookRequest {
  /** payload url */
  payloadUrl?: string;
  /** types */
  types?: string[];
  /** active */
  active?: boolean;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Webhooks operations.
 */
export class WebhooksService extends BaseService {

  /**
   * List all webhooks for a project
   * @param projectId - The project ID
   * @returns Array of Webhook
   */
  async list(projectId: number): Promise<Webhook[]> {
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
   * @param projectId - The project ID
   * @param req - Request parameters
   * @returns The Webhook
   *
   * @example
   * ```ts
   * const result = await client.webhooks.create(123, { ... });
   * ```
   */
  async create(projectId: number, req: CreateWebhookRequest): Promise<Webhook> {
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
          body: {
            payload_url: req.payloadUrl,
            types: req.types,
            active: req.active,
          },
        })
    );
    return response;
  }

  /**
   * Get a single webhook by id
   * @param projectId - The project ID
   * @param webhookId - The webhook ID
   * @returns The Webhook
   */
  async get(projectId: number, webhookId: number): Promise<Webhook> {
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
   * @param projectId - The project ID
   * @param webhookId - The webhook ID
   * @param req - Request parameters
   * @returns The Webhook
   */
  async update(projectId: number, webhookId: number, req: UpdateWebhookRequest): Promise<Webhook> {
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
          body: {
            payload_url: req.payloadUrl,
            types: req.types,
            active: req.active,
          },
        })
    );
    return response;
  }

  /**
   * Delete a webhook
   * @param projectId - The project ID
   * @param webhookId - The webhook ID
   * @returns void
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