/**
 * Account service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for updateAccountName.
 */
export interface UpdateAccountNameAccountRequest {
  /** Display name */
  name: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Account operations.
 */
export class AccountService extends BaseService {

  /**
   * Get the account for the current access token
   * @returns The account
   *
   * @example
   * ```ts
   * const result = await client.account.account();
   * ```
   */
  async account(): Promise<components["schemas"]["GetAccountResponseContent"]> {
    const response = await this.request(
      {
        service: "Account",
        operation: "GetAccount",
        resourceType: "account",
        isMutation: false,
      },
      () =>
        this.client.GET("/account.json", {
        })
    );
    return response;
  }

  /**
   * Upload or replace the account logo.
   * @param file - File or Blob to upload
   * @param filename - Display name for the uploaded file
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.account.updateAccountLogo();
   * ```
   */
  async updateAccountLogo(file: Blob | File, filename?: string): Promise<void> {
    const url = `${this.baseUrl}` +
      `/account/logo.json`;
    return this.requestMultipartUpload(
      {
        service: "Account",
        operation: "UpdateAccountLogo",
        resourceType: "account_logo",
        isMutation: true,
      },
      url,
      "PUT",
      file,
      "logo",
      filename,
    );
  }


  /**
   * Remove the account logo. Only administrators and account owners can use this endpoint.
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.account.removeAccountLogo();
   * ```
   */
  async removeAccountLogo(): Promise<void> {
    await this.request(
      {
        service: "Account",
        operation: "RemoveAccountLogo",
        resourceType: "resource",
        isMutation: true,
      },
      () =>
        this.client.DELETE("/account/logo.json", {
        })
    );
  }

  /**
   * Rename the current account. Only account owners can use this endpoint.
   * @param req - Account_name update parameters
   * @returns The account_name
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.account.updateAccountName({ name: "My example" });
   * ```
   */
  async updateAccountName(req: UpdateAccountNameAccountRequest): Promise<components["schemas"]["UpdateAccountNameResponseContent"]> {
    if (!req.name) {
      throw Errors.validation("Name is required");
    }
    const response = await this.request(
      {
        service: "Account",
        operation: "UpdateAccountName",
        resourceType: "account_name",
        isMutation: true,
      },
      () =>
        this.client.PUT("/account/name.json", {
          body: {
            name: req.name,
          },
        })
    );
    return response;
  }
}