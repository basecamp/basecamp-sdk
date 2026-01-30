/**
 * Service for Documents operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Documents operations
 */
export class DocumentsService extends BaseService {

  /**
   * Get a single document by id
   */
  async get(projectId: number, documentId: number): Promise<components["schemas"]["GetDocumentResponseContent"]> {
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
   */
  async update(projectId: number, documentId: number, req: components["schemas"]["UpdateDocumentRequestContent"]): Promise<components["schemas"]["UpdateDocumentResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }

  /**
   * List documents in a vault
   */
  async list(projectId: number, vaultId: number): Promise<components["schemas"]["ListDocumentsResponseContent"]> {
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
   */
  async create(projectId: number, vaultId: number, req: components["schemas"]["CreateDocumentRequestContent"]): Promise<components["schemas"]["CreateDocumentResponseContent"]> {
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
          body: req,
        })
    );
    return response;
  }
}