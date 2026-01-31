/**
 * Webhooks service for the Basecamp API.
 *
 * Webhooks allow you to receive real-time notifications when events
 * occur in a Basecamp project. You can subscribe to specific event types
 * and receive HTTP POST requests to your specified URL.
 *
 * @example
 * ```ts
 * const webhooks = await client.webhooks.list(projectId);
 * const webhook = await client.webhooks.create(projectId, {
 *   payloadUrl: "https://example.com/webhook",
 *   types: ["Todo", "Comment"],
 * });
 * await client.webhooks.delete(projectId, webhookId);
 * ```
 */

import { BaseService } from "./base.js";
import { BasecampError, Errors } from "../errors.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp webhook subscription.
 */
export interface Webhook {
  id: number;
  active: boolean;
  created_at: string;
  updated_at: string;
  payload_url: string;
  types: string[];
  app_url?: string;
  url?: string;
}

/**
 * Request to create a new webhook.
 */
export interface CreateWebhookRequest {
  /** URL to receive webhook payloads (required) */
  payloadUrl: string;
  /** Event types to subscribe to (required), e.g. ["Todo", "Comment"] */
  types: string[];
  /** Whether the webhook is active (optional, defaults to true) */
  active?: boolean;
}

/**
 * Request to update an existing webhook.
 */
export interface UpdateWebhookRequest {
  /** URL to receive webhook payloads (optional) */
  payloadUrl?: string;
  /** Event types to subscribe to (optional) */
  types?: string[];
  /** Whether the webhook is active (optional) */
  active?: boolean;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing webhooks in Basecamp.
 */
export class WebhooksService extends BaseService {
  /**
   * Lists all webhooks for a project.
   *
   * @param projectId - The project (bucket) ID
   * @returns Array of webhooks
   *
   * @example
   * ```ts
   * const webhooks = await client.webhooks.list(projectId);
   * webhooks.forEach(w => console.log(w.payload_url, w.types, w.active));
   * ```
   */
  async list(projectId: number): Promise<Webhook[]> {
    const response = await this.request(
      {
        service: "Webhooks",
        operation: "List",
        resourceType: "webhook",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/webhooks.json", {
          params: { path: { projectId } },
        })
    );

    return (response ?? []) as Webhook[];
  }

  /**
   * Gets a webhook by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param webhookId - The webhook ID
   * @returns The webhook
   * @throws BasecampError with code "not_found" if webhook doesn't exist
   *
   * @example
   * ```ts
   * const webhook = await client.webhooks.get(projectId, webhookId);
   * console.log(webhook.payload_url, webhook.types);
   * ```
   */
  async get(projectId: number, webhookId: number): Promise<Webhook> {
    const response = await this.request(
      {
        service: "Webhooks",
        operation: "Get",
        resourceType: "webhook",
        isMutation: false,
        projectId,
        resourceId: webhookId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/webhooks/{webhookId}", {
          params: { path: { projectId, webhookId } },
        })
    );

    return response as unknown as Webhook;
  }

  /**
   * Creates a new webhook for a project.
   *
   * @param projectId - The project (bucket) ID
   * @param req - Webhook creation parameters
   * @returns The created webhook
   * @throws BasecampError with code "validation" if payloadUrl or types is missing
   *
   * @example
   * ```ts
   * const webhook = await client.webhooks.create(projectId, {
   *   payloadUrl: "https://example.com/webhook",
   *   types: ["Todo", "Todolist", "Comment"],
   *   active: true,
   * });
   * ```
   */
  async create(projectId: number, req: CreateWebhookRequest): Promise<Webhook> {
    if (!req.payloadUrl) {
      throw Errors.validation("Webhook payload_url is required");
    }
    try {
      const parsed = new URL(req.payloadUrl);
      if (parsed.protocol !== "https:") {
        throw Errors.validation("Webhook payload_url must use HTTPS");
      }
    } catch (err) {
      if (err instanceof BasecampError) throw err;
      throw Errors.validation(`Invalid webhook payload_url: ${req.payloadUrl}`);
    }
    if (!req.types || req.types.length === 0) {
      throw Errors.validation("Webhook types are required");
    }

    const response = await this.request(
      {
        service: "Webhooks",
        operation: "Create",
        resourceType: "webhook",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/webhooks.json", {
          params: { path: { projectId } },
          body: {
            payload_url: req.payloadUrl,
            types: req.types,
            active: req.active ?? true,
          },
        })
    );

    return response as unknown as Webhook;
  }

  /**
   * Updates an existing webhook.
   *
   * @param projectId - The project (bucket) ID
   * @param webhookId - The webhook ID
   * @param req - Webhook update parameters
   * @returns The updated webhook
   *
   * @example
   * ```ts
   * const webhook = await client.webhooks.update(projectId, webhookId, {
   *   active: false,
   * });
   * ```
   */
  async update(
    projectId: number,
    webhookId: number,
    req: UpdateWebhookRequest
  ): Promise<Webhook> {
    if (req.payloadUrl !== undefined) {
      try {
        const parsed = new URL(req.payloadUrl);
        if (parsed.protocol !== "https:") {
          throw Errors.validation("Webhook payload_url must use HTTPS");
        }
      } catch (err) {
        if (err instanceof BasecampError) throw err;
        throw Errors.validation(`Invalid webhook payload_url: ${req.payloadUrl}`);
      }
    }

    const response = await this.request(
      {
        service: "Webhooks",
        operation: "Update",
        resourceType: "webhook",
        isMutation: true,
        projectId,
        resourceId: webhookId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/webhooks/{webhookId}", {
          params: { path: { projectId, webhookId } },
          body: {
            payload_url: req.payloadUrl,
            types: req.types,
            active: req.active,
          },
        })
    );

    return response as unknown as Webhook;
  }

  /**
   * Deletes a webhook.
   *
   * @param projectId - The project (bucket) ID
   * @param webhookId - The webhook ID
   *
   * @example
   * ```ts
   * await client.webhooks.delete(projectId, webhookId);
   * ```
   */
  async delete(projectId: number, webhookId: number): Promise<void> {
    await this.request(
      {
        service: "Webhooks",
        operation: "Delete",
        resourceType: "webhook",
        isMutation: true,
        projectId,
        resourceId: webhookId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/webhooks/{webhookId}", {
          params: { path: { projectId, webhookId } },
        })
    );
  }
}
