/**
 * Templates service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Template entity from the Basecamp API. */
export type Template = components["schemas"]["Template"];

/**
 * Options for list.
 */
export interface ListTemplateOptions {
  /** active|archived|trashed */
  status?: string;
}

/**
 * Request parameters for create.
 */
export interface CreateTemplateRequest {
  /** name */
  name: string;
  /** description */
  description?: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateTemplateRequest {
  /** name */
  name?: string;
  /** description */
  description?: string;
}

/**
 * Request parameters for createProject.
 */
export interface CreateProjectTemplateRequest {
  /** name */
  name: string;
  /** description */
  description?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Templates operations.
 */
export class TemplatesService extends BaseService {

  /**
   * List all templates visible to the current user
   * @param options - Optional parameters
   * @returns Array of Template
   */
  async list(options?: ListTemplateOptions): Promise<Template[]> {
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
   * @param req - Request parameters
   * @returns The Template
   *
   * @example
   * ```ts
   * const result = await client.templates.create({ ... });
   * ```
   */
  async create(req: CreateTemplateRequest): Promise<Template> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateTemplate",
        resourceType: "template",
        isMutation: true,
      },
      () =>
        this.client.POST("/templates.json", {
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Get a single template by id
   * @param templateId - The template ID
   * @returns The Template
   */
  async get(templateId: number): Promise<Template> {
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
   * @param templateId - The template ID
   * @param req - Request parameters
   * @returns The Template
   */
  async update(templateId: number, req: UpdateTemplateRequest): Promise<Template> {
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
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Delete a template (trash it)
   * @param templateId - The template ID
   * @returns void
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
   * @param templateId - The template ID
   * @param req - Request parameters
   * @returns The project_from_template
   *
   * @example
   * ```ts
   * const result = await client.templates.createProject(123, { ... });
   * ```
   */
  async createProject(templateId: number, req: CreateProjectTemplateRequest): Promise<components["schemas"]["CreateProjectFromTemplateResponseContent"]> {
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
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Get the status of a project construction
   * @param templateId - The template ID
   * @param constructionId - The construction ID
   * @returns The project_construction
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