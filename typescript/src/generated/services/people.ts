/**
 * People service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Person entity from the Basecamp API. */
export type Person = components["schemas"]["Person"];

/**
 * Request parameters for updateProjectAccess.
 */
export interface UpdateProjectAccessPeopleRequest {
  /** grant */
  grant?: number[];
  /** revoke */
  revoke?: number[];
  /** create */
  create?: components["schemas"]["CreatePersonRequest"][];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for People operations.
 */
export class PeopleService extends BaseService {

  /**
   * List all account users who can be pinged
   * @returns Array of Person
   */
  async listPingable(): Promise<Person[]> {
    const response = await this.request(
      {
        service: "People",
        operation: "ListPingablePeople",
        resourceType: "pingable_people",
        isMutation: false,
      },
      () =>
        this.client.GET("/circles/people.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Get the current authenticated user's profile
   * @returns The Person
   */
  async me(): Promise<Person> {
    const response = await this.request(
      {
        service: "People",
        operation: "GetMyProfile",
        resourceType: "my_profile",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/profile.json", {
        })
    );
    return response;
  }

  /**
   * List all people visible to the current user
   * @returns Array of Person
   */
  async list(): Promise<Person[]> {
    const response = await this.request(
      {
        service: "People",
        operation: "ListPeople",
        resourceType: "people",
        isMutation: false,
      },
      () =>
        this.client.GET("/people.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Get a person by ID
   * @param personId - The person ID
   * @returns The Person
   */
  async get(personId: number): Promise<Person> {
    const response = await this.request(
      {
        service: "People",
        operation: "GetPerson",
        resourceType: "person",
        isMutation: false,
        resourceId: personId,
      },
      () =>
        this.client.GET("/people/{personId}", {
          params: {
            path: { personId },
          },
        })
    );
    return response;
  }

  /**
   * List all active people on a project
   * @param projectId - The project ID
   * @returns Array of Person
   */
  async listForProject(projectId: number): Promise<Person[]> {
    const response = await this.request(
      {
        service: "People",
        operation: "ListProjectPeople",
        resourceType: "project_people",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/projects/{projectId}/people.json", {
          params: {
            path: { projectId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Update project access (grant/revoke/create people)
   * @param projectId - The project ID
   * @param req - Request parameters
   * @returns The project_access
   */
  async updateProjectAccess(projectId: number, req: UpdateProjectAccessPeopleRequest): Promise<components["schemas"]["UpdateProjectAccessResponseContent"]> {
    const response = await this.request(
      {
        service: "People",
        operation: "UpdateProjectAccess",
        resourceType: "project_access",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.PUT("/projects/{projectId}/people/users.json", {
          params: {
            path: { projectId },
          },
          body: req as any,
        })
    );
    return response;
  }

  /**
   * List people who can be assigned todos
   * @returns Array of Person
   */
  async listAssignable(): Promise<Person[]> {
    const response = await this.request(
      {
        service: "People",
        operation: "ListAssignablePeople",
        resourceType: "assignable_people",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/todos/assigned.json", {
        })
    );
    return response ?? [];
  }
}