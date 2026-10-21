/**
 * Embeds service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for create.
 */
export interface CreateEmbedRequest {
  /** The URL of the content to embed. */
  url: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Embeds operations.
 */
export class EmbedsService extends BaseService {

  /**
   * Create an embed for a URL, or return the one Basecamp already has for it.
   * @param req - Embed creation parameters
   * @returns The embed
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.embeds.create({ url: "example" });
   * ```
   */
  async create(req: CreateEmbedRequest): Promise<components["schemas"]["CreateEmbedResponseContent"]> {
    if (!req.url) {
      throw Errors.validation("Url is required");
    }
    const response = await this.request(
      {
        service: "Embeds",
        operation: "CreateEmbed",
        resourceType: "embed",
        isMutation: true,
      },
      () =>
        this.client.POST("/embeds.json", {
          body: {
            url: req.url,
          },
        })
    );
    return response;
  }

  /**
   * Get an embed by its sgid.
   * @param sgid - An embed's signed global id, an opaque string.
   * @returns The embed
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.embeds.get("example");
   * ```
   */
  async get(sgid: string): Promise<components["schemas"]["GetEmbedResponseContent"]> {
    const response = await this.request(
      {
        service: "Embeds",
        operation: "GetEmbed",
        resourceType: "embed",
        isMutation: false,
      },
      () =>
        this.client.GET("/embeds/{sgid}", {
          params: {
            path: { sgid },
          },
        })
    );
    return response;
  }
}