/**
 * Uploads service for the Basecamp API.
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

/** Upload entity from the Basecamp API. */
export type Upload = components["schemas"]["Upload"];
/** UploadVersion entity from the Basecamp API. */
export type UploadVersion = components["schemas"]["UploadVersion"];

/**
 * Request parameters for update.
 */
export interface UpdateUploadRequest {
  /** Rich text description (HTML) */
  description?: string;
  /** Base name */
  baseName?: string;
}

/**
 * Options for listVersions.
 */
export interface ListVersionsUploadOptions extends PaginationOptions {
}

/**
 * Request parameters for createVersion.
 */
export interface CreateVersionUploadRequest {
  /** Attachable sgid */
  attachableSgid: string;
  /** Omit to keep the uploaded file's own name. Sending "" also keeps it. */
  baseName?: string;
  /** Presence-aware: omit to carry the previous version's description forward,
send "" to clear it, send a value to set it. */
  description?: string;
  /** Who to notify: "default", "everyone", or "custom" (the people in subscriptions).

Omit both this and subscriptions to notify nobody. A subscriptions array sent
without notify is read as "custom". */
  notify?: string;
  /** People to notify about the replacement and subscribe to the upload. */
  subscriptions?: number[];
}

/**
 * Options for list.
 */
export interface ListUploadOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for create.
 */
export interface CreateUploadRequest {
  /** Attachable sgid */
  attachableSgid: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Base name */
  baseName?: string;
  /** Subscriptions */
  subscriptions?: number[];
  /** Visible to clients */
  visibleToClients?: boolean;
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
   * @param uploadId - The upload ID
   * @returns The Upload
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.uploads.get(123);
   * ```
   */
  async get(uploadId: number): Promise<Upload> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "GetUpload",
        resourceType: "upload",
        isMutation: false,
        resourceId: uploadId,
      },
      () =>
        this.client.GET("/uploads/{uploadId}", {
          params: {
            path: { uploadId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing upload
   * @param uploadId - The upload ID
   * @param req - Upload update parameters
   * @returns The Upload
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.uploads.update(123, { });
   * ```
   */
  async update(uploadId: number, req: UpdateUploadRequest): Promise<Upload> {
    const response = await this.request(
      {
        service: "Uploads",
        operation: "UpdateUpload",
        resourceType: "upload",
        isMutation: true,
        resourceId: uploadId,
      },
      () =>
        this.client.PUT("/uploads/{uploadId}", {
          params: {
            path: { uploadId },
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
   * @param uploadId - The upload ID
   * @param options - Optional query parameters
   * @returns All UploadVersion across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.uploads.listVersions(123);
   * ```
   */
  async listVersions(uploadId: number, options?: ListVersionsUploadOptions): Promise<ListResult<UploadVersion>> {
    return this.requestPaginated(
      {
        service: "Uploads",
        operation: "ListUploadVersions",
        resourceType: "upload_version",
        isMutation: false,
        resourceId: uploadId,
      },
      () =>
        this.client.GET("/uploads/{uploadId}/versions.json", {
          params: {
            path: { uploadId },
          },
        })
      , options
    );
  }

  /**
   * Replace an upload's file with a new version
   * @param uploadId - The upload ID
   * @param req - Upload_version creation parameters
   * @returns The Upload
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.uploads.createVersion(123, { attachableSgid: "example" });
   * ```
   */
  async createVersion(uploadId: number, req: CreateVersionUploadRequest): Promise<Upload> {
    if (!req.attachableSgid) {
      throw Errors.validation("Attachable sgid is required");
    }
    const response = await this.request(
      {
        service: "Uploads",
        operation: "CreateUploadVersion",
        resourceType: "upload_version",
        isMutation: true,
        resourceId: uploadId,
      },
      () =>
        this.client.POST("/uploads/{uploadId}/versions.json", {
          params: {
            path: { uploadId },
          },
          body: {
            attachable_sgid: req.attachableSgid,
            base_name: req.baseName,
            description: req.description,
            notify: req.notify,
            subscriptions: req.subscriptions,
          },
        })
    );
    return response;
  }

  /**
   * List uploads in a vault
   * @param vaultId - The vault ID
   * @param options - Optional query parameters
   * @returns All Upload across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.uploads.list(123);
   *
   * // With options
   * const filtered = await client.uploads.list(123, { page: 1 });
   * ```
   */
  async list(vaultId: number, options?: ListUploadOptions): Promise<ListResult<Upload>> {
    return this.requestPaginated(
      {
        service: "Uploads",
        operation: "ListUploads",
        resourceType: "upload",
        isMutation: false,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/vaults/{vaultId}/uploads.json", {
          params: {
            path: { vaultId },
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Create a new upload in a vault
   * @param vaultId - The vault ID
   * @param req - Upload creation parameters
   * @returns The Upload
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.uploads.create(123, { attachableSgid: "example" });
   * ```
   */
  async create(vaultId: number, req: CreateUploadRequest): Promise<Upload> {
    if (!req.attachableSgid) {
      throw Errors.validation("Attachable sgid is required");
    }
    const response = await this.request(
      {
        service: "Uploads",
        operation: "CreateUpload",
        resourceType: "upload",
        isMutation: true,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/vaults/{vaultId}/uploads.json", {
          params: {
            path: { vaultId },
          },
          body: {
            attachable_sgid: req.attachableSgid,
            description: req.description,
            base_name: req.baseName,
            subscriptions: req.subscriptions,
            visible_to_clients: req.visibleToClients,
          },
        })
    );
    return response;
  }
}