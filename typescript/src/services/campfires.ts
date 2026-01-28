/**
 * Campfires service for the Basecamp API.
 *
 * Campfires are real-time chat rooms within Basecamp projects.
 * They contain lines (messages) and can have chatbot integrations.
 *
 * @example
 * ```ts
 * const campfires = await client.campfires.list();
 * const lines = await client.campfires.listLines(projectId, campfireId);
 * await client.campfires.createLine(projectId, campfireId, "Hello team!");
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp Campfire (real-time chat room).
 */
export type Campfire = components["schemas"]["Campfire"];

/**
 * A line (message) in a Campfire chat.
 */
export type CampfireLine = components["schemas"]["CampfireLine"];

/**
 * A Basecamp chatbot integration.
 */
export type Chatbot = components["schemas"]["Chatbot"];

/**
 * Request to create a new chatbot.
 */
export interface CreateChatbotRequest {
  /**
   * Chatbot name used to invoke queries and commands (required).
   * No spaces, emoji, or non-word characters allowed.
   */
  serviceName: string;
  /** HTTPS URL that Basecamp calls when the bot is addressed (optional) */
  commandUrl?: string;
}

/**
 * Request to update an existing chatbot.
 */
export interface UpdateChatbotRequest {
  /**
   * Chatbot name used to invoke queries and commands (required).
   * No spaces, emoji, or non-word characters allowed.
   */
  serviceName: string;
  /** HTTPS URL that Basecamp calls when the bot is addressed (optional) */
  commandUrl?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp Campfires.
 */
export class CampfiresService extends BaseService {
  /**
   * Lists all campfires across the account.
   *
   * @returns Array of campfires
   *
   * @example
   * ```ts
   * const campfires = await client.campfires.list();
   * ```
   */
  async list(): Promise<Campfire[]> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "List",
        resourceType: "campfire",
        isMutation: false,
      },
      () => this.client.GET("/chats.json", {})
    );

    return response?.campfires ?? [];
  }

  /**
   * Gets a campfire by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @returns The campfire
   * @throws BasecampError with code "not_found" if campfire doesn't exist
   *
   * @example
   * ```ts
   * const campfire = await client.campfires.get(projectId, campfireId);
   * console.log(campfire.title);
   * ```
   */
  async get(projectId: number, campfireId: number): Promise<Campfire> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "Get",
        resourceType: "campfire",
        isMutation: false,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}", {
          params: { path: { projectId, campfireId } },
        })
    );

    return response.campfire!;
  }

  /**
   * Lists all lines (messages) in a campfire.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @returns Array of campfire lines
   *
   * @example
   * ```ts
   * const lines = await client.campfires.listLines(projectId, campfireId);
   * ```
   */
  async listLines(projectId: number, campfireId: number): Promise<CampfireLine[]> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "ListLines",
        resourceType: "campfire_line",
        isMutation: false,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}/lines.json", {
          params: { path: { projectId, campfireId } },
        })
    );

    return response?.lines ?? [];
  }

  /**
   * Gets a single line (message) from a campfire.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @param lineId - The line ID
   * @returns The campfire line
   * @throws BasecampError with code "not_found" if line doesn't exist
   *
   * @example
   * ```ts
   * const line = await client.campfires.getLine(projectId, campfireId, lineId);
   * console.log(line.content);
   * ```
   */
  async getLine(projectId: number, campfireId: number, lineId: number): Promise<CampfireLine> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "GetLine",
        resourceType: "campfire_line",
        isMutation: false,
        projectId,
        resourceId: lineId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}/lines/{lineId}", {
          params: { path: { projectId, campfireId, lineId } },
        })
    );

    return response.line!;
  }

  /**
   * Creates a new line (message) in a campfire.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @param content - The plain text message content
   * @returns The created line
   * @throws BasecampError with code "validation" if content is missing
   *
   * @example
   * ```ts
   * const line = await client.campfires.createLine(projectId, campfireId, "Hello team!");
   * ```
   */
  async createLine(projectId: number, campfireId: number, content: string): Promise<CampfireLine> {
    if (!content) {
      throw Errors.validation("Campfire line content is required");
    }

    const response = await this.request(
      {
        service: "Campfires",
        operation: "CreateLine",
        resourceType: "campfire_line",
        isMutation: true,
        projectId,
        resourceId: campfireId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/chats/{campfireId}/lines.json", {
          params: { path: { projectId, campfireId } },
          body: { content },
        })
    );

    return response.line!;
  }

  /**
   * Deletes a line (message) from a campfire.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @param lineId - The line ID
   *
   * @example
   * ```ts
   * await client.campfires.deleteLine(projectId, campfireId, lineId);
   * ```
   */
  async deleteLine(projectId: number, campfireId: number, lineId: number): Promise<void> {
    await this.request(
      {
        service: "Campfires",
        operation: "DeleteLine",
        resourceType: "campfire_line",
        isMutation: true,
        projectId,
        resourceId: lineId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/chats/{campfireId}/lines/{lineId}", {
          params: { path: { projectId, campfireId, lineId } },
        })
    );
  }

  /**
   * Lists all chatbots for a campfire.
   *
   * Note: Chatbots are account-wide but with campfire-specific callback URLs.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @returns Array of chatbots
   *
   * @example
   * ```ts
   * const bots = await client.campfires.listChatbots(projectId, campfireId);
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
          params: { path: { projectId, campfireId } },
        })
    );

    return response?.chatbots ?? [];
  }

  /**
   * Gets a chatbot by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @param chatbotId - The chatbot ID
   * @returns The chatbot
   * @throws BasecampError with code "not_found" if chatbot doesn't exist
   *
   * @example
   * ```ts
   * const bot = await client.campfires.getChatbot(projectId, campfireId, chatbotId);
   * console.log(bot.service_name);
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
        resourceId: chatbotId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: { path: { projectId, campfireId, chatbotId } },
        })
    );

    return response.chatbot!;
  }

  /**
   * Creates a new chatbot for a campfire.
   *
   * Note: Chatbots are account-wide and can only be managed by administrators.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @param req - Chatbot creation parameters
   * @returns The created chatbot with its lines_url for posting
   * @throws BasecampError with code "validation" if service_name is missing
   *
   * @example
   * ```ts
   * const bot = await client.campfires.createChatbot(projectId, campfireId, {
   *   serviceName: "mybot",
   *   commandUrl: "https://example.com/bot/callback",
   * });
   * ```
   */
  async createChatbot(
    projectId: number,
    campfireId: number,
    req: CreateChatbotRequest
  ): Promise<Chatbot> {
    if (!req.serviceName) {
      throw Errors.validation("Chatbot service_name is required");
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
          params: { path: { projectId, campfireId } },
          body: {
            service_name: req.serviceName,
            command_url: req.commandUrl,
          },
        })
    );

    return response.chatbot!;
  }

  /**
   * Updates an existing chatbot.
   *
   * Note: Updates to chatbots are account-wide.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @param chatbotId - The chatbot ID
   * @param req - Chatbot update parameters
   * @returns The updated chatbot
   * @throws BasecampError with code "validation" if service_name is missing
   *
   * @example
   * ```ts
   * const bot = await client.campfires.updateChatbot(projectId, campfireId, chatbotId, {
   *   serviceName: "updatedbot",
   * });
   * ```
   */
  async updateChatbot(
    projectId: number,
    campfireId: number,
    chatbotId: number,
    req: UpdateChatbotRequest
  ): Promise<Chatbot> {
    if (!req.serviceName) {
      throw Errors.validation("Chatbot service_name is required");
    }

    const response = await this.request(
      {
        service: "Campfires",
        operation: "UpdateChatbot",
        resourceType: "chatbot",
        isMutation: true,
        projectId,
        resourceId: chatbotId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: { path: { projectId, campfireId, chatbotId } },
          body: {
            service_name: req.serviceName,
            command_url: req.commandUrl,
          },
        })
    );

    return response.chatbot!;
  }

  /**
   * Deletes a chatbot.
   *
   * Note: Deleting a chatbot removes it from the entire account.
   *
   * @param projectId - The project (bucket) ID
   * @param campfireId - The campfire ID
   * @param chatbotId - The chatbot ID
   *
   * @example
   * ```ts
   * await client.campfires.deleteChatbot(projectId, campfireId, chatbotId);
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
        resourceId: chatbotId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: { path: { projectId, campfireId, chatbotId } },
        })
    );
  }
}
