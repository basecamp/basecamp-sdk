/**
 * Projects service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Project entity from the Basecamp API. */
export type Project = components["schemas"]["Project"];

/**
 * Options for list.
 */
export interface ListProjectOptions {
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
}

/**
 * Request parameters for create.
 */
export interface CreateProjectRequest {
  /** Display name */
  name: string;
  /** Rich text description (HTML) */
  description?: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateProjectRequest {
  /** Display name */
  name: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Admissions */
  admissions?: "invite" | "employee" | "team";
  /** Schedule date range settings */
  scheduleAttributes?: components["schemas"]["ScheduleAttributes"];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Projects operations.
 */
export class ProjectsService extends BaseService {

  /**
   * List projects (active by default; optionally archived/trashed)
   * @param options - Optional query parameters
   * @returns Array of Project
   *
   * @example
   * ```ts
   * const result = await client.projects.list();
   *
   * // With options
   * const filtered = await client.projects.list({ status: "active" });
   * ```
   */
  async list(options?: ListProjectOptions): Promise<Project[]> {
    const response = await this.request(
      {
        service: "Projects",
        operation: "ListProjects",
        resourceType: "project",
        isMutation: false,
      },
      () =>
        this.client.GET("/projects.json", {
          params: {
            query: { status: options?.status },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new project
   * @param req - Project creation parameters
   * @returns The Project
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.projects.create({ name: "My example" });
   * ```
   */
  async create(req: CreateProjectRequest): Promise<Project> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Projects",
        operation: "CreateProject",
        resourceType: "project",
        isMutation: true,
      },
      () =>
        this.client.POST("/projects.json", {
          body: {
            name: req.name,
            description: req.description,
          },
        })
    );
    return response;
  }

  /**
   * Get a single project by id
   * @param projectId - The project ID
   * @returns The Project
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.projects.get(123);
   * ```
   */
  async get(projectId: number): Promise<Project> {
    const response = await this.request(
      {
        service: "Projects",
        operation: "GetProject",
        resourceType: "project",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/projects/{projectId}", {
          params: {
            path: { projectId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing project
   * @param projectId - The project ID
   * @param req - Project update parameters
   * @returns The Project
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.projects.update(123, { name: "My example" });
   * ```
   */
  async update(projectId: number, req: UpdateProjectRequest): Promise<Project> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Projects",
        operation: "UpdateProject",
        resourceType: "project",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.PUT("/projects/{projectId}", {
          params: {
            path: { projectId },
          },
          body: {
            name: req.name,
            description: req.description,
            admissions: req.admissions,
            schedule_attributes: req.scheduleAttributes,
          },
        })
    );
    return response;
  }

  /**
   * Trash a project. Trashed items can be recovered.
   * @param projectId - The project ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.projects.trash(123);
   * ```
   */
  async trash(projectId: number): Promise<void> {
    await this.request(
      {
        service: "Projects",
        operation: "TrashProject",
        resourceType: "project",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.DELETE("/projects/{projectId}", {
          params: {
            path: { projectId },
          },
        })
    );
  }
}