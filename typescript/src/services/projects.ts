/**
 * Projects service for the Basecamp API.
 *
 * Projects (also called "buckets" in the API) are the top-level
 * containers for all content in Basecamp.
 *
 * @example
 * ```ts
 * const projects = await client.projects.list();
 * const project = await client.projects.get(12345);
 * const newProject = await client.projects.create({ name: "My Project" });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp project.
 */
export type Project = components["schemas"]["Project"];

/**
 * A tool in a project's dock.
 */
export type DockItem = components["schemas"]["DockItem"];

/**
 * Valid project statuses.
 */
export type ProjectStatus = "active" | "archived" | "trashed";

/**
 * Options for listing projects.
 */
export interface ProjectListOptions {
  /** Filter by project status. Defaults to active. */
  status?: ProjectStatus;
}

/**
 * Request to create a new project.
 */
export interface CreateProjectRequest {
  /** Project name (required) */
  name: string;
  /** Project description (optional) */
  description?: string;
}

/**
 * Request to update an existing project.
 */
export interface UpdateProjectRequest {
  /** Project name (required for update) */
  name: string;
  /** Project description (optional) */
  description?: string;
  /** Access policy: "invite", "employee", or "team" (optional) */
  admissions?: string;
  /** Project schedule dates (optional) */
  scheduleAttributes?: {
    /** Start date in ISO 8601 format (YYYY-MM-DD) */
    startDate: string;
    /** End date in ISO 8601 format (YYYY-MM-DD) */
    endDate: string;
  };
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp projects.
 */
export class ProjectsService extends BaseService {
  /**
   * Lists all projects visible to the current user.
   * By default, returns active projects sorted by most recently created.
   *
   * @param options - Optional filters
   * @returns Array of projects
   *
   * @example
   * ```ts
   * // List active projects
   * const projects = await client.projects.list();
   *
   * // List archived projects
   * const archived = await client.projects.list({ status: "archived" });
   * ```
   */
  async list(options?: ProjectListOptions): Promise<Project[]> {
    const response = await this.request(
      {
        service: "Projects",
        operation: "List",
        resourceType: "project",
        isMutation: false,
      },
      () =>
        this.client.GET("/projects.json", {
          params: { query: options?.status ? { status: options.status } : undefined },
        })
    );

    return response ?? [];
  }

  /**
   * Gets a project by ID.
   *
   * @param id - The project ID
   * @returns The project
   * @throws BasecampError with code "not_found" if project doesn't exist
   *
   * @example
   * ```ts
   * const project = await client.projects.get(12345);
   * console.log(project.name);
   * ```
   */
  async get(id: number): Promise<Project> {
    const response = await this.request(
      {
        service: "Projects",
        operation: "Get",
        resourceType: "project",
        isMutation: false,
        resourceId: id,
      },
      () =>
        this.client.GET("/projects/{projectId}", {
          params: { path: { projectId: id } },
        })
    );

    return response;
  }

  /**
   * Creates a new project.
   *
   * @param req - Project creation parameters
   * @returns The created project
   * @throws BasecampError with code "validation" if name is missing
   *
   * @example
   * ```ts
   * const project = await client.projects.create({
   *   name: "My New Project",
   *   description: "A great project",
   * });
   * ```
   */
  async create(req: CreateProjectRequest): Promise<Project> {
    if (!req.name) {
      throw Errors.validation("Project name is required");
    }

    const response = await this.request(
      {
        service: "Projects",
        operation: "Create",
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
   * Updates an existing project.
   *
   * @param id - The project ID
   * @param req - Project update parameters
   * @returns The updated project
   * @throws BasecampError with code "validation" if name is missing
   *
   * @example
   * ```ts
   * const project = await client.projects.update(12345, {
   *   name: "Updated Name",
   *   description: "New description",
   * });
   * ```
   */
  async update(id: number, req: UpdateProjectRequest): Promise<Project> {
    if (!req.name) {
      throw Errors.validation("Project name is required");
    }

    const body: {
      name: string;
      description?: string;
      admissions?: string;
      schedule_attributes?: { start_date: string; end_date: string };
    } = {
      name: req.name,
      description: req.description,
      admissions: req.admissions,
    };

    if (req.scheduleAttributes) {
      body.schedule_attributes = {
        start_date: req.scheduleAttributes.startDate,
        end_date: req.scheduleAttributes.endDate,
      };
    }

    const response = await this.request(
      {
        service: "Projects",
        operation: "Update",
        resourceType: "project",
        isMutation: true,
        resourceId: id,
      },
      () =>
        this.client.PUT("/projects/{projectId}", {
          params: { path: { projectId: id } },
          body,
        })
    );

    return response;
  }

  /**
   * Moves a project to the trash.
   * Trashed projects are deleted after 30 days.
   *
   * @param id - The project ID
   *
   * @example
   * ```ts
   * await client.projects.trash(12345);
   * ```
   */
  async trash(id: number): Promise<void> {
    await this.request(
      {
        service: "Projects",
        operation: "Trash",
        resourceType: "project",
        isMutation: true,
        resourceId: id,
      },
      () =>
        this.client.DELETE("/projects/{projectId}", {
          params: { path: { projectId: id } },
        })
    );
  }
}
