/**
 * People service for Basecamp SDK.
 *
 * Provides functionality to list and get people (users) in an account
 * and manage project access.
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

/**
 * A person (user) in Basecamp.
 * Uses the generated schema type.
 */
export type Person = components["schemas"]["Person"];

/**
 * A company associated with a person.
 */
export type PersonCompany = components["schemas"]["PersonCompany"];

/**
 * Request to create a new person.
 */
export interface CreatePersonRequest {
  /** Person's full name (required) */
  name: string;
  /** Person's email address (required) */
  emailAddress: string;
  /** Job title */
  title?: string;
  /** Company name */
  companyName?: string;
}

/**
 * Request to update project access.
 */
export interface UpdateProjectAccessRequest {
  /** Person IDs to grant access to */
  grant?: number[];
  /** Person IDs to revoke access from */
  revoke?: number[];
  /** New people to create and grant access to */
  create?: CreatePersonRequest[];
}

/**
 * Response from updating project access.
 */
export interface UpdateProjectAccessResponse {
  /** People who were granted access */
  granted: Person[];
  /** People whose access was revoked */
  revoked: Person[];
}

/**
 * Service for people-related operations.
 *
 * @example
 * ```ts
 * // List all people in the account
 * const people = await client.people.list();
 *
 * // Get the current user
 * const me = await client.people.me();
 *
 * // List people on a specific project
 * const projectPeople = await client.people.listProjectPeople(projectId);
 * ```
 */
export class PeopleService extends BaseService {
  /**
   * Lists all people visible to the current user in the account.
   *
   * @returns Array of people
   *
   * @example
   * ```ts
   * const people = await client.people.list();
   * for (const person of people) {
   *   console.log(`${person.name} (${person.email_address})`);
   * }
   * ```
   */
  async list(): Promise<Person[]> {
    const response = await this.request(
      {
        service: "People",
        operation: "List",
        resourceType: "person",
        isMutation: false,
      },
      () => this.client.GET("/people.json")
    );

    // Response may be wrapped in { people: [...] }
    const data = response as { people?: Person[] } | Person[];
    if (Array.isArray(data)) {
      return data;
    }
    return data.people ?? [];
  }

  /**
   * Gets a person by ID.
   *
   * @param personId - The person's ID
   * @returns The person
   *
   * @example
   * ```ts
   * const person = await client.people.get(12345);
   * console.log(person.name);
   * ```
   */
  async get(personId: number): Promise<Person> {
    const response = await this.request(
      {
        service: "People",
        operation: "Get",
        resourceType: "person",
        isMutation: false,
        resourceId: personId,
      },
      () =>
        this.client.GET("/people/{personId}", {
          params: { path: { personId } },
        })
    );

    // Response may be wrapped in { person: {...} }
    const data = response as { person?: Person } | Person;
    if ("person" in data && data.person) {
      return data.person;
    }
    return data as Person;
  }

  /**
   * Gets the current authenticated user's profile.
   *
   * @returns The current user's person record
   *
   * @example
   * ```ts
   * const me = await client.people.me();
   * console.log(`Logged in as ${me.name}`);
   * ```
   */
  async me(): Promise<Person> {
    const response = await this.request(
      {
        service: "People",
        operation: "Me",
        resourceType: "person",
        isMutation: false,
      },
      () => this.client.GET("/my/profile.json")
    );

    // Response may be wrapped in { person: {...} }
    const data = response as { person?: Person } | Person;
    if ("person" in data && data.person) {
      return data.person;
    }
    return data as Person;
  }

  /**
   * Lists all active people on a project.
   *
   * @param projectId - The project ID
   * @returns Array of people on the project
   *
   * @example
   * ```ts
   * const people = await client.people.listProjectPeople(projectId);
   * console.log(`${people.length} people on this project`);
   * ```
   */
  async listProjectPeople(projectId: number): Promise<Person[]> {
    const response = await this.request(
      {
        service: "People",
        operation: "ListProjectPeople",
        resourceType: "person",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/projects/{projectId}/people.json", {
          params: { path: { projectId } },
        })
    );

    // Response may be wrapped in { people: [...] }
    const data = response as { people?: Person[] } | Person[];
    if (Array.isArray(data)) {
      return data;
    }
    return data.people ?? [];
  }

  /**
   * Lists all account users who can be pinged.
   *
   * Pingable users are those who can receive direct messages
   * and be mentioned in posts.
   *
   * @returns Array of pingable people
   *
   * @example
   * ```ts
   * const pingable = await client.people.pingable();
   * // Use for autocomplete in mention features
   * ```
   */
  async pingable(): Promise<Person[]> {
    const response = await this.request(
      {
        service: "People",
        operation: "Pingable",
        resourceType: "person",
        isMutation: false,
      },
      () => this.client.GET("/circles/people.json")
    );

    // Response may be wrapped in { people: [...] }
    const data = response as { people?: Person[] } | Person[];
    if (Array.isArray(data)) {
      return data;
    }
    return data.people ?? [];
  }

  /**
   * Updates project access for people.
   *
   * Grants or revokes access for existing people, or creates new people
   * and grants them access.
   *
   * @param projectId - The project ID
   * @param request - The access update request
   * @returns The granted and revoked people
   *
   * @example
   * ```ts
   * // Grant access to existing users
   * const result = await client.people.updateProjectAccess(projectId, {
   *   grant: [userId1, userId2],
   * });
   *
   * // Create a new user and grant access
   * const result = await client.people.updateProjectAccess(projectId, {
   *   create: [{ name: "New User", emailAddress: "new@example.com" }],
   * });
   * ```
   */
  async updateProjectAccess(
    projectId: number,
    request: UpdateProjectAccessRequest
  ): Promise<UpdateProjectAccessResponse> {
    if (!request.grant?.length && !request.revoke?.length && !request.create?.length) {
      throw Errors.validation("At least one of grant, revoke, or create must be specified");
    }

    const response = await this.request(
      {
        service: "People",
        operation: "UpdateProjectAccess",
        resourceType: "person",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.PUT("/projects/{projectId}/people/users.json", {
          params: { path: { projectId } },
          body: {
            grant: request.grant,
            revoke: request.revoke,
            create: request.create?.map((p) => ({
              name: p.name,
              email_address: p.emailAddress,
              title: p.title,
              company_name: p.companyName,
            })),
          },
        })
    );

    // Response has { result: { granted: [...], revoked: [...] } }
    const data = response as { result?: { granted?: Person[]; revoked?: Person[] } };
    return {
      granted: data.result?.granted ?? [],
      revoked: data.result?.revoked ?? [],
    };
  }
}
