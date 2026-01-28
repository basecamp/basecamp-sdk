/**
 * Vaults service for the Basecamp API.
 *
 * Vaults are folders in the Files & Documents tool. They can contain
 * documents, uploads (files), and nested vaults (subfolders).
 *
 * @example
 * ```ts
 * // Get a vault
 * const vault = await client.vaults.get(projectId, vaultId);
 *
 * // List child vaults (subfolders)
 * const subfolders = await client.vaults.list(projectId, vaultId);
 *
 * // Create a new subfolder
 * const newVault = await client.vaults.create(projectId, parentVaultId, {
 *   title: "2024 Reports",
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
 * A Basecamp vault (folder).
 */
export type Vault = components["schemas"]["Vault"];

/**
 * A person associated with the vault (creator).
 */
export type Person = components["schemas"]["Person"];

/**
 * Request to create a new vault (folder).
 */
export interface CreateVaultRequest {
  /** Vault title/name (required) */
  title: string;
}

/**
 * Request to update an existing vault.
 */
export interface UpdateVaultRequest {
  /** Vault title/name (optional) */
  title?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp vaults (folders).
 */
export class VaultsService extends BaseService {
  /**
   * Gets a vault by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The vault ID
   * @returns The vault
   * @throws BasecampError with code "not_found" if vault doesn't exist
   *
   * @example
   * ```ts
   * const vault = await client.vaults.get(projectId, vaultId);
   * console.log(vault.title, vault.documents_count, vault.uploads_count);
   * ```
   */
  async get(projectId: number, vaultId: number): Promise<Vault> {
    const response = await this.request(
      {
        service: "Vaults",
        operation: "Get",
        resourceType: "vault",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}", {
          params: { path: { projectId, vaultId } },
        })
    );

    return response.vault!;
  }

  /**
   * Lists all child vaults (subfolders) in a vault.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The parent vault ID
   * @returns Array of child vaults
   *
   * @example
   * ```ts
   * const subfolders = await client.vaults.list(projectId, vaultId);
   * for (const folder of subfolders) {
   *   console.log(folder.title, folder.position);
   * }
   * ```
   */
  async list(projectId: number, vaultId: number): Promise<Vault[]> {
    const response = await this.request(
      {
        service: "Vaults",
        operation: "List",
        resourceType: "vault",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}/vaults.json", {
          params: { path: { projectId, vaultId } },
        })
    );

    return response?.vaults ?? [];
  }

  /**
   * Creates a new child vault (subfolder) in a vault.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The parent vault ID
   * @param req - Vault creation parameters
   * @returns The created vault
   * @throws BasecampError with code "validation" if title is missing
   *
   * @example
   * ```ts
   * const newFolder = await client.vaults.create(projectId, parentVaultId, {
   *   title: "Q4 Reports",
   * });
   * ```
   */
  async create(projectId: number, vaultId: number, req: CreateVaultRequest): Promise<Vault> {
    if (!req.title) {
      throw Errors.validation("Vault title is required");
    }

    const response = await this.request(
      {
        service: "Vaults",
        operation: "Create",
        resourceType: "vault",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/vaults/{vaultId}/vaults.json", {
          params: { path: { projectId, vaultId } },
          body: {
            title: req.title,
          },
        })
    );

    return response.vault!;
  }

  /**
   * Updates an existing vault.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The vault ID
   * @param req - Vault update parameters
   * @returns The updated vault
   *
   * @example
   * ```ts
   * const updated = await client.vaults.update(projectId, vaultId, {
   *   title: "Renamed Folder",
   * });
   * ```
   */
  async update(projectId: number, vaultId: number, req: UpdateVaultRequest): Promise<Vault> {
    const response = await this.request(
      {
        service: "Vaults",
        operation: "Update",
        resourceType: "vault",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/vaults/{vaultId}", {
          params: { path: { projectId, vaultId } },
          body: {
            title: req.title,
          },
        })
    );

    return response.vault!;
  }
}
