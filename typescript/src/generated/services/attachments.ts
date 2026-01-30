/**
 * Service for Attachments operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Attachments operations
 */
export class AttachmentsService extends BaseService {

  /**
   * Create an attachment (upload a file for embedding)
   */
  async create(data: components["schemas"]["CreateAttachmentInputPayload"], contentType: string, name: string): Promise<components["schemas"]["CreateAttachmentResponseContent"]> {
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