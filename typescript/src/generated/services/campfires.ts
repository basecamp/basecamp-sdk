/**
 * Campfires service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Campfire entity from the Basecamp API. */
export type Campfire = components["schemas"]["Campfire"];
/** Chatbot entity from the Basecamp API. */
export type Chatbot = components["schemas"]["Chatbot"];
/** CampfireLine entity from the Basecamp API. */
export type CampfireLine = components["schemas"]["CampfireLine"];

/**
 * Request parameters for createChatbot.
 */
export interface CreateChatbotCampfireRequest {
  /** Service name */
  serviceName: string;
  /** Command url */
  commandUrl?: string;
}

/**
 * Request parameters for updateChatbot.
 */
export interface UpdateChatbotCampfireRequest {
  /** Service name */
  serviceName: string;
  /** Command url */
  commandUrl?: string;
}

/**
 * Request parameters for createLine.
 */
export interface CreateLineCampfireRequest {
  /** Text content */
  content: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Campfires operations.
 */
export class CampfiresService extends BaseService {

  /**
   * Get a campfire by ID
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @returns The Campfire
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.campfires.get(123, 123);
   * ```
   */
  async get(projectId: number, campfireId: number): Promise<Campfire> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "GetCampfire",
        resourceType: "campfire",
        isMutation: false,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}", {
          params: {
            path: { projectId, campfireId },
          },
        })
    );
    return response;
  }

  /**
   * List all chatbots for a campfire
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @returns Array of Chatbot
   *
   * @example
   * ```ts
   * const result = await client.campfires.listChatbots(123, 123);
   * ```
   */
  async listChatbots(projectId: number, campfireId: number): Promise<Chatbot[]> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "ListChatbots",
        resourceType: "chatbot",
        isMutation: false,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}/integrations.json", {
          params: {
            path: { projectId, campfireId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new chatbot for a campfire
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @param req - Chatbot creation parameters
   * @returns The Chatbot
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.campfires.createChatbot(123, 123, { serviceName: "example" });
   * ```
   */
  async createChatbot(projectId: number, campfireId: number, req: CreateChatbotCampfireRequest): Promise<Chatbot> {
    if (!req.serviceName) {
      throw Errors.validation("Service name is required");
    }
    const response = await this.request(
      {
        service: "Campfires",
        operation: "CreateChatbot",
        resourceType: "chatbot",
        isMutation: true,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/chats/{campfireId}/integrations.json", {
          params: {
            path: { projectId, campfireId },
          },
          body: {
            service_name: req.serviceName,
            command_url: req.commandUrl,
          },
        })
    );
    return response;
  }

  /**
   * Get a chatbot by ID
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @param chatbotId - The chatbot ID
   * @returns The Chatbot
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.campfires.getChatbot(123, 123, 123);
   * ```
   */
  async getChatbot(projectId: number, campfireId: number, chatbotId: number): Promise<Chatbot> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "GetChatbot",
        resourceType: "chatbot",
        isMutation: false,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: {
            path: { projectId, campfireId, chatbotId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing chatbot
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @param chatbotId - The chatbot ID
   * @param req - Chatbot update parameters
   * @returns The Chatbot
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.campfires.updateChatbot(123, 123, 123, { serviceName: "example" });
   * ```
   */
  async updateChatbot(projectId: number, campfireId: number, chatbotId: number, req: UpdateChatbotCampfireRequest): Promise<Chatbot> {
    if (!req.serviceName) {
      throw Errors.validation("Service name is required");
    }
    const response = await this.request(
      {
        service: "Campfires",
        operation: "UpdateChatbot",
        resourceType: "chatbot",
        isMutation: true,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: {
            path: { projectId, campfireId, chatbotId },
          },
          body: {
            service_name: req.serviceName,
            command_url: req.commandUrl,
          },
        })
    );
    return response;
  }

  /**
   * Delete a chatbot
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @param chatbotId - The chatbot ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.campfires.deleteChatbot(123, 123, 123);
   * ```
   */
  async deleteChatbot(projectId: number, campfireId: number, chatbotId: number): Promise<void> {
    await this.request(
      {
        service: "Campfires",
        operation: "DeleteChatbot",
        resourceType: "chatbot",
        isMutation: true,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: {
            path: { projectId, campfireId, chatbotId },
          },
        })
    );
  }

  /**
   * List all lines (messages) in a campfire
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @returns Array of CampfireLine
   *
   * @example
   * ```ts
   * const result = await client.campfires.listLines(123, 123);
   * ```
   */
  async listLines(projectId: number, campfireId: number): Promise<CampfireLine[]> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "ListCampfireLines",
        resourceType: "campfire_line",
        isMutation: false,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}/lines.json", {
          params: {
            path: { projectId, campfireId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new line (message) in a campfire
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @param req - Campfire_line creation parameters
   * @returns The CampfireLine
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.campfires.createLine(123, 123, { content: "Hello world" });
   * ```
   */
  async createLine(projectId: number, campfireId: number, req: CreateLineCampfireRequest): Promise<CampfireLine> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Campfires",
        operation: "CreateCampfireLine",
        resourceType: "campfire_line",
        isMutation: true,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/chats/{campfireId}/lines.json", {
          params: {
            path: { projectId, campfireId },
          },
          body: {
            content: req.content,
          },
        })
    );
    return response;
  }

  /**
   * Get a campfire line by ID
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @param lineId - The line ID
   * @returns The CampfireLine
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.campfires.getLine(123, 123, 123);
   * ```
   */
  async getLine(projectId: number, campfireId: number, lineId: number): Promise<CampfireLine> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "GetCampfireLine",
        resourceType: "campfire_line",
        isMutation: false,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}/lines/{lineId}", {
          params: {
            path: { projectId, campfireId, lineId },
          },
        })
    );
    return response;
  }

  /**
   * Delete a campfire line
   * @param projectId - The project ID
   * @param campfireId - The campfire ID
   * @param lineId - The line ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.campfires.deleteLine(123, 123, 123);
   * ```
   */
  async deleteLine(projectId: number, campfireId: number, lineId: number): Promise<void> {
    await this.request(
      {
        service: "Campfires",
        operation: "DeleteCampfireLine",
        resourceType: "campfire_line",
        isMutation: true,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/chats/{campfireId}/lines/{lineId}", {
          params: {
            path: { projectId, campfireId, lineId },
          },
        })
    );
  }

  /**
   * List all campfires across the account
   * @returns Array of Campfire
   *
   * @example
   * ```ts
   * const result = await client.campfires.list();
   * ```
   */
  async list(): Promise<Campfire[]> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "ListCampfires",
        resourceType: "campfire",
        isMutation: false,
      },
      () =>
        this.client.GET("/chats.json", {
        })
    );
    return response ?? [];
  }
}