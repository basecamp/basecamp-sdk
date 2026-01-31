/**
 * Attachments service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Attachments operations.
 */
export class AttachmentsService extends BaseService {

  /**
   * Create an attachment (upload a file for embedding)
   * @param data - Binary file data to upload
   * @param contentType - MIME type of the file (e.g., "image/png", "application/pdf")
   * @param name - name
   * @returns The attachment
   *
   * @example
   * ```ts
   * const result = await client.attachments.create(fileData, "image/png", "name");
   * ```
   */
  async create(data: ArrayBuffer | Uint8Array | string, contentType: string, name: string): Promise<components["schemas"]["CreateAttachmentResponseContent"]> {
    const response = await this.request(
      {
        service: "Attachments",
        operation: "CreateAttachment",
        resourceType: "attachment",
        isMutation: true,
      },
      () =>
        this.client.POST("/attachments.json", {
          params: {
            query: { name: name },
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            header: { "Content-Type": contentType } as any,
          },
          body: data as unknown as string,
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          bodySerializer: (body: unknown) => body as any,
        })
    );
    return response;
  }
}