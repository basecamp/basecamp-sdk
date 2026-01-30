/**
 * Tools service for the Basecamp API.
 *
 * Tools are dock items in a Basecamp project (e.g., Message Board,
 * Todos, Schedule, etc.). This service allows you to manage these tools.
 *
 * @example
 * ```ts
 * const tool = await client.tools.get(projectId, toolId);
 * await client.tools.enable(projectId, toolId);
 * await client.tools.reposition(projectId, toolId, 1);
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A dock tool in a Basecamp project.
 */
export type Tool = components["schemas"]["Tool"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp project dock tools.
 */
export class ToolsService extends BaseService {
  /**
   * Gets a tool by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param toolId - The tool ID
   * @returns The tool
   * @throws BasecampError with code "not_found" if tool doesn't exist
   *
   * @example
   * ```ts
   * const tool = await client.tools.get(projectId, toolId);
   * console.log(tool.name, tool.enabled);
   * ```
   */
  async get(projectId: number, toolId: number): Promise<Tool> {
    const response = await this.request(
      {
        service: "Tools",
        operation: "Get",
        resourceType: "tool",
        isMutation: false,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/dock/tools/{toolId}", {
          params: { path: { projectId, toolId } },
        })
    );

    return response;
  }

  /**
   * Clones an existing tool to create a new one.
   *
   * @param projectId - The project (bucket) ID
   * @param sourceToolId - The ID of the tool to clone
   * @returns The newly created tool
   *
   * @example
   * ```ts
   * // Clone a todolist to create a new one
   * const newTool = await client.tools.clone(projectId, existingToolId);
   * console.log("Created new tool:", newTool.id);
   * ```
   */
  async clone(projectId: number, sourceToolId: number): Promise<Tool> {
    const response = await this.request(
      {
        service: "Tools",
        operation: "Clone",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: sourceToolId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/dock/tools/{sourceToolId}/clone.json", {
          params: { path: { projectId, sourceToolId } },
        })
    );

    return response;
  }

  /**
   * Updates (renames) an existing tool.
   *
   * @param projectId - The project (bucket) ID
   * @param toolId - The tool ID
   * @param title - The new title for the tool
   * @returns The updated tool
   * @throws BasecampError with code "validation" if title is missing
   *
   * @example
   * ```ts
   * const tool = await client.tools.update(projectId, toolId, "Sprint Backlog");
   * ```
   */
  async update(projectId: number, toolId: number, title: string): Promise<Tool> {
    if (!title) {
      throw Errors.validation("Tool title is required");
    }

    const response = await this.request(
      {
        service: "Tools",
        operation: "Update",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/dock/tools/{toolId}", {
          params: { path: { projectId, toolId } },
          body: {
            title,
          },
        })
    );

    return response;
  }

  /**
   * Deletes a tool (moves it to trash).
   * Trashed tools can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param toolId - The tool ID
   *
   * @example
   * ```ts
   * await client.tools.delete(projectId, toolId);
   * ```
   */
  async delete(projectId: number, toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "Delete",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/dock/tools/{toolId}", {
          params: { path: { projectId, toolId } },
        })
    );
  }

  /**
   * Enables a tool (shows it on the project dock).
   * The tool will be placed at the end of the dock.
   *
   * @param projectId - The project (bucket) ID
   * @param toolId - The tool ID
   *
   * @example
   * ```ts
   * await client.tools.enable(projectId, toolId);
   * ```
   */
  async enable(projectId: number, toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "Enable",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/dock/tools/{toolId}/position.json", {
          params: { path: { projectId, toolId } },
        })
    );
  }

  /**
   * Disables a tool (hides it from the project dock).
   * The tool is not deleted, just hidden from the dock.
   *
   * @param projectId - The project (bucket) ID
   * @param toolId - The tool ID
   *
   * @example
   * ```ts
   * await client.tools.disable(projectId, toolId);
   * ```
   */
  async disable(projectId: number, toolId: number): Promise<void> {
    await this.request(
      {
        service: "Tools",
        operation: "Disable",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.DELETE("/buckets/{projectId}/dock/tools/{toolId}/position.json", {
          params: { path: { projectId, toolId } },
        })
    );
  }

  /**
   * Changes the position of a tool on the project dock.
   *
   * @param projectId - The project (bucket) ID
   * @param toolId - The tool ID
   * @param position - The new position (1-based, 1 = first position on dock)
   *
   * @example
   * ```ts
   * // Move tool to first position on dock
   * await client.tools.reposition(projectId, toolId, 1);
   * ```
   */
  async reposition(projectId: number, toolId: number, position: number): Promise<void> {
    if (position < 1) {
      throw Errors.validation("Position must be at least 1");
    }

    await this.request(
      {
        service: "Tools",
        operation: "Reposition",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: toolId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/dock/tools/{toolId}/position.json", {
          params: { path: { projectId, toolId } },
          body: {
            position,
          },
        })
    );
  }
}
