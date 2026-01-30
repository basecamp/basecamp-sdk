/**
 * Service for Tools operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Tools operations
 */
export class ToolsService extends BaseService {

  /**
   * Clone an existing tool to create a new one
   */
  async clone(projectId: number, sourceToolId: number): Promise<components["schemas"]["CloneToolResponseContent"]> {
    const response = await this.request(
      {
        service: "Tools",
        operation: "CloneTool",
        resourceType: "tool",
        isMutation: true,
        projectId,
        resourceId: sourceToolId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/dock/tools/{sourceToolId}/clone.json", {
          params: {
            path: { projectId, sourceToolId },
          },
        })
    );
    return response;
  }

  /**
   * Get a dock tool by id
   */
  async get(projectId: number, toolId: number): Promise<components["schemas"]["GetToolResponseContent"]> {
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
   */
  async update(projectId: number, toolId: number, req: components["schemas"]["UpdateToolRequestContent"]): Promise<components["schemas"]["UpdateToolResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Delete a tool (trash it)
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
        this.client.POST("/buckets/{projectId}/dock/tools/{toolId}/position.json", {
          params: {
            path: { projectId, toolId },
          },
        })
    );
  }

  /**
   * Reposition a tool on the project dock
   */
  async reposition(projectId: number, toolId: number, req: components["schemas"]["RepositionToolRequestContent"]): Promise<void> {
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
        this.client.PUT("/buckets/{projectId}/dock/tools/{toolId}/position.json", {
          params: {
            path: { projectId, toolId },
          },
          body: req,
        })
    );
  }

  /**
   * Disable a tool (hide it from the project dock)
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
        this.client.DELETE("/buckets/{projectId}/dock/tools/{toolId}/position.json", {
          params: {
            path: { projectId, toolId },
          },
        })
    );
  }
}