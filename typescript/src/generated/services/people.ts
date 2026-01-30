/**
 * Service for People operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for People operations
 */
export class PeopleService extends BaseService {

  /**
   * List all account users who can be pinged
   */
  async listPingable(): Promise<components["schemas"]["ListPingablePeopleResponseContent"]> {
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
   */
  async myProfile(): Promise<components["schemas"]["GetMyProfileResponseContent"]> {
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
   */
  async list(): Promise<components["schemas"]["ListPeopleResponseContent"]> {
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
   */
  async get(personId: number): Promise<components["schemas"]["GetPersonResponseContent"]> {
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
   */
  async listForProject(projectId: number): Promise<components["schemas"]["ListProjectPeopleResponseContent"]> {
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
   */
  async updateProjectAccess(projectId: number, req: components["schemas"]["UpdateProjectAccessRequestContent"]): Promise<components["schemas"]["UpdateProjectAccessResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List people who can be assigned todos
   */
  async listAssignable(): Promise<components["schemas"]["ListAssignablePeopleResponseContent"]> {
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