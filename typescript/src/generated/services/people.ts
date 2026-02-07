/**
 * People service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";

// =============================================================================
// Types
// =============================================================================

/** Person entity from the Basecamp API. */
export type Person = components["schemas"]["Person"];

/**
 * Options for listPingable.
 */
export interface ListPingablePeopleOptions extends PaginationOptions {
}

/**
 * Options for list.
 */
export interface ListPeopleOptions extends PaginationOptions {
}

/**
 * Options for listForProject.
 */
export interface ListForProjectPeopleOptions extends PaginationOptions {
}

/**
 * Request parameters for updateProjectAccess.
 */
export interface UpdateProjectAccessPeopleRequest {
  /** Grant */
  grant?: number[];
  /** Revoke */
  revoke?: number[];
  /** Create */
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
   * @param options - Optional query parameters
   * @returns All Person across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.people.listPingable();
   * ```
   */
  async listPingable(options?: ListPingablePeopleOptions): Promise<ListResult<Person>> {
    return this.requestPaginated(
      {
        service: "People",
        operation: "ListPingablePeople",
        resourceType: "pingable_people",
        isMutation: false,
      },
      () =>
        this.client.GET("/circles/people.json", {
        })
      , options
    );
  }

  /**
   * Get the current authenticated user's profile
   * @returns The Person
   *
   * @example
   * ```ts
   * const result = await client.people.me();
   * ```
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
   * @param options - Optional query parameters
   * @returns All Person across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.people.list();
   * ```
   */
  async list(options?: ListPeopleOptions): Promise<ListResult<Person>> {
    return this.requestPaginated(
      {
        service: "People",
        operation: "ListPeople",
        resourceType: "people",
        isMutation: false,
      },
      () =>
        this.client.GET("/people.json", {
        })
      , options
    );
  }

  /**
   * Get a person by ID
   * @param personId - The person ID
   * @returns The Person
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.people.get(123);
   * ```
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
   * @param options - Optional query parameters
   * @returns All Person across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.people.listForProject(123);
   * ```
   */
  async listForProject(projectId: number, options?: ListForProjectPeopleOptions): Promise<ListResult<Person>> {
    return this.requestPaginated(
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
      , options
    );
  }

  /**
   * Update project access (grant/revoke/create people)
   * @param projectId - The project ID
   * @param req - Project_access update parameters
   * @returns The project_access
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.people.updateProjectAccess(123, { });
   * ```
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
          body: {
            grant: req.grant,
            revoke: req.revoke,
            create: req.create,
          },
        })
    );
    return response;
  }

  /**
   * List people who can be assigned todos
   * @returns Array of Person
   *
   * @example
   * ```ts
   * const result = await client.people.listAssignable();
   * ```
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