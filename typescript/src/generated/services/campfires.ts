/**
 * Service for Campfires operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Campfires operations
 */
export class CampfiresService extends BaseService {

  /**
   * Get a campfire by ID
   */
  async get(projectId: number, campfireId: number): Promise<components["schemas"]["GetCampfireResponseContent"]> {
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
   */
  async listChatbots(projectId: number, campfireId: number): Promise<components["schemas"]["ListChatbotsResponseContent"]> {
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
   */
  async createChatbot(projectId: number, campfireId: number, req: components["schemas"]["CreateChatbotRequestContent"]): Promise<components["schemas"]["CreateChatbotResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a chatbot by ID
   */
  async getChatbot(projectId: number, campfireId: number, chatbotId: number): Promise<components["schemas"]["GetChatbotResponseContent"]> {
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
   */
  async updateChatbot(projectId: number, campfireId: number, chatbotId: number, req: components["schemas"]["UpdateChatbotRequestContent"]): Promise<components["schemas"]["UpdateChatbotResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Delete a chatbot
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
   */
  async listLines(projectId: number, campfireId: number): Promise<components["schemas"]["ListCampfireLinesResponseContent"]> {
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
   */
  async createLine(projectId: number, campfireId: number, req: components["schemas"]["CreateCampfireLineRequestContent"]): Promise<components["schemas"]["CreateCampfireLineResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a campfire line by ID
   */
  async getLine(projectId: number, campfireId: number, lineId: number): Promise<components["schemas"]["GetCampfireLineResponseContent"]> {
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
   */
  async list(): Promise<components["schemas"]["ListCampfiresResponseContent"]> {
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