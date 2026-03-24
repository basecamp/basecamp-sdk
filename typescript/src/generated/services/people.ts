/**
 * People service for the Basecamp API.
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

/** Person entity from the Basecamp API. */
export type Person = components["schemas"]["Person"];

/**
 * Options for listPingable.
 */
export interface ListPingablePeopleOptions extends PaginationOptions {
}

/**
 * Request parameters for updateMyPreferences.
 */
export interface UpdateMyPreferencesPeopleRequest {
  /** Person */
  person: components["schemas"]["PreferencesPayload"];
}

/**
 * Request parameters for updateMyProfile.
 */
export interface UpdateMyProfilePeopleRequest {
  /** Display name */
  name?: string;
  /** Email address */
  emailAddress?: string;
  /** Title */
  title?: string;
  /** Bio */
  bio?: string;
  /** Location */
  location?: string;
  /** Time zone name */
  timeZoneName?: string;
  /** First week day */
  firstWeekDay?: components["schemas"]["FirstWeekDay"];
  /** Time format */
  timeFormat?: string;
}

/**
 * Options for list.
 */
export interface ListPeopleOptions extends PaginationOptions {
}

/**
 * Request parameters for enableOutOfOffice.
 */
export interface EnableOutOfOfficePeopleRequest {
  /** Out of office */
  outOfOffice: components["schemas"]["OutOfOfficePayload"];
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
   * Get the current user's preferences
   * @returns The my_preference
   *
   * @example
   * ```ts
   * const result = await client.people.myPreferences();
   * ```
   */
  async myPreferences(): Promise<components["schemas"]["GetMyPreferencesResponseContent"]> {
    const response = await this.request(
      {
        service: "People",
        operation: "GetMyPreferences",
        resourceType: "my_preference",
        isMutation: false,
      },
      () =>
        this.client.GET("/my/preferences.json", {
        })
    );
    return response;
  }

  /**
   * Update the current user's preferences
   * @param req - My_preference update parameters
   * @returns The my_preference
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.people.updateMyPreferences({ person: "example" });
   * ```
   */
  async updateMyPreferences(req: UpdateMyPreferencesPeopleRequest): Promise<components["schemas"]["UpdateMyPreferencesResponseContent"]> {
    if (!req.person) {
      throw Errors.validation("Person is required");
    }
    const response = await this.request(
      {
        service: "People",
        operation: "UpdateMyPreferences",
        resourceType: "my_preference",
        isMutation: true,
      },
      () =>
        this.client.PUT("/my/preferences.json", {
          body: {
            person: req.person,
          },
        })
    );
    return response;
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
   * Update the current authenticated user's profile
   * @param req - My_profile update parameters
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.people.updateMyProfile({ });
   * ```
   */
  async updateMyProfile(req: UpdateMyProfilePeopleRequest): Promise<void> {
    await this.request(
      {
        service: "People",
        operation: "UpdateMyProfile",
        resourceType: "my_profile",
        isMutation: true,
      },
      () =>
        this.client.PUT("/my/profile.json", {
          body: {
            name: req.name,
            email_address: req.emailAddress,
            title: req.title,
            bio: req.bio,
            location: req.location,
            time_zone_name: req.timeZoneName,
            first_week_day: req.firstWeekDay,
            time_format: req.timeFormat,
          },
        })
    );
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
   * Get the out of office status for a person
   * @param personId - The person ID
   * @returns The out_of_office
   *
   * @example
   * ```ts
   * const result = await client.people.outOfOffice(123);
   * ```
   */
  async outOfOffice(personId: number): Promise<components["schemas"]["GetOutOfOfficeResponseContent"]> {
    const response = await this.request(
      {
        service: "People",
        operation: "GetOutOfOffice",
        resourceType: "out_of_office",
        isMutation: false,
        resourceId: personId,
      },
      () =>
        this.client.GET("/people/{personId}/out_of_office.json", {
          params: {
            path: { personId },
          },
        })
    );
    return response;
  }

  /**
   * Enable or replace out of office for a person.
   * @param personId - The person ID
   * @param req - Out_of_office request parameters
   * @returns The out_of_office
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.people.enableOutOfOffice(123, { outOfOffice: "example" });
   * ```
   */
  async enableOutOfOffice(personId: number, req: EnableOutOfOfficePeopleRequest): Promise<components["schemas"]["EnableOutOfOfficeResponseContent"]> {
    if (!req.outOfOffice) {
      throw Errors.validation("Out of office is required");
    }
    const response = await this.request(
      {
        service: "People",
        operation: "EnableOutOfOffice",
        resourceType: "out_of_office",
        isMutation: true,
        resourceId: personId,
      },
      () =>
        this.client.POST("/people/{personId}/out_of_office.json", {
          params: {
            path: { personId },
          },
          body: {
            out_of_office: req.outOfOffice,
          },
        })
    );
    return response;
  }

  /**
   * Disable out of office for a person.
   * @param personId - The person ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.people.disableOutOfOffice(123);
   * ```
   */
  async disableOutOfOffice(personId: number): Promise<void> {
    await this.request(
      {
        service: "People",
        operation: "DisableOutOfOffice",
        resourceType: "out_of_office",
        isMutation: true,
        resourceId: personId,
      },
      () =>
        this.client.DELETE("/people/{personId}/out_of_office.json", {
          params: {
            path: { personId },
          },
        })
    );
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