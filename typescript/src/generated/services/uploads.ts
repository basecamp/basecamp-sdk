/**
 * Uploads service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Upload entity from the Basecamp API. */
export type Upload = components["schemas"]["Upload"];

/**
 * Request parameters for update.
 */
export interface UpdateUploadRequest {
  /** description */
  description?: string;
  /** base name */
  baseName?: string;
}

/**
 * Request parameters for create.
 */
export interface CreateUploadRequest {
  /** attachable sgid */
  attachableSgid: string;
  /** description */
  description?: string;
  /** base name */
  baseName?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Uploads operations.
 */
export class UploadsService extends BaseService {

  /**
   * Get a single upload by id
   * @param projectId - The project ID
   * @param uploadId - The upload ID
   * @returns The Upload
   */
  async get(projectId: number, uploadId: number): Promise<Upload> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "GetUpload",
        resourceType: "upload",
        isMutation: false,
        projectId,
        resourceId: uploadId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/uploads/{uploadId}", {
          params: {
            path: { projectId, uploadId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing upload
   * @param projectId - The project ID
   * @param uploadId - The upload ID
   * @param req - Request parameters
   * @returns The Upload
   */
  async update(projectId: number, uploadId: number, req: UpdateUploadRequest): Promise<Upload> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "UpdateUpload",
        resourceType: "upload",
        isMutation: true,
        projectId,
        resourceId: uploadId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/uploads/{uploadId}", {
          params: {
            path: { projectId, uploadId },
          },
          body: {
            description: req.description,
            base_name: req.baseName,
          },
        })
    );
    return response;
  }

  /**
   * List versions of an upload
   * @param projectId - The project ID
   * @param uploadId - The upload ID
   * @returns Array of Upload
   */
  async listVersions(projectId: number, uploadId: number): Promise<Upload[]> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "ListUploadVersions",
        resourceType: "upload_version",
        isMutation: false,
        projectId,
        resourceId: uploadId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/uploads/{uploadId}/versions.json", {
          params: {
            path: { projectId, uploadId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * List uploads in a vault
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @returns Array of Upload
   */
  async list(projectId: number, vaultId: number): Promise<Upload[]> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "ListUploads",
        resourceType: "upload",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}/uploads.json", {
          params: {
            path: { projectId, vaultId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new upload in a vault
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @param req - Request parameters
   * @returns The Upload
   *
   * @example
   * ```ts
   * const result = await client.uploads.create(123, 123, { ... });
   * ```
   */
  async create(projectId: number, vaultId: number, req: CreateUploadRequest): Promise<Upload> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "CreateUpload",
        resourceType: "upload",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/vaults/{vaultId}/uploads.json", {
          params: {
            path: { projectId, vaultId },
          },
          body: {
            attachable_sgid: req.attachableSgid,
            description: req.description,
            base_name: req.baseName,
          },
        })
    );
    return response;
  }
}