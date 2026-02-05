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
   * @param projectId - The project ID
   * @param req - Tool request parameters
   * @returns The Tool
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.tools.clone(123, { sourceRecordingId: 1 });
   * ```
   */
  async clone(projectId: number, req: CloneToolRequest): Promise<Tool> {
    const response = await this.request(
      {
        service: "Tools",
        operation: "CloneTool",
        resourceType: "tool",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/dock/tools.json", {
          params: {
            path: { projectId },
          },
          body: {
            source_recording_id: req.sourceRecordingId,
          },
        })
    );
    return response;
  }

  /**
   * Get a dock tool by id
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @returns The Tool
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.tools.get(123, 123);
   * ```
   */
  async get(projectId: number, toolId: number): Promise<Tool> {
    const response = await this.request(
      {
        service: "Tools",
        operation: "GetTool",
        resourceType: "tool",
        isMutation: false,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/dock/tools/{toolId}", {
          params: {
            path: { projectId, toolId },
          },
        })
    );
    return response;
  }

  /**
   * Update (rename) an existing tool
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @param req - Tool update parameters
   * @returns The Tool
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.tools.update(123, 123, { title: "example" });
   * ```
   */
  async update(projectId: number, toolId: number, req: UpdateToolRequest): Promise<Tool> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    const response = await this.request(
      {
        service: "Tools",
        operation: "UpdateTool",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/dock/tools/{toolId}", {
          params: {
            path: { projectId, toolId },
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
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.delete(123, 123);
   * ```
   */
  async delete(projectId: number, toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "DeleteTool",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/dock/tools/{toolId}", {
          params: {
            path: { projectId, toolId },
          },
        })
    );
  }

  /**
   * Enable a tool (show it on the project dock)
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.enable(123, 123);
   * ```
   */
  async enable(projectId: number, toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "EnableTool",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/recordings/{toolId}/position.json", {
          params: {
            path: { projectId, toolId },
          },
        })
    );
  }

  /**
   * Reposition a tool on the project dock
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @param req - Tool request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.reposition(123, 123, { position: 1 });
   * ```
   */
  async reposition(projectId: number, toolId: number, req: RepositionToolRequest): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "RepositionTool",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{toolId}/position.json", {
          params: {
            path: { projectId, toolId },
          },
          body: {
            position: req.position,
          },
        })
    );
  }

  /**
   * Disable a tool (hide it from the project dock)
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.tools.disable(123, 123);
   * ```
   */
  async disable(projectId: number, toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "DisableTool",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/recordings/{toolId}/position.json", {
          params: {
            path: { projectId, toolId },
          },
        })
    );
  }
}