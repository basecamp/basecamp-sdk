/**
 * Service for Projects operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Projects operations
 */
export class ProjectsService extends BaseService {

  /**
   * List projects (active by default; optionally archived/trashed)
   */
  async list(options?: { status?: string }): Promise<components["schemas"]["ListProjectsResponseContent"]> {
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
   */
  async create(req: components["schemas"]["CreateProjectRequestContent"]): Promise<components["schemas"]["CreateProjectResponseContent"]> {
    const response = await this.request(
      {
        service: "Projects",
        operation: "CreateProject",
        resourceType: "project",
        isMutation: true,
      },
      () =>
        this.client.POST("/projects.json", {
          body: req,
        })
    );
    return response;
  }

  /**
   * Get a single project by id
   */
  async get(projectId: number): Promise<components["schemas"]["GetProjectResponseContent"]> {
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
   */
  async update(projectId: number, req: components["schemas"]["UpdateProjectRequestContent"]): Promise<components["schemas"]["UpdateProjectResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * Trash a project (returns 204 No Content)
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