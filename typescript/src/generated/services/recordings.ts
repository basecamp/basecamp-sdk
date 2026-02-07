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
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Recordings operations.
 */
export class RecordingsService extends BaseService {

  /**
   * Get a single recording by id
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns The Recording
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.recordings.get(123, 123);
   * ```
   */
  async get(projectId: number, recordingId: number): Promise<Recording> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "GetRecording",
        resourceType: "recording",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
    return response;
  }

  /**
   * Unarchive a recording (restore to active status)
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.unarchive(123, 123);
   * ```
   */
  async unarchive(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "UnarchiveRecording",
        resourceType: "recording",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/active.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
  }

  /**
   * Archive a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.archive(123, 123);
   * ```
   */
  async archive(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "ArchiveRecording",
        resourceType: "recording",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/archived.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
  }

  /**
   * Trash a recording. Trashed items can be recovered.
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.recordings.trash(123, 123);
   * ```
   */
  async trash(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "TrashRecording",
        resourceType: "recording",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: {
            path: { projectId, recordingId },
          },
        })
    );
  }

  /**
   * List recordings of a given type across projects
   * @param type - Comment|Document|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
   * @param options - Optional query parameters
   * @returns All Recording across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.recordings.list("type");
   *
   * // With options
   * const filtered = await client.recordings.list("type", { bucket: [123] });
   * ```
   */
  async list(type: "Comment" | "Document" | "Kanban::Card" | "Kanban::Step" | "Message" | "Question::Answer" | "Schedule::Entry" | "Todo" | "Todolist" | "Upload" | "Vault", options?: ListRecordingOptions): Promise<ListResult<Recording>> {
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
            query: { type: type, bucket: options?.bucket?.join(","), status: options?.status, sort: options?.sort, direction: options?.direction },
          },
        })
      , options
    );
  }
}