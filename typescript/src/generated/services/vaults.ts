/**
 * Service for Vaults operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Vaults operations
 */
export class VaultsService extends BaseService {

  /**
   * Get a single vault by id
   */
  async get(projectId: number, vaultId: number): Promise<components["schemas"]["GetVaultResponseContent"]> {
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
   */
  async update(projectId: number, vaultId: number, req: components["schemas"]["UpdateVaultRequestContent"]): Promise<components["schemas"]["UpdateVaultResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List vaults (subfolders) in a vault
   */
  async list(projectId: number, vaultId: number): Promise<components["schemas"]["ListVaultsResponseContent"]> {
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
   */
  async create(projectId: number, vaultId: number, req: components["schemas"]["CreateVaultRequestContent"]): Promise<components["schemas"]["CreateVaultResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }
}