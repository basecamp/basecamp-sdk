/**
 * Documents service for the Basecamp API.
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

/** Document entity from the Basecamp API. */
export type Document = components["schemas"]["Document"];

/**
 * Request parameters for update.
 */
export interface UpdateDocumentRequest {
  /** Title */
  title?: string;
  /** Text content */
  content?: string;
}

/**
 * Options for list.
 */
export interface ListDocumentOptions extends PaginationOptions {
}

/**
 * Request parameters for create.
 */
export interface CreateDocumentRequest {
  /** Title */
  title: string;
  /** Text content */
  content?: string;
  /** Status */
  status?: "active" | "drafted";
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Documents operations.
 */
export class DocumentsService extends BaseService {

  /**
   * Get a single document by id
   * @param documentId - The document ID
   * @returns The Document
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.documents.get(123);
   * ```
   */
  async get(documentId: number): Promise<Document> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "GetDocument",
        resourceType: "document",
        isMutation: false,
        resourceId: documentId,
      },
      () =>
        this.client.GET("/documents/{documentId}", {
          params: {
            path: { documentId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing document
   * @param documentId - The document ID
   * @param req - Document update parameters
   * @returns The Document
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.documents.update(123, { });
   * ```
   */
  async update(documentId: number, req: UpdateDocumentRequest): Promise<Document> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "UpdateDocument",
        resourceType: "document",
        isMutation: true,
        resourceId: documentId,
      },
      () =>
        this.client.PUT("/documents/{documentId}", {
          params: {
            path: { documentId },
          },
          body: {
            title: req.title,
            content: req.content,
          },
        })
    );
    return response;
  }

  /**
   * List documents in a vault
   * @param vaultId - The vault ID
   * @param options - Optional query parameters
   * @returns All Document across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.documents.list(123);
   * ```
   */
  async list(vaultId: number, options?: ListDocumentOptions): Promise<ListResult<Document>> {
    return this.requestPaginated(
      {
        service: "Documents",
        operation: "ListDocuments",
        resourceType: "document",
        isMutation: false,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/vaults/{vaultId}/documents.json", {
          params: {
            path: { vaultId },
          },
        })
      , options
    );
  }

  /**
   * Create a new document in a vault
   * @param vaultId - The vault ID
   * @param req - Document creation parameters
   * @returns The Document
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.documents.create(123, { title: "example" });
   * ```
   */
  async create(vaultId: number, req: CreateDocumentRequest): Promise<Document> {
    if (!req.title) {
      throw Errors.validation("Title is required");
    }
    const response = await this.request(
      {
        service: "Documents",
        operation: "CreateDocument",
        resourceType: "document",
        isMutation: true,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/vaults/{vaultId}/documents.json", {
          params: {
            path: { vaultId },
          },
          body: {
            title: req.title,
            content: req.content,
            status: req.status,
          },
        })
    );
    return response;
  }
}