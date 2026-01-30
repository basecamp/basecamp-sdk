/**
 * Service for Templates operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Templates operations
 */
export class TemplatesService extends BaseService {

  /**
   * List all templates visible to the current user
   */
  async list(options?: { status?: string }): Promise<components["schemas"]["ListTemplatesResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "ListTemplates",
        resourceType: "template",
        isMutation: false,
      },
      () =>
        this.client.GET("/templates.json", {
          params: {
            query: { status: options?.status },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new template
   */
  async create(req: components["schemas"]["CreateTemplateRequestContent"]): Promise<components["schemas"]["CreateTemplateResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateTemplate",
        resourceType: "template",
        isMutation: true,
      },
      () =>
        this.client.POST("/templates.json", {
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a single template by id
   */
  async get(templateId: number): Promise<components["schemas"]["GetTemplateResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetTemplate",
        resourceType: "template",
        isMutation: false,
        resourceId: templateId,
      },
      () =>
        this.client.GET("/templates/{templateId}", {
          params: {
            path: { templateId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing template
   */
  async update(templateId: number, req: components["schemas"]["UpdateTemplateRequestContent"]): Promise<components["schemas"]["UpdateTemplateResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "UpdateTemplate",
        resourceType: "template",
        isMutation: true,
        resourceId: templateId,
      },
      () =>
        this.client.PUT("/templates/{templateId}", {
          params: {
            path: { templateId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Delete a template (trash it)
   */
  async delete(templateId: number): Promise<void> {
    await this.request(
      {
        service: "Templates",
        operation: "DeleteTemplate",
        resourceType: "template",
        isMutation: true,
        resourceId: templateId,
      },
      () =>
        this.client.DELETE("/templates/{templateId}", {
          params: {
            path: { templateId },
          },
        })
    );
  }

  /**
   * Create a project from a template (asynchronous)
   */
  async createProject(templateId: number, req: components["schemas"]["CreateProjectFromTemplateRequestContent"]): Promise<components["schemas"]["CreateProjectFromTemplateResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateProjectFromTemplate",
        resourceType: "project_from_template",
        isMutation: true,
        resourceId: templateId,
      },
      () =>
        this.client.POST("/templates/{templateId}/project_constructions.json", {
          params: {
            path: { templateId },
          },
          body: req,
        })
    );
    return response;
  }

  /**
   * Get the status of a project construction
   */
  async getConstruction(templateId: number, constructionId: number): Promise<components["schemas"]["GetProjectConstructionResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetProjectConstruction",
        resourceType: "project_construction",
        isMutation: false,
        resourceId: templateId,
      },
      () =>
        this.client.GET("/templates/{templateId}/project_constructions/{constructionId}", {
          params: {
            path: { templateId, constructionId },
          },
        })
    );
    return response;
  }
}