/**
 * Webhooks service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Webhook entity from the Basecamp API. */
export type Webhook = components["schemas"]["Webhook"];

/**
 * Request parameters for create.
 */
export interface CreateWebhookRequest {
  /** Payload url */
  payloadUrl: string;
  /** Types */
  types: string[];
  /** Active */
  active?: boolean;
}

/**
 * Request parameters for update.
 */
export interface UpdateWebhookRequest {
  /** Payload url */
  payloadUrl?: string;
  /** Types */
  types?: string[];
  /** Active */
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
   *
   * @example
   * ```ts
   * const result = await client.webhooks.list(123);
   * ```
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
   * @param req - Webhook creation parameters
   * @returns The Webhook
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.webhooks.create(123, { payloadUrl: "example", types: [1234] });
   * ```
   */
  async create(projectId: number, req: CreateWebhookRequest): Promise<Webhook> {
    if (!req.payloadUrl) {
      throw Errors.validation("Payload url is required");
    }
    if (!req.types) {
      throw Errors.validation("Types is required");
    }
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
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.webhooks.get(123, 123);
   * ```
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
   * @param req - Webhook update parameters
   * @returns The Webhook
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.webhooks.update(123, 123, { });
   * ```
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
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.webhooks.delete(123, 123);
   * ```
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