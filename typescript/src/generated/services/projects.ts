/**
 * Projects service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Project entity from the Basecamp API. */
export type Project = components["schemas"]["Project"];

/**
 * Options for list.
 */
export interface ListProjectOptions {
  /** active|archived|trashed */
  status?: string;
}

/**
 * Request parameters for create.
 */
export interface CreateProjectRequest {
  /** name */
  name: string;
  /** description */
  description?: string;
}

/**
 * Request parameters for update.
 */
export interface UpdateProjectRequest {
  /** name */
  name: string;
  /** description */
  description?: string;
  /** invite|employee|team */
  admissions?: string;
  /** schedule attributes */
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
   * @param options - Optional parameters
   * @returns Array of Project
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
   * @param req - Request parameters
   * @returns The Project
   *
   * @example
   * ```ts
   * const result = await client.projects.create({ ... });
   * ```
   */
  async create(req: CreateProjectRequest): Promise<Project> {
    const response = await this.request(
      {
        service: "Projects",
        operation: "CreateProject",
        resourceType: "project",
        isMutation: true,
      },
      () =>
        this.client.POST("/projects.json", {
          body: req as any,
        })
    );
    return response;
  }

  /**
   * Get a single project by id
   * @param projectId - The project ID
   * @returns The Project
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
   * @param req - Request parameters
   * @returns The Project
   */
  async update(projectId: number, req: UpdateProjectRequest): Promise<Project> {
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
   * Trash a project (returns 204 No Content)
   * @param projectId - The project ID
   * @returns void
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