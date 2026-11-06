/**
 * CloudFiles service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** CloudFile entity from the Basecamp API. */
export type CloudFile = components["schemas"]["CloudFile"];

/**
 * Request parameters for createCloudFile.
 */
export interface CreateCloudFileCloudFileRequest {
  /** Url */
  url: string;
  /** Short identifier for the external service — "dropbox", "google_doc",
"figma", "other", … Derived from the CloudFile::Service subclass name, so it
is always present. `other` accepts any well-formed HTTPS URL. */
  service: string;
  /** Title */
  title?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Subscriptions */
  subscriptions?: number[];
  /** Whether the cloud file is visible to the project's clients. Applies only
when creating directly in the tool's vault — an item created inside a
folder inherits the folder's visibility and ignores this. A client caller
always creates client-visible records regardless of what is sent. */
  visibleToClients?: boolean;
  /** Who to notify: "default" (the project's subscribers, without clients),
"everyone" (everyone on the project who can see it), or "custom" (the people
in subscriptions). "default" and "everyone" leave out the caller.

Omit both this and subscriptions to notify nobody. A subscriptions array sent
without notify is read as "custom". */
  notify?: string;
}

/**
 * Request parameters for updateCloudFile.
 */
export interface UpdateCloudFileCloudFileRequest {
  /** Url */
  url: string;
  /** Short identifier for the external service — "dropbox", "google_doc",
"figma", "other", … Derived from the CloudFile::Service subclass name, so it
is always present. `other` accepts any well-formed HTTPS URL. */
  service: string;
  /** Title */
  title?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Subscriptions */
  subscriptions?: number[];
  /** Who to notify: "default" (the project's subscribers, without clients),
"everyone" (everyone on the project who can see it), or "custom" (the people
in subscriptions). "default" and "everyone" leave out the caller.

Applies only to a drafted cloud file, whose subscribers it recomputes; a
published one ignores it. Omit both this and subscriptions to keep a draft's
current subscribers. A subscriptions array sent without notify is read as
"custom". */
  notify?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for CloudFiles operations.
 */
export class CloudFilesService extends BaseService {

  /**
   * Create a new cloud file in a vault.
   * @param bucketId - The bucket ID
   * @param vaultId - The vault ID
   * @param req - Cloud_file creation parameters
   * @returns The CloudFile
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.cloudFiles.createCloudFile(123, 123, { url: "example", service: "example" });
   * ```
   */
  async createCloudFile(bucketId: number, vaultId: number, req: CreateCloudFileCloudFileRequest): Promise<CloudFile> {
    if (!req.url) {
      throw Errors.validation("Url is required");
    }
    if (!req.service) {
      throw Errors.validation("Service is required");
    }
    const response = await this.request(
      {
        service: "CloudFiles",
        operation: "CreateCloudFile",
        resourceType: "cloud_file",
        isMutation: true,
        projectId: bucketId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{bucketId}/vaults/{vaultId}/cloud_files.json", {
          params: {
            path: { bucketId, vaultId },
          },
          body: {
            url: req.url,
            service: req.service,
            title: req.title,
            description: req.description,
            subscriptions: req.subscriptions,
            visible_to_clients: req.visibleToClients,
            notify: req.notify,
          },
        })
    );
    return response;
  }

  /**
   * Get a single cloud file by id
   * @param cloudFileId - The cloud file ID
   * @returns The CloudFile
   *
   * @example
   * ```ts
   * const result = await client.cloudFiles.cloudFile(123);
   * ```
   */
  async cloudFile(cloudFileId: number): Promise<CloudFile> {
    const response = await this.request(
      {
        service: "CloudFiles",
        operation: "GetCloudFile",
        resourceType: "cloud_file",
        isMutation: false,
        resourceId: cloudFileId,
      },
      () =>
        this.client.GET("/cloud_files/{cloudFileId}", {
          params: {
            path: { cloudFileId },
          },
        })
    );
    return response;
  }

  /**
   * Replace a cloud file with a new complete representation.
   * @param cloudFileId - The cloud file ID
   * @param req - Cloud_file update parameters
   * @returns The CloudFile
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.cloudFiles.updateCloudFile(123, { url: "example", service: "example" });
   * ```
   */
  async updateCloudFile(cloudFileId: number, req: UpdateCloudFileCloudFileRequest): Promise<CloudFile> {
    if (!req.url) {
      throw Errors.validation("Url is required");
    }
    if (!req.service) {
      throw Errors.validation("Service is required");
    }
    const response = await this.request(
      {
        service: "CloudFiles",
        operation: "UpdateCloudFile",
        resourceType: "cloud_file",
        isMutation: true,
        resourceId: cloudFileId,
      },
      () =>
        this.client.PUT("/cloud_files/{cloudFileId}", {
          params: {
            path: { cloudFileId },
          },
          body: {
            url: req.url,
            service: req.service,
            title: req.title,
            description: req.description,
            subscriptions: req.subscriptions,
            notify: req.notify,
          },
        })
    );
    return response;
  }
}