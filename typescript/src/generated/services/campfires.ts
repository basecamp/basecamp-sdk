/**
 * Campfires service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Chatbot entity from the Basecamp API. */
export type Chatbot = components["schemas"]["Chatbot"];
/** Campfire entity from the Basecamp API. */
export type Campfire = components["schemas"]["Campfire"];
/** CampfireLine entity from the Basecamp API. */
export type CampfireLine = components["schemas"]["CampfireLine"];

/**
 * Options for listChatbots.
 */
export interface ListChatbotsCampfireOptions extends PaginationOptions {
}

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
 * Options for list.
 */
export interface ListCampfireOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8. */
  page?: number;
}

/**
 * Options for listLines.
 */
export interface ListLinesCampfireOptions extends PaginationOptions {
  /** Filter by sort */
  sort?: "created_at" | "updated_at";
  /** Filter by direction */
  direction?: "asc" | "desc";
  /** Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for createLine.
 */
export interface CreateLineCampfireRequest {
  /** Text content */
  content: string;
  /** Content type */
  contentType?: string;
}

/**
 * Request parameters for updateLine.
 */
export interface UpdateLineCampfireRequest {
  /** The new line content, interpreted as rich text (HTML) */
  content: string;
}

/**
 * Options for listUploads.
 */
export interface ListUploadsCampfireOptions extends PaginationOptions {
  /** Filter by sort */
  sort?: "created_at" | "updated_at";
  /** Filter by direction */
  direction?: "asc" | "desc";
  /** Page number for paginating through results. Defaults to 1. Semantics vary by SDK; see SPEC section 8. */
  page?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Campfires operations.
 */
export class CampfiresService extends BaseService {

  /**
   * List all chatbots for a campfire
   * @param bucketId - The bucket ID
   * @param campfireId - The campfire ID
   * @param options - Optional query parameters
   * @returns All Chatbot across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.campfires.listChatbots(123, 123);
   * ```
   */
  async listChatbots(bucketId: number, campfireId: number, options?: ListChatbotsCampfireOptions): Promise<ListResult<Chatbot>> {
    return this.requestPaginated(
      {
        service: "Campfires",
        operation: "ListChatbots",
        resourceType: "chatbot",
        isMutation: false,
        projectId: bucketId,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/buckets/{bucketId}/chats/{campfireId}/integrations.json", {
          params: {
            path: { bucketId, campfireId },
          },
        })
      , options
    );
  }

