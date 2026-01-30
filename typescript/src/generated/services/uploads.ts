/**
 * Service for Uploads operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Uploads operations
 */
export class UploadsService extends BaseService {

  /**
   * Get a single upload by id
   */
  async get(projectId: number, uploadId: number): Promise<components["schemas"]["GetUploadResponseContent"]> {
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
   */
  async update(projectId: number, uploadId: number, req: components["schemas"]["UpdateUploadRequestContent"]): Promise<components["schemas"]["UpdateUploadResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List versions of an upload
   */
  async listVersions(projectId: number, uploadId: number): Promise<components["schemas"]["ListUploadVersionsResponseContent"]> {
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
   */
  async list(projectId: number, vaultId: number): Promise<components["schemas"]["ListUploadsResponseContent"]> {
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
   */
  async create(projectId: number, vaultId: number, req: components["schemas"]["CreateUploadRequestContent"]): Promise<components["schemas"]["CreateUploadResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }
}