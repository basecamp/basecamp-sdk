/**
 * GoogleDocuments service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================

/** GoogleDocument entity from the Basecamp API. */
export type GoogleDocument = components["schemas"]["GoogleDocument"];

/**
 * Request parameters for createGoogleDocument.
 */
export interface CreateGoogleDocumentGoogleDocumentRequest {
  /** Url */
  url: string;
  /** One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
unrecognized value is rejected up front with a field-keyed 422
({"errors": {"document_type": ["is not a valid document type"]}}) rather
than reaching validation. */
  documentType: string;
  /** Title */
  title?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** active|drafted — defaults to drafted */
  status?: string;
  /** Subscriptions */
  subscriptions?: number[];
  /** Whether the document is visible to the project's clients. Applies only
when creating directly in the tool's vault — an item created inside a
folder inherits the folder's visibility and ignores this. A client caller
always creates client-visible records regardless of what is sent. */
  visibleToClients?: boolean;
}

/**
 * Request parameters for updateGoogleDocument.
 */
export interface UpdateGoogleDocumentGoogleDocumentRequest {
  /** Url */
  url: string;
  /** One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
unrecognized value is rejected up front with a field-keyed 422
({"errors": {"document_type": ["is not a valid document type"]}}) rather
than reaching validation. */
  documentType: string;
  /** Title */
  title?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Status */
  status?: "active" | "drafted";
  /** Subscriptions */
  subscriptions?: number[];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for GoogleDocuments operations.
 */
export class GoogleDocumentsService extends BaseService {

  /**
   * Create a new Google document in a vault.
   * @param bucketId - The bucket ID
   * @param vaultId - The vault ID
   * @param req - Google_document creation parameters
   * @returns The GoogleDocument
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.googleDocuments.createGoogleDocument(123, 123, { url: "example", documentType: "example" });
   * ```
   */
  async createGoogleDocument(bucketId: number, vaultId: number, req: CreateGoogleDocumentGoogleDocumentRequest): Promise<GoogleDocument> {
    if (!req.url) {
      throw Errors.validation("Url is required");
    }
    if (!req.documentType) {
      throw Errors.validation("Document type is required");
    }
    const response = await this.request(
      {
        service: "GoogleDocuments",
        operation: "CreateGoogleDocument",
        resourceType: "google_document",
        isMutation: true,
        projectId: bucketId,
        resourceId: vaultId,
      },
      () =>
        this.client.POST("/buckets/{bucketId}/vaults/{vaultId}/google_documents.json", {
          params: {
            path: { bucketId, vaultId },
          },
          body: {
            url: req.url,
            document_type: req.documentType,
            title: req.title,
            description: req.description,
            status: req.status,
            subscriptions: req.subscriptions,
            visible_to_clients: req.visibleToClients,
          },
        })
    );
    return response;
  }

  /**
   * Get a single Google document by id
   * @param googleDocumentId - The google document ID
   * @returns The GoogleDocument
   *
   * @example
   * ```ts
   * const result = await client.googleDocuments.googleDocument(123);
   * ```
   */
  async googleDocument(googleDocumentId: number): Promise<GoogleDocument> {
    const response = await this.request(
      {
        service: "GoogleDocuments",
        operation: "GetGoogleDocument",
        resourceType: "google_document",
        isMutation: false,
        resourceId: googleDocumentId,
      },
      () =>
        this.client.GET("/google_documents/{googleDocumentId}", {
          params: {
            path: { googleDocumentId },
          },
        })
    );
    return response;
  }

  /**
   * Replace a Google document with a new complete representation.
   * @param googleDocumentId - The google document ID
   * @param req - Google_document update parameters
   * @returns The GoogleDocument
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.googleDocuments.updateGoogleDocument(123, { url: "example", documentType: "example" });
   * ```
   */
  async updateGoogleDocument(googleDocumentId: number, req: UpdateGoogleDocumentGoogleDocumentRequest): Promise<GoogleDocument> {
    if (!req.url) {
      throw Errors.validation("Url is required");
    }
    if (!req.documentType) {
      throw Errors.validation("Document type is required");
    }
    const response = await this.request(
      {
        service: "GoogleDocuments",
        operation: "UpdateGoogleDocument",
        resourceType: "google_document",
        isMutation: true,
        resourceId: googleDocumentId,
      },
      () =>
        this.client.PUT("/google_documents/{googleDocumentId}", {
          params: {
            path: { googleDocumentId },
          },
          body: {
            url: req.url,
            document_type: req.documentType,
            title: req.title,
            description: req.description,
            status: req.status,
            subscriptions: req.subscriptions,
          },
        })
    );
    return response;
  }
}