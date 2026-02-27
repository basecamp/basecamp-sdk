/**
 * Tools service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Tool entity from the Basecamp API. */
export type Tool = components["schemas"]["Tool"];

/**
 * Request parameters for clone.
 */
export interface CloneToolRequest {
  /** Source recording id */
  sourceRecordingId: number;
}

/**
 * Request parameters for update.
 */
export interface UpdateToolRequest {
  /** Title */
  title: string;
}

/**
 * Request parameters for reposition.
 */
export interface RepositionToolRequest {
  /** Position for ordering (1-based) */
  position: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Tools operations.
 */
export class ToolsService extends BaseService {

  /**
   * Clone an existing tool to create a new one
   * @param req - Tool request parameters
   * @returns The Tool
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.tools.clone({ sourceRecordingId: 1 });
   * ```
   */
  async clone(req: CloneToolRequest): Promise<Tool> {
    const response = await this.request(
      {
        service: "Tools",
        operation: "CloneTool",
        resourceType: "tool",
        isMutation: true,
      },
      () =>
        this.client.POST("/dock/tools.json", {
          body: {
            source_recording_id: req.sourceRecordingId,
          },
        })
    );
    return response;
  }

  /**
   * Get a dock tool by id
   * @param toolId - The tool ID
   * @returns The Tool
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.tools.get(123);
   * ```
   */
  async get(toolId: number): Promise<Tool> {
    const response = await this.request(
      {
        service: "Tools",
        operation: "GetTool",
        resourceType: "tool",
        isMutation: false,
        resourceId: toolId,
      },
      () =>
        this.client.GET("/dock/tools/{toolId}", {
          params: {
            path: { toolId },
          },
        })
    );
    return response;
  }

  /**
   * Update (rename) an existing tool
   * @param toolId - The tool ID
   * @param req - Tool update parameters
   * @returns The Tool
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.tools.update(123, { title: "example" });
   * ```
   */
  async update(toolId: number, req: UpdateToolRequest): Promise<Tool> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    const response = await this.request(
      {
        service: "Tools",
        operation: "UpdateTool",
        resourceType: "tool",
        isMutation: true,
        resourceId: toolId,
      },
      () =>
        this.client.PUT("/dock/tools/{toolId}", {
          params: {
            path: { toolId },
          },
          body: {
            title: req.title,
          },
        })
    );
    return response;
  }

  /**
   * Delete a tool (trash it)
   * @param toolId - The tool ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.delete(123);
   * ```
   */
  async delete(toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "DeleteTool",
        resourceType: "tool",
        isMutation: true,
        resourceId: toolId,
      },
      () =>
        this.client.DELETE("/dock/tools/{toolId}", {
          params: {
            path: { toolId },
          },
        })
    );
  }

  /**
   * Enable a tool (show it on the project dock)
   * @param toolId - The tool ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.enable(123);
   * ```
   */
  async enable(toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "EnableTool",
        resourceType: "tool",
        isMutation: true,
        resourceId: toolId,
      },
      () =>
        this.client.POST("/recordings/{toolId}/position.json", {
          params: {
            path: { toolId },
          },
        })
    );
  }

  /**
   * Reposition a tool on the project dock
   * @param toolId - The tool ID
   * @param req - Tool request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.reposition(123, { position: 1 });
   * ```
   */
  async reposition(toolId: number, req: RepositionToolRequest): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "RepositionTool",
        resourceType: "tool",
        isMutation: true,
        resourceId: toolId,
      },
      () =>
        this.client.PUT("/recordings/{toolId}/position.json", {
          params: {
            path: { toolId },
          },
          body: {
            position: req.position,
          },
        })
    );
  }

  /**
   * Disable a tool (hide it from the project dock)
   * @param toolId - The tool ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.disable(123);
   * ```
   */
  async disable(toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "DisableTool",
        resourceType: "tool",
        isMutation: true,
        resourceId: toolId,
      },
      () =>
        this.client.DELETE("/recordings/{toolId}/position.json", {
          params: {
            path: { toolId },
          },
        })
    );
  }
}