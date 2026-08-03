/**
 * Folders service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Folder entity from the Basecamp API. */
export type Folder = components["schemas"]["Folder"];
/** FolderWithProjects entity from the Basecamp API. */
export type FolderWithProjects = components["schemas"]["FolderWithProjects"];

/**
 * Request parameters for createFolder.
 */
export interface CreateFolderFolderRequest {
  /** The folder's name. Defaults to `New folder` when blank, null, or omitted. */
  name?: string;
  /** IDs of the projects to file into the folder — the same ids the folder
reports back as `bucket_ids` and expands as `projects`. This does not
round-trip under its own name. Omit it, or send null or an empty array,
for an empty folder. */
  projectIds?: number[];
}

/**
 * Request parameters for updateFolder.
 */
export interface UpdateFolderFolderRequest {
  /** The folder's new name. Blank is rejected with 422 — unlike create, update
does not fall back to a default name. */
  name: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Folders operations.
 */
export class FoldersService extends BaseService {

  /**
   * List the authenticated user's folders in home-screen order.
   * @returns Array of Folder
   *
   * @example
   * ```ts
   * const result = await client.folders.listFolders();
   * ```
   */
  async listFolders(): Promise<Folder[]> {
    const response = await this.request(
      {
        service: "Folders",
        operation: "ListFolders",
        resourceType: "folder",
        isMutation: false,
      },
      () =>
        this.client.GET("/stacks.json", {
        })
    );
    return response ?? [];
  }

  /**
   * Create a folder for the authenticated user and file the given projects into it.
   * @param req - Folder creation parameters
   * @returns The FolderWithProjects
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.folders.createFolder({ });
   * ```
   */
  async createFolder(req: CreateFolderFolderRequest): Promise<FolderWithProjects> {
    const response = await this.request(
      {
        service: "Folders",
        operation: "CreateFolder",
        resourceType: "folder",
        isMutation: true,
      },
      () =>
        this.client.POST("/stacks.json", {
          body: {
            name: req.name,
            project_ids: req.projectIds,
          },
        })
    );
    return response;
  }

  /**
   * Get one folder, with the projects grouped inside it expanded under `projects`.
   * @param folderId - The folder ID
   * @returns The FolderWithProjects
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.folders.getFolder(123);
   * ```
   */
  async getFolder(folderId: number): Promise<FolderWithProjects> {
    const response = await this.request(
      {
        service: "Folders",
        operation: "GetFolder",
        resourceType: "folder",
        isMutation: false,
        resourceId: folderId,
      },
      () =>
        this.client.GET("/stacks/{folderId}", {
          params: {
            path: { folderId },
          },
        })
    );
    return response;
  }

  /**
   * Rename a folder.
   * @param folderId - The folder ID
   * @param req - Folder update parameters
   * @returns The FolderWithProjects
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.folders.updateFolder(123, { name: "My example" });
   * ```
   */
  async updateFolder(folderId: number, req: UpdateFolderFolderRequest): Promise<FolderWithProjects> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Folders",
        operation: "UpdateFolder",
        resourceType: "folder",
        isMutation: true,
        resourceId: folderId,
      },
      () =>
        this.client.PUT("/stacks/{folderId}", {
          params: {
            path: { folderId },
          },
          body: {
            name: req.name,
          },
        })
    );
    return response;
  }

  /**
   * Delete a folder and unpin its projects from the home screen.
   * @param folderId - The folder ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.folders.deleteFolder(123);
   * ```
   */
  async deleteFolder(folderId: number): Promise<void> {
    await this.request(
      {
        service: "Folders",
        operation: "DeleteFolder",
        resourceType: "folder",
        isMutation: true,
        resourceId: folderId,
      },
      () =>
        this.client.DELETE("/stacks/{folderId}", {
          params: {
            path: { folderId },
          },
        })
    );
  }
}