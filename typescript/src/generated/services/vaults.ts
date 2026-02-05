/**
 * Vaults service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** Vault entity from the Basecamp API. */
export type Vault = components["schemas"]["Vault"];

/**
 * Request parameters for update.
 */
export interface UpdateVaultRequest {
  /** Title */
  title?: string;
}

/**
 * Request parameters for create.
 */
export interface CreateVaultRequest {
  /** Title */
  title: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Vaults operations.
 */
export class VaultsService extends BaseService {

  /**
   * Get a single vault by id
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @returns The Vault
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.vaults.get(123, 123);
   * ```
   */
  async get(projectId: number, vaultId: number): Promise<Vault> {
    const response = await this.request(
      {
        service: "Vaults",
        operation: "GetVault",
        resourceType: "vault",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}", {
          params: {
            path: { projectId, vaultId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing vault
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @param req - Vault update parameters
   * @returns The Vault
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.vaults.update(123, 123, { });
   * ```
   */
  async update(projectId: number, vaultId: number, req: UpdateVaultRequest): Promise<Vault> {
    const response = await this.request(
      {
        service: "Vaults",
        operation: "UpdateVault",
        resourceType: "vault",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/vaults/{vaultId}", {
          params: {
            path: { projectId, vaultId },
          },
          body: {
            title: req.title,
          },
        })
    );
    return response;
  }

  /**
   * List vaults (subfolders) in a vault
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @returns Array of Vault
   *
   * @example
   * ```ts
   * const result = await client.vaults.list(123, 123);
   * ```
   */
  async list(projectId: number, vaultId: number): Promise<Vault[]> {
    const response = await this.request(
      {
        service: "Vaults",
        operation: "ListVaults",
        resourceType: "vault",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}/vaults.json", {
          params: {
            path: { projectId, vaultId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new vault (subfolder) in a vault
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @param req - Vault creation parameters
   * @returns The Vault
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.vaults.create(123, 123, { title: "example" });
   * ```
   */
  async create(projectId: number, vaultId: number, req: CreateVaultRequest): Promise<Vault> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    const response = await this.request(
      {
        service: "Vaults",
        operation: "CreateVault",
        resourceType: "vault",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/vaults/{vaultId}/vaults.json", {
          params: {
            path: { projectId, vaultId },
          },
          body: {
            title: req.title,
          },
        })
    );
    return response;
  }
}