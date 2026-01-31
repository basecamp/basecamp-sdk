/**
 * Documents service for the Basecamp API.
 *
 * Documents are rich text files stored within vaults. They support
 * HTML content and can be in draft or active status.
 *
 * @example
 * ```ts
 * // Get a document
 * const doc = await client.documents.get(projectId, documentId);
 *
 * // List documents in a vault
 * const docs = await client.documents.list(projectId, vaultId);
 *
 * // Create a new document
 * const newDoc = await client.documents.create(projectId, vaultId, {
 *   title: "Meeting Notes",
 *   content: "<p>Notes from today's meeting...</p>",
 * });
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp document.
 */
export type Document = components["schemas"]["Document"];

/**
 * A person associated with the document (creator).
 */
export type Person = components["schemas"]["Person"];

/**
 * Valid document statuses.
 */
export type DocumentStatus = "drafted" | "active";

/**
 * Request to create a new document.
 */
export interface CreateDocumentRequest {
  /** Document title (required) */
  title: string;
  /** Document body in HTML (optional) */
  content?: string;
  /** Status: "drafted" or "active" (optional, defaults to active) */
  status?: DocumentStatus;
}

/**
 * Request to update an existing document.
 */
export interface UpdateDocumentRequest {
  /** Document title (optional) */
  title?: string;
  /** Document body in HTML (optional) */
  content?: string;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp documents.
 */
export class DocumentsService extends BaseService {
  /**
   * Gets a document by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param documentId - The document ID
   * @returns The document
   * @throws BasecampError with code "not_found" if document doesn't exist
   *
   * @example
   * ```ts
   * const doc = await client.documents.get(projectId, documentId);
   * console.log(doc.title, doc.content);
   * ```
   */
  async get(projectId: number, documentId: number): Promise<Document> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "Get",
        resourceType: "document",
        isMutation: false,
        projectId,
        resourceId: documentId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/documents/{documentId}", {
          params: { path: { projectId, documentId } },
        })
    );

    return response;
  }

  /**
   * Lists all documents in a vault.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The vault ID
   * @returns Array of documents
   *
   * @example
   * ```ts
   * const docs = await client.documents.list(projectId, vaultId);
   * for (const doc of docs) {
   *   console.log(doc.title, doc.comments_count);
   * }
   * ```
   */
  async list(projectId: number, vaultId: number): Promise<Document[]> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "List",
        resourceType: "document",
        isMutation: false,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/vaults/{vaultId}/documents.json", {
          params: { path: { projectId, vaultId } },
        })
    );

    return response ?? [];
  }

  /**
   * Creates a new document in a vault.
   *
   * @param projectId - The project (bucket) ID
   * @param vaultId - The vault ID
   * @param req - Document creation parameters
   * @returns The created document
   * @throws BasecampError with code "validation" if title is missing
   *
   * @example
   * ```ts
   * // Create an active document
   * const doc = await client.documents.create(projectId, vaultId, {
   *   title: "Project Plan",
   *   content: "<h1>Project Plan</h1><p>Details here...</p>",
   * });
   *
   * // Create a draft document
   * const draft = await client.documents.create(projectId, vaultId, {
   *   title: "Work in Progress",
   *   status: "drafted",
   * });
   * ```
   */
  async create(projectId: number, vaultId: number, req: CreateDocumentRequest): Promise<Document> {
    if (!req.title) {
      throw Errors.validation("Document title is required");
    }

    const response = await this.request(
      {
        service: "Documents",
        operation: "Create",
        resourceType: "document",
        isMutation: true,
        projectId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{projectId}/vaults/{vaultId}/documents.json", {
          params: { path: { projectId, vaultId } },
          body: {
            title: req.title,
            content: req.content,
            status: req.status,
          },
        })
    );

    return response;
  }

  /**
   * Updates an existing document.
   *
   * @param projectId - The project (bucket) ID
   * @param documentId - The document ID
   * @param req - Document update parameters
   * @returns The updated document
   *
   * @example
   * ```ts
   * const updated = await client.documents.update(projectId, documentId, {
   *   title: "Updated Title",
   *   content: "<p>New content...</p>",
   * });
   * ```
   */
  async update(projectId: number, documentId: number, req: UpdateDocumentRequest): Promise<Document> {
    const response = await this.request(
      {
        service: "Documents",
        operation: "Update",
        resourceType: "document",
        isMutation: true,
        projectId,
        resourceId: documentId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/documents/{documentId}", {
          params: { path: { projectId, documentId } },
          body: {
            title: req.title,
            content: req.content,
          },
        })
    );

    return response;
  }

  /**
   * Moves a document to the trash.
   * Trashed documents can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param documentId - The document ID
   *
   * @example
   * ```ts
   * await client.documents.trash(projectId, documentId);
   * ```
   */
  async trash(projectId: number, documentId: number): Promise<void> {
    await this.request(
      {
        service: "Documents",
        operation: "Trash",
        resourceType: "document",
        isMutation: true,
        projectId,
        resourceId: documentId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId: documentId } },
        })
    );
  }
}
