/**
 * Tools service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Tool entity from the Basecamp API. */
export type Tool = components["schemas"]["Tool"];

/**
 * Request parameters for clone.
 */
export interface CloneToolRequest {
  /** source recording id */
  sourceRecordingId: number;
}

/**
 * Request parameters for update.
 */
export interface UpdateToolRequest {
  /** title */
  title: string;
}

/**
 * Request parameters for reposition.
 */
export interface RepositionToolRequest {
  /** position */
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
   * @param req - Request parameters
   * @returns The Tool
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
   * @param req - Request parameters
   * @returns The Tool
   */
  async update(projectId: number, toolId: number, req: UpdateToolRequest): Promise<Tool> {
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
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Delete a tool (trash it)
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @returns void
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
   * @param req - Request parameters
   * @returns void
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
          body: req as any,
        })
    );
  }

  /**
   * Disable a tool (hide it from the project dock)
   * @param projectId - The project ID
   * @param toolId - The tool ID
   * @returns void
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