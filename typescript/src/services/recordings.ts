/**
 * Recordings service for the Basecamp API.
 *
 * Recordings are the base type for most content in Basecamp including
 * messages, todos, comments, documents, uploads, and more. This service
 * provides cross-cutting operations like listing, archiving, trashing,
 * and setting client visibility.
 *
 * @example
 * ```ts
 * // List all todos across projects
 * const todos = await client.recordings.list("Todo");
 *
 * // Archive a recording
 * await client.recordings.archive(projectId, recordingId);
 *
 * // Set client visibility
 * await client.recordings.setClientVisibility(projectId, recordingId, true);
 * ```
 */

import { BaseService } from "./base.js";
import { Errors } from "../errors.js";
import type { components } from "../generated/schema.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A Basecamp recording.
 */
export type Recording = components["schemas"]["Recording"];

/**
 * Recording parent reference.
 */
export type RecordingParent = components["schemas"]["RecordingParent"];

/**
 * Recording bucket (project) reference.
 */
export type RecordingBucket = components["schemas"]["RecordingBucket"];

/**
 * A person associated with the recording (creator).
 */
export type Person = components["schemas"]["Person"];

/**
 * Recording types supported by the Basecamp API.
 */
export type RecordingType =
  | "Comment"
  | "Document"
  | "Kanban::Card"
  | "Kanban::Step"
  | "Message"
  | "Question::Answer"
  | "Schedule::Entry"
  | "Todo"
  | "Todolist"
  | "Upload"
  | "Vault";

/**
 * Valid recording statuses.
 */
export type RecordingStatus = "active" | "archived" | "trashed";

/**
 * Sort fields for recording lists.
 */
export type RecordingSortField = "created_at" | "updated_at";

/**
 * Sort directions for recording lists.
 */
export type RecordingSortDirection = "asc" | "desc";

/**
 * Options for listing recordings.
 */
export interface RecordingsListOptions {
  /**
   * Filter by project IDs.
   * Defaults to all active projects visible to the user.
   */
  bucket?: number[];
  /**
   * Filter by recording status.
   * Defaults to "active".
   */
  status?: RecordingStatus;
  /**
   * Sort field.
   * Defaults to "created_at".
   */
  sort?: RecordingSortField;
  /**
   * Sort direction.
   * Defaults to "desc".
   */
  direction?: RecordingSortDirection;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing Basecamp recordings.
 */
export class RecordingsService extends BaseService {
  /**
   * Lists all recordings of a given type across projects.
   *
   * @param type - The recording type to list
   * @param options - Optional filters and sorting
   * @returns Array of recordings
   *
   * @example
   * ```ts
   * // List all active todos
   * const todos = await client.recordings.list("Todo");
   *
   * // List archived documents
   * const docs = await client.recordings.list("Document", { status: "archived" });
   *
   * // List messages in specific projects, sorted by updated_at
   * const messages = await client.recordings.list("Message", {
   *   bucket: [projectId1, projectId2],
   *   sort: "updated_at",
   *   direction: "desc",
   * });
   * ```
   */
  async list(type: RecordingType, options?: RecordingsListOptions): Promise<Recording[]> {
    if (!type) {
      throw Errors.validation("Recording type is required");
    }

    const query: {
      type: string;
      bucket?: string;
      status?: string;
      sort?: string;
      direction?: string;
    } = { type };

    if (options?.bucket?.length) {
      query.bucket = options.bucket.join(",");
    }
    if (options?.status) {
      query.status = options.status;
    }
    if (options?.sort) {
      query.sort = options.sort;
    }
    if (options?.direction) {
      query.direction = options.direction;
    }

    const response = await this.request(
      {
        service: "Recordings",
        operation: "List",
        resourceType: "recording",
        isMutation: false,
      },
      () =>
        this.client.GET("/projects/recordings.json", {
          params: { query },
        })
    );

    return response?.recordings ?? [];
  }

  /**
   * Gets a recording by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   * @returns The recording
   * @throws BasecampError with code "not_found" if recording doesn't exist
   *
   * @example
   * ```ts
   * const recording = await client.recordings.get(projectId, recordingId);
   * console.log(recording.type, recording.title, recording.status);
   * ```
   */
  async get(projectId: number, recordingId: number): Promise<Recording> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "Get",
        resourceType: "recording",
        isMutation: false,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/recordings/{recordingId}", {
          params: { path: { projectId, recordingId } },
        })
    );

    return response.recording!;
  }

  /**
   * Moves a recording to the trash.
   * Trashed recordings can be recovered from the trash.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   *
   * @example
   * ```ts
   * await client.recordings.trash(projectId, recordingId);
   * ```
   */
  async trash(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "Trash",
        resourceType: "recording",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/trashed.json", {
          params: { path: { projectId, recordingId } },
        })
    );
  }

  /**
   * Archives a recording.
   * Archived recordings are hidden but not deleted.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   *
   * @example
   * ```ts
   * await client.recordings.archive(projectId, recordingId);
   * ```
   */
  async archive(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "Archive",
        resourceType: "recording",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/archived.json", {
          params: { path: { projectId, recordingId } },
        })
    );
  }

  /**
   * Unarchives a recording, restoring it to active status.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   *
   * @example
   * ```ts
   * await client.recordings.unarchive(projectId, recordingId);
   * ```
   */
  async unarchive(projectId: number, recordingId: number): Promise<void> {
    await this.request(
      {
        service: "Recordings",
        operation: "Unarchive",
        resourceType: "recording",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/status/active.json", {
          params: { path: { projectId, recordingId } },
        })
    );
  }

  /**
   * Sets whether a recording is visible to clients.
   * Not all recordings support client visibility. Some inherit visibility
   * from their parent.
   *
   * @param projectId - The project (bucket) ID
   * @param recordingId - The recording ID
   * @param visible - Whether the recording should be visible to clients
   * @returns The updated recording
   *
   * @example
   * ```ts
   * // Make a recording visible to clients
   * const updated = await client.recordings.setClientVisibility(
   *   projectId,
   *   recordingId,
   *   true
   * );
   *
   * // Hide a recording from clients
   * await client.recordings.setClientVisibility(projectId, recordingId, false);
   * ```
   */
  async setClientVisibility(
    projectId: number,
    recordingId: number,
    visible: boolean
  ): Promise<Recording> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "SetClientVisibility",
        resourceType: "recording",
        isMutation: true,
        projectId,
        resourceId: recordingId,
      },
      () =>
        this.client.PUT("/buckets/{projectId}/recordings/{recordingId}/client_visibility.json", {
          params: { path: { projectId, recordingId } },
          body: {
            visible_to_clients: visible,
          },
        })
    );

    return response.recording!;
  }
}