  /**
   * Create a new chatbot for a campfire
   * @param bucketId - The bucket ID
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
  async createChatbot(bucketId: number, campfireId: number, req: CreateChatbotCampfireRequest): Promise<Chatbot> {
    if (!req.serviceName) {
      throw Errors.validation("Service name is required");
    }
    const response = await this.request(
      {
        service: "Campfires",
        operation: "CreateChatbot",
        resourceType: "chatbot",
        isMutation: true,
        projectId: bucketId,
        resourceId: campfireId,
      },
      () =>
        this.client.POST("/buckets/{bucketId}/chats/{campfireId}/integrations.json", {
          params: {
            path: { bucketId, campfireId },
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
   * @param bucketId - The bucket ID
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
  async getChatbot(bucketId: number, campfireId: number, chatbotId: number): Promise<Chatbot> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "GetChatbot",
        resourceType: "chatbot",
        isMutation: false,
        projectId: bucketId,
        resourceId: chatbotId,
      },
      () =>
        this.client.GET("/buckets/{bucketId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: {
            path: { bucketId, campfireId, chatbotId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing chatbot
   * @param bucketId - The bucket ID
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
  async updateChatbot(bucketId: number, campfireId: number, chatbotId: number, req: UpdateChatbotCampfireRequest): Promise<Chatbot> {
    if (!req.serviceName) {
      throw Errors.validation("Service name is required");
    }
    const response = await this.request(
      {
        service: "Campfires",
        operation: "UpdateChatbot",
        resourceType: "chatbot",
        isMutation: true,
        projectId: bucketId,
        resourceId: chatbotId,
      },
      () =>
        this.client.PUT("/buckets/{bucketId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: {
            path: { bucketId, campfireId, chatbotId },
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
   * @param bucketId - The bucket ID
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
  async deleteChatbot(bucketId: number, campfireId: number, chatbotId: number): Promise<void> {
    await this.request(
      {
        service: "Campfires",
        operation: "DeleteChatbot",
        resourceType: "chatbot",
        isMutation: true,
        projectId: bucketId,
        resourceId: chatbotId,
      },
      () =>
        this.client.DELETE("/buckets/{bucketId}/chats/{campfireId}/integrations/{chatbotId}", {
          params: {
            path: { bucketId, campfireId, chatbotId },
          },
        })
    );
  }

  /**
   * List all campfires across the account
   * @param options - Optional query parameters
   * @returns All Campfire across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.campfires.list();
   *
   * // With options
   * const filtered = await client.campfires.list({ page: 1 });
   * ```
   */
  async list(options?: ListCampfireOptions): Promise<ListResult<Campfire>> {
    return this.requestPaginated(
      {
        service: "Campfires",
        operation: "ListCampfires",
        resourceType: "campfire",
        isMutation: false,
      },
      () =>
        this.client.GET("/chats.json", {
          params: {
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get a campfire by ID
   * @param campfireId - The campfire ID
   * @returns The Campfire
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.campfires.get(123);
   * ```
   */
  async get(campfireId: number): Promise<Campfire> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "GetCampfire",
        resourceType: "campfire",
        isMutation: false,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/chats/{campfireId}", {
          params: {
            path: { campfireId },
          },
        })
    );
    return response;
  }

  /**
   * List all lines (messages) in a campfire
   * @param campfireId - The campfire ID
   * @param options - Optional query parameters
   * @returns All CampfireLine across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.campfires.listLines(123);
   *
   * // With options
   * const filtered = await client.campfires.listLines(123, { sort: "created_at" });
   * ```
   */
  async listLines(campfireId: number, options?: ListLinesCampfireOptions): Promise<ListResult<CampfireLine>> {
    return this.requestPaginated(
      {
        service: "Campfires",
        operation: "ListCampfireLines",
        resourceType: "campfire_line",
        isMutation: false,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/chats/{campfireId}/lines.json", {
          params: {
            path: { campfireId },
            query: { sort: options?.sort, direction: options?.direction, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Create a new line (message) in a campfire
   * @param campfireId - The campfire ID
   * @param req - Campfire_line creation parameters
   * @returns The CampfireLine
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.campfires.createLine(123, { content: "Hello world" });
   * ```
   */
  async createLine(campfireId: number, req: CreateLineCampfireRequest): Promise<CampfireLine> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    const response = await this.request(
      {
        service: "Campfires",
        operation: "CreateCampfireLine",
        resourceType: "campfire_line",
        isMutation: true,
        resourceId: campfireId,
      },
      () =>
        this.client.POST("/chats/{campfireId}/lines.json", {
          params: {
            path: { campfireId },
          },
          body: {
            content: req.content,
            content_type: req.contentType,
          },
        })
    );
    return response;
  }

  /**
   * Get a campfire line by ID
   * @param campfireId - The campfire ID
   * @param lineId - The line ID
   * @returns The CampfireLine
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.campfires.getLine(123, 123);
   * ```
   */
  async getLine(campfireId: number, lineId: number): Promise<CampfireLine> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "GetCampfireLine",
        resourceType: "campfire_line",
        isMutation: false,
        resourceId: lineId,
      },
      () =>
        this.client.GET("/chats/{campfireId}/lines/{lineId}", {
          params: {
            path: { campfireId, lineId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing campfire line; the content is always treated as rich text (HTML).
   * @param campfireId - The campfire ID
   * @param lineId - The line ID
   * @param req - Campfire_line update parameters
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.campfires.updateLine(123, 123, { content: "Hello world" });
   * ```
   */
  async updateLine(campfireId: number, lineId: number, req: UpdateLineCampfireRequest): Promise<void> {
    if (!req.content) {
      throw Errors.validation("Content is required");
    }
    await this.request(
      {
        service: "Campfires",
        operation: "UpdateCampfireLine",
        resourceType: "campfire_line",
        isMutation: true,
        resourceId: lineId,
      },
      () =>
        this.client.PUT("/chats/{campfireId}/lines/{lineId}", {
          params: {
            path: { campfireId, lineId },
          },
          body: {
            content: req.content,
          },
        })
    );
  }

  /**
   * Delete a campfire line; allowed for the line's creator or an admin.
   * @param campfireId - The campfire ID
   * @param lineId - The line ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.campfires.deleteLine(123, 123);
   * ```
   */
  async deleteLine(campfireId: number, lineId: number): Promise<void> {
    await this.request(
      {
        service: "Campfires",
        operation: "DeleteCampfireLine",
        resourceType: "campfire_line",
        isMutation: true,
        resourceId: lineId,
      },
      () =>
        this.client.DELETE("/chats/{campfireId}/lines/{lineId}", {
          params: {
            path: { campfireId, lineId },
          },
        })
    );
  }

  /**
   * List uploaded files in a campfire
   * @param campfireId - The campfire ID
   * @param options - Optional query parameters
   * @returns All CampfireLine across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.campfires.listUploads(123);
   *
   * // With options
   * const filtered = await client.campfires.listUploads(123, { sort: "created_at" });
   * ```
   */
  async listUploads(campfireId: number, options?: ListUploadsCampfireOptions): Promise<ListResult<CampfireLine>> {
    return this.requestPaginated(
      {
        service: "Campfires",
        operation: "ListCampfireUploads",
        resourceType: "campfire_upload",
        isMutation: false,
        resourceId: campfireId,
      },
      () =>
        this.client.GET("/chats/{campfireId}/uploads.json", {
          params: {
            path: { campfireId },
            query: { sort: options?.sort, direction: options?.direction, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Upload a file to a campfire
   * @param campfireId - The campfire ID
   * @param data - Binary file data to upload
   * @param contentType - MIME type of the file (e.g., "image/png", "application/pdf")
   * @param name - Filename for the uploaded file (e.g. "report.pdf").
   * @returns The CampfireLine
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.campfires.createUpload(123, fileData, "image/png", "name");
   * ```
   */
  async createUpload(campfireId: number, data: ArrayBuffer | Uint8Array | string, contentType: string, name: string): Promise<CampfireLine> {
    const response = await this.request(
      {
        service: "Campfires",
        operation: "CreateCampfireUpload",
        resourceType: "campfire_upload",
        isMutation: true,
        resourceId: campfireId,
      },
      () =>
        this.client.POST("/chats/{campfireId}/uploads.json", {
          params: {
            path: { campfireId },
            query: { name: name },
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            header: { "Content-Type": contentType } as any,
          },
          body: data as unknown as string,
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          bodySerializer: (body: unknown) => body as any,
        })
    );
    return response;
  }
}