/**
 * Templates service for the Basecamp API.
 *
 * Templates allow you to create reusable project structures.
 *
 * @example
 * ```ts
 * const templates = await client.templates.list();
 * const template = await client.templates.get(templateId);
 * const construction = await client.templates.createProject(templateId, {
 *   name: "Q1 Planning",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp project template.
 */
export type Template = components["schemas"]["Template"];

/**
 * Status of a project being created from a template.
 */
export type ProjectConstruction = components["schemas"]["ProjectConstruction"];

/**
 * Request to create a new template.
 */
export interface CreateTemplateRequest {
  /** Template name (required) */
  name: string;
  /** Template description (optional) */
  description?: string;
}

/**
 * Request to update an existing template.
 */
export interface UpdateTemplateRequest {
  /** Template name (required) */
  name: string;
  /** Template description (optional) */
  description?: string;
}

/**
 * Request to create a project from a template.
 */
export interface CreateProjectFromTemplateRequest {
  /** Project name (required) */
  name: string;
  /** Project description (optional) */
  description?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp templates.
 */
export class TemplatesService extends BaseService {
  /**
   * Lists all templates visible to the current user.
   *
   * @returns Array of templates
   *
   * @example
   * ```ts
   * const templates = await client.templates.list();
   * for (const template of templates) {
   *   console.log(template.name);
   * }
   * ```
   */
  async list(): Promise<Template[]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "List",
        resourceType: "template",
        isMutation: false,
      },
      () => this.client.GET("/templates.json")
    );

    return response ?? [];
  }

  /**
   * Gets a template by ID.
   *
   * @param templateId - The template ID
   * @returns The template
   * @throws BasecampError with code "not_found" if template doesn't exist
   *
   * @example
   * ```ts
   * const template = await client.templates.get(templateId);
   * console.log(template.name, template.description);
   * ```
   */
  async get(templateId: number): Promise<Template> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "Get",
        resourceType: "template",
        isMutation: false,
        resourceId: templateId,
      },
      () =>
        this.client.GET("/templates/{templateId}", {
          params: { path: { templateId } },
        })
    );

    return response;
  }

  /**
   * Creates a new template.
   *
   * @param req - Template creation parameters
   * @returns The created template
   * @throws BasecampError with code "validation" if name is missing
   *
   * @example
   * ```ts
   * const template = await client.templates.create({
   *   name: "Marketing Campaign",
   *   description: "Standard marketing campaign project structure",
   * });
   * ```
   */
  async create(req: CreateTemplateRequest): Promise<Template> {
    if (!req.name) {
      throw Errors.validation("Template name is required");
    }

    const response = await this.request(
      {
        service: "Templates",
        operation: "Create",
        resourceType: "template",
        isMutation: true,
      },
      () =>
        this.client.POST("/templates.json", {
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );

    return response;
  }

  /**
   * Updates an existing template.
   *
   * @param templateId - The template ID
   * @param req - Template update parameters
   * @returns The updated template
   * @throws BasecampError with code "validation" if name is missing
   *
   * @example
   * ```ts
   * const template = await client.templates.update(templateId, {
   *   name: "Updated Template Name",
   *   description: "New description",
   * });
   * ```
   */
  async update(templateId: number, req: UpdateTemplateRequest): Promise<Template> {
    if (!req.name) {
      throw Errors.validation("Template name is required");
    }

    const response = await this.request(
      {
        service: "Templates",
        operation: "Update",
        resourceType: "template",
        isMutation: true,
        resourceId: templateId,
      },
      () =>
        this.client.PUT("/templates/{templateId}", {
          params: { path: { templateId } },
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );

    return response;
  }

  /**
   * Deletes a template (moves it to trash).
   *
   * @param templateId - The template ID
   *
   * @example
   * ```ts
   * await client.templates.delete(templateId);
   * ```
   */
  async delete(templateId: number): Promise<void> {
    await this.request(
      {
        service: "Templates",
        operation: "Delete",
        resourceType: "template",
        isMutation: true,
        resourceId: templateId,
      },
      () =>
        this.client.DELETE("/templates/{templateId}", {
          params: { path: { templateId } },
        })
    );
  }

  /**
   * Creates a new project from a template.
   *
   * This operation is asynchronous. Use `getConstruction` to check the status
   * of the project creation.
   *
   * @param templateId - The template ID
   * @param req - Project creation parameters
   * @returns The project construction status
   * @throws BasecampError with code "validation" if name is missing
   *
   * @example
   * ```ts
   * const construction = await client.templates.createProject(templateId, {
   *   name: "Q1 Marketing Campaign",
   *   description: "Campaign for Q1 2024",
   * });
   *
   * // Check status
   * const status = await client.templates.getConstruction(
   *   templateId,
   *   construction.id
   * );
   * if (status.status === "completed") {
   *   console.log("Project created:", status.project?.name);
   * }
   * ```
   */
  async createProject(
    templateId: number,
    req: CreateProjectFromTemplateRequest
  ): Promise<ProjectConstruction> {
    if (!req.name) {
      throw Errors.validation("Project name is required");
    }

    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateProject",
        resourceType: "project_construction",
        isMutation: true,
        resourceId: templateId,
      },
      () =>
        this.client.POST("/templates/{templateId}/project_constructions.json", {
          params: { path: { templateId } },
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );

    return response;
  }

  /**
   * Gets the status of a project construction.
   *
   * @param templateId - The template ID
   * @param constructionId - The construction ID
   * @returns The project construction status
   *
   * @example
   * ```ts
   * const status = await client.templates.getConstruction(templateId, constructionId);
   *
   * switch (status.status) {
   *   case "pending":
   *     console.log("Project is being created...");
   *     break;
   *   case "completed":
   *     console.log("Project ready:", status.project?.id);
   *     break;
   *   case "failed":
   *     console.log("Project creation failed");
   *     break;
   * }
   * ```
   */
  async getConstruction(templateId: number, constructionId: number): Promise<ProjectConstruction> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetConstruction",
        resourceType: "project_construction",
        isMutation: false,
        resourceId: constructionId,
      },
      () =>
        this.client.GET("/templates/{templateId}/project_constructions/{constructionId}", {
          params: { path: { templateId, constructionId } },
        })
    );

    return response;
  }
}
