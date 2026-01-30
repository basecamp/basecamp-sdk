/**
 * Attachments service for the Basecamp API.
 *
 * Attachments are used to upload files that can be embedded in rich text content
 * like messages, comments, and documents. After uploading, you receive an
 * attachable_sgid that can be used to embed the file in HTML content.
 *
 * @example
 * ```ts
 * // Upload a file
 * const attachment = await client.attachments.create({
 *   filename: "report.pdf",
 *   contentType: "application/pdf",
 *   data: fileBuffer,
 * });
 *
 * // Use the SGID in HTML content
 * const html = `<bc-attachment sgid="${attachment.attachableSgid}"></bc-attachment>`;
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";

// =============================================================================
// Types
// =============================================================================

/**
 * Response from creating an attachment.
 */
export interface AttachmentResponse {
  /** The signed global ID for embedding in rich text content */
  attachableSgid: string;
}

/**
 * Request to create a new attachment.
 */
export interface CreateAttachmentRequest {
  /** Filename for the uploaded file (required) */
  filename: string;
  /** MIME content type (e.g., "image/png", "application/pdf") (required) */
  contentType: string;
  /** File data as an ArrayBuffer, Uint8Array, or Blob (required) */
  data: ArrayBuffer | Uint8Array | Blob;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for uploading file attachments.
 */
export class AttachmentsService extends BaseService {
  /**
   * Creates an attachment by uploading a file.
   * Returns an attachable_sgid for embedding the file in rich text content.
   *
   * @param req - Attachment creation parameters
   * @returns The attachment response with attachable_sgid
   * @throws BasecampError with code "validation" if required fields are missing
   *
   * @example
   * ```ts
   * // Upload an image
   * const imageBuffer = await fs.readFile("photo.jpg");
   * const attachment = await client.attachments.create({
   *   filename: "photo.jpg",
   *   contentType: "image/jpeg",
   *   data: imageBuffer,
   * });
   *
   * // Embed in a message
   * await client.messages.create(projectId, boardId, {
   *   subject: "New photo",
   *   content: `<bc-attachment sgid="${attachment.attachableSgid}"></bc-attachment>`,
   * });
   * ```
   */
  async create(req: CreateAttachmentRequest): Promise<AttachmentResponse> {
    if (!req.filename) {
      throw Errors.validation("Attachment filename is required");
    }
    if (!req.contentType) {
      throw Errors.validation("Attachment content type is required");
    }
    if (!req.data) {
      throw Errors.validation("Attachment data is required");
    }

    // Check if data has content
    const size =
      req.data instanceof Blob
        ? req.data.size
        : req.data instanceof ArrayBuffer
          ? req.data.byteLength
          : req.data.byteLength;

    if (size === 0) {
      throw Errors.validation("Attachment data cannot be empty");
    }

    const response = await this.request(
      {
        service: "Attachments",
        operation: "Create",
        resourceType: "attachment",
        isMutation: true,
      },
      () =>
        this.client.POST("/attachments.json", {
          params: {
            query: { name: req.filename },
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            header: { "Content-Type": req.contentType } as any,
          },
          body: req.data as unknown as string,
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          bodySerializer: (body: unknown) => body as any,
        })
    );

    // Response now correctly returns { attachable_sgid: string }
    return {
      attachableSgid: response?.attachable_sgid ?? "",
    };
  }
}
