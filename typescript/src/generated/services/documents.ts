/**
 * Documents service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Document entity from the Basecamp API. */
export type Document = components["schemas"]["Document"];

/**
 * Request parameters for update.
 */
export interface UpdateDocumentRequest {
  /** title */
  title?: string;
  /** content */
  content?: string;
}

/**
 * Request parameters for create.
 */
export interface CreateDocumentRequest {
  /** title */
  title: string;
  /** content */
  content?: string;
  /** active|drafted */
  status?: string;
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
   * @param projectId - The project ID
   * @param documentId - The document ID
   * @returns The Document
   */
  async get(projectId: number, documentId: number): Promise<Document> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "GetDocument",
        resourceType: "document",
        isMutation: false,
        projectId,
        resourceId: documentId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/documents/{documentId}", {
          params: {
            path: { projectId, documentId },
          },
        })
    );
    return response;
  }

  /**
   * Update an existing document
   * @param projectId - The project ID
   * @param documentId - The document ID
   * @param req - Request parameters
   * @returns The Document
   */
  async update(projectId: number, documentId: number, req: UpdateDocumentRequest): Promise<Document> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "UpdateDocument",
        resourceType: "document",
        isMutation: true,
        projectId,
        resourceId: documentId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/documents/{documentId}", {
          params: {
            path: { projectId, documentId },
          },
          body: req as any,
        })
    );
    return response;
  }

  /**
   * List documents in a vault
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @returns Array of Document
   */
  async list(projectId: number, vaultId: number): Promise<Document[]> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "ListDocuments",
        resourceType: "document",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}/documents.json", {
          params: {
            path: { projectId, vaultId },
          },
        })
    );
    return response ?? [];
  }

  /**
   * Create a new document in a vault
   * @param projectId - The project ID
   * @param vaultId - The vault ID
   * @param req - Request parameters
   * @returns The Document
   *
   * @example
   * ```ts
   * const result = await client.documents.create(123, 123, { ... });
   * ```
   */
  async create(projectId: number, vaultId: number, req: CreateDocumentRequest): Promise<Document> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "CreateDocument",
        resourceType: "document",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/vaults/{vaultId}/documents.json", {
          params: {
            path: { projectId, vaultId },
          },
          body: req as any,
        })
    );
    return response;
  }
}