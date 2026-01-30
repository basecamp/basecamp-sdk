/**
 * Uploads service for the Basecamp API.
 *
 * Uploads are files stored within vaults. They are created from
 * attachments (via attachable_sgid) and can have descriptions
 * and version history.
 *
 * @example
 * ```ts
 * // Get an upload
 * const upload = await client.uploads.get(projectId, uploadId);
 *
 * // List uploads in a vault
 * const uploads = await client.uploads.list(projectId, vaultId);
 *
 * // Create a new upload (requires uploading attachment first)
 * const attachment = await client.attachments.create({
 *   filename: "report.pdf",
 *   contentType: "application/pdf",
 *   data: fileBuffer,
 * });
 * const upload = await client.uploads.create(projectId, vaultId, {
 *   attachableSgid: attachment.attachableSgid,
 *   description: "Q4 financial report",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp upload (file).
 */
export type Upload = components["schemas"]["Upload"];

/**
 * A person associated with the upload (creator).
 */
export type Person = components["schemas"]["Person"];

/**
 * Request to create a new upload.
 */
export interface CreateUploadRequest {
  /** Signed global ID from attachment upload (required) */
  attachableSgid: string;
  /** Upload description in HTML (optional) */
  description?: string;
  /** Filename without extension (optional) */
  baseName?: string;
}

/**
 * Request to update an existing upload.
 */
export interface UpdateUploadRequest {
  /** Upload description in HTML (optional) */
  description?: string;
  /** Filename without extension (optional) */
  baseName?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp uploads (files).
 */
export class UploadsService extends BaseService {
  /**
   * Gets an upload by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param uploadId - The upload ID
   * @returns The upload
   * @throws BasecampError with code "not_found" if upload doesn't exist
   *
   * @example
   * ```ts
   * const upload = await client.uploads.get(projectId, uploadId);
   * console.log(upload.filename, upload.byte_size, upload.download_url);
   * ```
   */
  async get(projectId: number, uploadId: number): Promise<Upload> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "Get",
        resourceType: "upload",
        isMutation: false,
        projectId,
        resourceId: uploadId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/uploads/{uploadId}", {
          params: { path: { projectId, uploadId } },
        })
    );

    return response;
  }

  /**
   * Lists all uploads in a vault.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The vault ID
   * @returns Array of uploads
   *
   * @example
   * ```ts
   * const uploads = await client.uploads.list(projectId, vaultId);
   * for (const upload of uploads) {
   *   console.log(upload.filename, upload.content_type, upload.byte_size);
   * }
   * ```
   */
  async list(projectId: number, vaultId: number): Promise<Upload[]> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "List",
        resourceType: "upload",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}/uploads.json", {
          params: { path: { projectId, vaultId } },
        })
    );

    return response ?? [];
  }

  /**
   * Creates a new upload in a vault.
   * The attachable_sgid must be obtained from the Attachments.create endpoint.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The vault ID
   * @param req - Upload creation parameters
   * @returns The created upload
   * @throws BasecampError with code "validation" if attachableSgid is missing
   *
   * @example
   * ```ts
   * // First upload the file as an attachment
   * const attachment = await client.attachments.create({
   *   filename: "presentation.pptx",
   *   contentType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
   *   data: fileBuffer,
   * });
   *
   * // Then create the upload in a vault
   * const upload = await client.uploads.create(projectId, vaultId, {
   *   attachableSgid: attachment.attachableSgid,
   *   description: "Q4 Strategy Presentation",
   * });
   * ```
   */
  async create(projectId: number, vaultId: number, req: CreateUploadRequest): Promise<Upload> {
    if (!req.attachableSgid) {
      throw Errors.validation("Upload attachable_sgid is required");
    }

    const response = await this.request(
      {
        service: "Uploads",
        operation: "Create",
        resourceType: "upload",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/vaults/{vaultId}/uploads.json", {
          params: { path: { projectId, vaultId } },
          body: {
            attachable_sgid: req.attachableSgid,
            description: req.description,
            base_name: req.baseName,
          },
        })
    );

    return response;
  }

  /**
   * Updates an existing upload.
   *
   * @param projectId - The project (bucket) ID
   * @param uploadId - The upload ID
   * @param req - Upload update parameters
   * @returns The updated upload
   *
   * @example
   * ```ts
   * const updated = await client.uploads.update(projectId, uploadId, {
   *   description: "Updated description",
   *   baseName: "new-filename",
   * });
   * ```
   */
  async update(projectId: number, uploadId: number, req: UpdateUploadRequest): Promise<Upload> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "Update",
        resourceType: "upload",
        isMutation: true,
        projectId,
        resourceId: uploadId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/uploads/{uploadId}", {
          params: { path: { projectId, uploadId } },
          body: {
            description: req.description,
            base_name: req.baseName,
          },
        })
    );

    return response;
  }

  /**
   * Lists all versions of an upload.
   *
   * @param projectId - The project (bucket) ID
   * @param uploadId - The upload ID
   * @returns Array of upload versions
   *
   * @example
   * ```ts
   * const versions = await client.uploads.listVersions(projectId, uploadId);
   * for (const version of versions) {
   *   console.log(version.created_at, version.filename, version.byte_size);
   * }
   * ```
   */
  async listVersions(projectId: number, uploadId: number): Promise<Upload[]> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "ListVersions",
        resourceType: "upload",
        isMutation: false,
        projectId,
        resourceId: uploadId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/uploads/{uploadId}/versions.json", {
          params: { path: { projectId, uploadId } },
        })
    );

    return response ?? [];
  }

  /**
   * Moves an upload to the trash.
   * Trashed uploads can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param uploadId - The upload ID
   *
   * @example
   * ```ts
   * await client.uploads.trash(projectId, uploadId);
   * ```
   */
  async trash(projectId: number, uploadId: number): Promise<void> {
    await this.request(
      {
        service: "Uploads",
        operation: "Trash",
        resourceType: "upload",
        isMutation: true,
        projectId,
        resourceId: uploadId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: uploadId } },
        })
    );
  }
}
