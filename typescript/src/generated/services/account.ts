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
   * Upload or replace the account logo via multipart form upload.
   * @param data - Binary file data to upload
   * @param contentType - MIME type of the file (e.g., "image/png", "application/pdf")
   * @returns void
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * await client.account.updateAccountLogo(fileData, "image/png");
   * ```
   */
  async updateAccountLogo(data: ArrayBuffer | Uint8Array | string, contentType: string): Promise<void> {
    await this.request(
      {
        service: "Account",
        operation: "UpdateAccountLogo",
        resourceType: "account_logo",
        isMutation: true,
      },
      () =>
        this.client.PUT("/account/logo.json", {
          params: {
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            header: { "Content-Type": contentType } as any,
          },
          body: data as unknown as string,
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          bodySerializer: (body: unknown) => body as any,
        })
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