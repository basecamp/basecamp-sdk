/**
 * Templates service for the Basecamp API.
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

/** Template entity from the Basecamp API. */
export type Template = components["schemas"]["Template"];

/**
 * Request parameters for createLibraryCopy.
 */
export interface CreateLibraryCopyTemplateRequest {
  /** Template recording id */
  templateRecordingId: number;
  /** Destination parent id */
  destinationParentId: number;
  /** Confirm granting destination-project access to people referenced by the template. */
  addingPeopleConfirmed?: boolean;
}

/**
 * Options for list.
 */
export interface ListTemplateOptions extends PaginationOptions {
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for create.
 */
export interface CreateTemplateRequest {
  /** Display name */
  name: string;
  /** Rich text description (HTML) */
  description?: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateTemplateRequest {
  /** Display name */
  name?: string;
  /** Rich text description (HTML) */
  description?: string;
}

/**
 * Request parameters for createProject.
 */
export interface CreateProjectTemplateRequest {
  /** Project */
  project: components["schemas"]["ProjectConstructionAttributes"];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Templates operations.
 */
export class TemplatesService extends BaseService {

  /**
   * Get the account's to-do list template library
   * @returns The template_library
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.templates.getLibrary();
   * ```
   */
  async getLibrary(): Promise<components["schemas"]["GetTemplateLibraryResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetTemplateLibrary",
        resourceType: "template_library",
        isMutation: false,
      },
      () =>
        this.client.GET("/template_library.json", {
        })
    );
    return response;
  }

  /**
   * Start copying a to-do list template into a project
   * @param req - Template_library_copy creation parameters
   * @returns The template_library_copy
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.createLibraryCopy({ templateRecordingId: 1, destinationParentId: 1 });
   * ```
   */
  async createLibraryCopy(req: CreateLibraryCopyTemplateRequest): Promise<components["schemas"]["CreateTemplateLibraryCopyResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateTemplateLibraryCopy",
        resourceType: "template_library_copy",
        isMutation: true,
      },
      () =>
        this.client.POST("/template_library/copies.json", {
          body: {
            template_recording_id: req.templateRecordingId,
            destination_parent_id: req.destinationParentId,
            adding_people_confirmed: req.addingPeopleConfirmed,
          },
        })
    );
    return response;
  }

  /**
   * Get the current status of a to-do list template copy
   * @param copyId - The copy ID
   * @returns The template_library_copy
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.templates.getLibraryCopy(123);
   * ```
   */
  async getLibraryCopy(copyId: number): Promise<components["schemas"]["GetTemplateLibraryCopyResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetTemplateLibraryCopy",
        resourceType: "template_library_copy",
        isMutation: false,
        resourceId: copyId,
      },
      () =>
        this.client.GET("/template_library/copies/{copyId}", {
          params: {
            path: { copyId },
          },
        })
    );
    return response;
  }

  /**
   * List all templates visible to the current user
   * @param options - Optional query parameters
   * @returns All Template across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.templates.list();
   *
   * // With options
   * const filtered = await client.templates.list({ status: "active" });
   * ```
   */
  async list(options?: ListTemplateOptions): Promise<ListResult<Template>> {
    return this.requestPaginated(
      {
        service: "Templates",
        operation: "ListTemplates",
        resourceType: "template",
        isMutation: false,
      },
      () =>
        this.client.GET("/templates.json", {
          params: {
            query: { status: options?.status, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Create a new template
   * @param req - Template creation parameters
   * @returns The Template
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.create({ name: "My example" });
   * ```
   */
  async create(req: CreateTemplateRequest): Promise<Template> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateTemplate",
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
   * Get a single template by id
   * @param templateId - The template ID
   * @returns The Template
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.templates.get(123);
   * ```
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
   * @param req - Template update parameters
   * @returns The Template
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.update(123, { });
   * ```
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
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );
    return response;
  }

  /**
   * Delete a template (trash it)
   * @param templateId - The template ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.templates.delete(123);
   * ```
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
   * @param req - Project_from_template creation parameters
   * @returns The project_from_template
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.createProject(123, { project: { name: "My example" } });
   * ```
   */
  async createProject(templateId: number, req: CreateProjectTemplateRequest): Promise<components["schemas"]["CreateProjectFromTemplateResponseContent"]> {
    if (!req.project) {
      throw Errors.validation("Project is required");
    }
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
          body: {
            project: req.project,
          },
        })
    );
    return response;
  }

  /**
   * Get the status of a project construction
   * @param templateId - The template ID
   * @param constructionId - The construction ID
   * @returns The project_construction
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.templates.getConstruction(123, 123);
   * ```
   */
  async getConstruction(templateId: number, constructionId: number): Promise<components["schemas"]["GetProjectConstructionResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetProjectConstruction",
        resourceType: "project_construction",
        isMutation: false,
        resourceId: constructionId,
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