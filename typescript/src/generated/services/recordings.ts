/**
 * Recordings service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";

// =============================================================================
// Types
// =============================================================================

/** Recording entity from the Basecamp API. */
export type Recording = components["schemas"]["Recording"];

/**
 * Options for list.
 */
export interface ListRecordingOptions extends PaginationOptions {
  /** Project IDs to filter by */
  bucket?: number[];
  /** Filter by status */
  status?: "active" | "archived" | "trashed";
  /** Filter by sort */
  sort?: "created_at" | "updated_at";
  /** Filter by direction */
  direction?: "asc" | "desc";
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for copy.
 */
export interface CopyRecordingRequest {
  /** The container to copy into, chosen as for MoveRecording's
filing_destination_id: a container of the kind the recording sits in now,
such as a to-do list, in this project or another, for a to-do. For a dock
tool, the destination project's dock. Give this or destination_project_id. */
  filingDestinationId?: number;
  /** The project to copy a dock tool to. Applies to tools only, and wins over
filing_destination_id when a request for a tool gives both. */
  destinationProjectId?: number;
  /** When copying into a vault, a folder inside it to put the copy in. */
  parentId?: number;
  /** Copy the recording's comments too. Defaults to false. */
  includeComments?: boolean;
}

/**
 * Request parameters for moveToVault.
 */
export interface MoveToVaultRecordingRequest {
  /** The destination vault. The recording's current vault keeps it where it is
and changes only its position. */
  parentId: number;
  /** 1-indexed position within the destination vault. Defaults to 1 (first); a
position past the end places it last. */
  position?: number;
}

/**
 * Request parameters for move.
 */
export interface MoveRecordingRequest {
  /** The container to move into, of the kind the recording sits in now: a card
table for a card, a message board for a message, a vault for a document,
upload, cloud file, Google document or folder, a schedule for a schedule
entry, a questionnaire for a check-in question, an inbox for an email
forward, a to-do set for a to-do list, and a to-do list, in this project
or another, for a to-do or to-do list group. For a dock tool, the
destination project's dock. Give this or destination_project_id. */
  filingDestinationId?: number;
  /** The project to move a dock tool to. Applies to tools only, and wins over
filing_destination_id when a request for a tool gives both. */
  destinationProjectId?: number;
  /** When moving into a vault, a folder inside it to put the recording in. */
  parentId?: number;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Recordings operations.
 */
export class RecordingsService extends BaseService {

  /**
   * Get the state of a move or copy started by MoveRecording or CopyRecording.
   * @param filingId - The filing ID
   * @returns The filing
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.recordings.getFiling(123);
   * ```
   */
  async getFiling(filingId: number): Promise<components["schemas"]["GetFilingResponseContent"]> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "GetFiling",
        resourceType: "filing",
        isMutation: false,
        resourceId: filingId,
      },
      () =>
        this.client.GET("/filings/{filingId}", {
          params: {
            path: { filingId },
          },
        })
    );
    return response;
  }

  /**
   * List recordings of a given type across projects
   * @param type - Comment|Document|Door|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
   * @param options - Optional query parameters
   * @returns Every Recording across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.recordings.list("type");
   *
   * // With options
   * const filtered = await client.recordings.list("type", { bucket: [123] });
   * ```
   */
  async list(type: "Comment" | "Document" | "Door" | "Kanban::Card" | "Kanban::Step" | "Message" | "Question::Answer" | "Schedule::Entry" | "Todo" | "Todolist" | "Upload" | "Vault", options?: ListRecordingOptions): Promise<ListResult<Recording>> {
    return this.requestPaginated(
      {
        service: "Recordings",
        operation: "ListRecordings",
        resourceType: "recording",
        isMutation: false,
      },
      () =>
        this.client.GET("/projects/recordings.json", {
          params: {
            query: { type: type, bucket: options?.bucket?.join(","), status: options?.status, sort: options?.sort, direction: options?.direction, page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Get any recording by its id, without knowing its type.
   * @param recordingId - The recording ID
   * @returns The Recording
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.recordings.get(123);
   * ```
   */
  async get(recordingId: number): Promise<Recording> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "GetRecording",
        resourceType: "recording",
        isMutation: false,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/recordings/{recordingId}", {
          params: {
            path: { recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Start copying a recording into another project; poll GetFiling for the result.
   * @param recordingId - The recording ID
   * @param req - Recording request parameters
   * @returns The recording
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.recordings.copy(123, { });
   * ```
   */
  async copy(recordingId: number, req: CopyRecordingRequest): Promise<components["schemas"]["CopyRecordingResponseContent"]> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "CopyRecording",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/copies.json", {
          params: {
            path: { recordingId },
          },
          body: {
            filing_destination_id: req.filingDestinationId,
            destination_project_id: req.destinationProjectId,
            parent_id: req.parentId,
            include_comments: req.includeComments,
          },
        })
    );
    return response;
  }

  /**
   * Move a document, upload or vault into another vault in its project, or reorder it in its own vault.
   * @param recordingId - The recording ID
   * @param req - Recording request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.moveToVault(123, { parentId: 1 });
   * ```
   */
  async moveToVault(recordingId: number, req: MoveToVaultRecordingRequest): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "MoveRecordingToVault",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/filing.json", {
          params: {
            path: { recordingId },
          },
          body: {
            parent_id: req.parentId,
            position: req.position,
          },
        })
    );
  }

  /**
   * Start moving a recording into another project; poll GetFiling for the result.
   * @param recordingId - The recording ID
   * @param req - Recording request parameters
   * @returns The recording
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.recordings.move(123, { });
   * ```
   */
  async move(recordingId: number, req: MoveRecordingRequest): Promise<components["schemas"]["MoveRecordingResponseContent"]> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "MoveRecording",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/moves.json", {
          params: {
            path: { recordingId },
          },
          body: {
            filing_destination_id: req.filingDestinationId,
            destination_project_id: req.destinationProjectId,
            parent_id: req.parentId,
          },
        })
    );
    return response;
  }

  /**
   * Put a recording's card in the spotlight area on its project or template home page.
   * @param recordingId - The recording ID
   * @returns The Recording
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * const result = await client.recordings.spotlight(123);
   * ```
   */
  async spotlight(recordingId: number): Promise<Recording> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "SpotlightRecording",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.POST("/recordings/{recordingId}/spotlight.json", {
          params: {
            path: { recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Remove a recording from the spotlight area.
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.unspotlight(123);
   * ```
   */
  async unspotlight(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "UnspotlightRecording",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.DELETE("/recordings/{recordingId}/spotlight.json", {
          params: {
            path: { recordingId },
          },
        })
    );
  }

  /**
   * Unarchive a recording (restore to active status)
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.unarchive(123);
   * ```
   */
  async unarchive(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "UnarchiveRecording",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/recordings/{recordingId}/status/active.json", {
          params: {
            path: { recordingId },
          },
        })
    );
  }

  /**
   * Archive a recording; bc3 answers 403 for types it never lets be archived, timesheet entries among them
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.archive(123);
   * ```
   */
  async archive(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "ArchiveRecording",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/recordings/{recordingId}/status/archived.json", {
          params: {
            path: { recordingId },
          },
        })
    );
  }

  /**
   * Trash a recording; bc3 answers 403 for types it never lets be trashed, timesheet entries among them (DestroyTimesheetEntry removes those, permanently). Trashed items can be recovered.
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.trash(123);
   * ```
   */
  async trash(recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "TrashRecording",
        resourceType: "recording",
        isMutation: true,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/recordings/{recordingId}/status/trashed.json", {
          params: {
            path: { recordingId },
          },
        })
    );
  }
}