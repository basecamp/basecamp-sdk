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

/** CardTable entity from the Basecamp API. */
export type CardTable = components["schemas"]["CardTable"];
/** Todolist entity from the Basecamp API. */
export type Todolist = components["schemas"]["Todolist"];
/** Template entity from the Basecamp API. */
export type Template = components["schemas"]["Template"];

/**
 * Request parameters for createTemplatification.
 */
export interface CreateTemplatificationTemplateRequest {
  /** What to call the template. Defaults to the source recording's own title. */
  templateName?: string;
  /** Carry the comments across. */
  copyComments?: boolean;
  /** Carry assignees and the people involved across, adding them to the library
if they are not already there. */
  copyAssignments?: boolean;
  /** Gather the cards into Triage. Card tables only, and ignored otherwise. */
  moveCardsToTriage?: boolean;
}

/**
 * Request parameters for createLibraryCardTable.
 */
export interface CreateLibraryCardTableTemplateRequest {
  /** The template's name. Write-only: the response carries it as `title`. */
  name: string;
}

/**
 * Request parameters for createLibraryCopy.
 */
export interface CreateLibraryCopyTemplateRequest {
  /** The to-do list or card table in the library to copy. */
  templateRecordingId: number;
  /** The destination project. Basecamp resolves the container from the
template's kind, so a caller naming a project needs to know nothing about
docks or to-do sets. Supply this or destination_parent_id; if both are
sent, destination_parent_id wins. */
  destinationProjectId?: number;
  /** The container to copy into, for a caller that already holds one: the
project's to-do set for a to-do list template, or its dock for a card
table. A caller who may not edit it gets 404, not 403. */
  destinationParentId?: number;
  /** Confirm granting destination-project access to people referenced by the template. */
  addingPeopleConfirmed?: boolean;
}

/**
 * Request parameters for createLibraryTodolist.
 */
export interface CreateLibraryTodolistTemplateRequest {
  /** What to call the template. */
  name: string;
  /** Rich text describing the template. */
  description?: string;
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
   * Templatify a to-do list or card table
   * @param bucketId - The bucket ID
   * @param recordingId - The to-do list or card table to templatify. Anything else is a 403.
   * @param req - Templatification creation parameters
   * @returns The templatification
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.createTemplatification(123, 123, { });
   * ```
   */
  async createTemplatification(bucketId: number, recordingId: number, req: CreateTemplatificationTemplateRequest): Promise<components["schemas"]["CreateTemplatificationResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateTemplatification",
        resourceType: "templatification",
        isMutation: true,
        projectId: bucketId,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/buckets/{bucketId}/recordings/{recordingId}/templatifications.json", {
          params: {
            path: { bucketId, recordingId },
          },
          body: {
            template_name: req.templateName,
            copy_comments: req.copyComments,
            copy_assignments: req.copyAssignments,
            move_cards_to_triage: req.moveCardsToTriage,
          },
        })
    );
    return response;
  }

  /**
   * Get a templatification
   * @param bucketId - The bucket ID
   * @param recordingId - The recording ID
   * @param templatificationId - The templatification ID
   * @returns The templatification
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.templates.getTemplatification(123, 123, 123);
   * ```
   */
  async getTemplatification(bucketId: number, recordingId: number, templatificationId: number): Promise<components["schemas"]["GetTemplatificationResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetTemplatification",
        resourceType: "templatification",
        isMutation: false,
        projectId: bucketId,
        resourceId: templatificationId,
      },
      () =>
        this.client.GET("/buckets/{bucketId}/recordings/{recordingId}/templatifications/{templatificationId}", {
          params: {
            path: { bucketId, recordingId, templatificationId },
          },
        })
    );
    return response;
  }

  /**
   * Get the account's card table templates
   * @returns The template_library_card_table
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.templates.getLibraryCardTables();
   * ```
   */
  async getLibraryCardTables(): Promise<components["schemas"]["GetTemplateLibraryCardTablesResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetTemplateLibraryCardTables",
        resourceType: "template_library_card_table",
        isMutation: false,
      },
      () =>
        this.client.GET("/template_library/card_tables.json", {
        })
    );
    return response;
  }

  /**
   * Create a card table template with the default columns
   * @param req - Template_library_card_table creation parameters
   * @returns The CardTable
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.createLibraryCardTable({ name: "My example" });
   * ```
   */
  async createLibraryCardTable(req: CreateLibraryCardTableTemplateRequest): Promise<CardTable> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateTemplateLibraryCardTable",
        resourceType: "template_library_card_table",
        isMutation: true,
      },
      () =>
        this.client.POST("/template_library/card_tables.json", {
          body: {
            name: req.name,
          },
        })
    );
    return response;
  }

  /**
   * Start copying a to-do list or card table template into a project
   * @param req - Template_library_copy creation parameters
   * @returns The template_library_copy
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.createLibraryCopy({ templateRecordingId: 1 });
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
            destination_project_id: req.destinationProjectId,
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
   * Get the account's to-do list templates
   * @returns The template_library_todolist
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.templates.getLibraryTodolists();
   * ```
   */
  async getLibraryTodolists(): Promise<components["schemas"]["GetTemplateLibraryTodolistsResponseContent"]> {
    const response = await this.request(
      {
        service: "Templates",
        operation: "GetTemplateLibraryTodolists",
        resourceType: "template_library_todolist",
        isMutation: false,
      },
      () =>
        this.client.GET("/template_library/todolists.json", {
        })
    );
    return response;
  }

  /**
   * Create an empty to-do list template
   * @param req - Template_library_todolist creation parameters
   * @returns The Todolist
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.templates.createLibraryTodolist({ name: "My example" });
   * ```
   */
  async createLibraryTodolist(req: CreateLibraryTodolistTemplateRequest): Promise<Todolist> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Templates",
        operation: "CreateTemplateLibraryTodolist",
        resourceType: "template_library_todolist",
        isMutation: true,
      },
      () =>
        this.client.POST("/template_library/todolists.json", {
          body: {
            name: req.name,
            description: req.description,
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