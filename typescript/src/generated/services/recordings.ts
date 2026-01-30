/**
 * Recordings service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** Recording entity from the Basecamp API. */
export type Recording = components["schemas"]["Recording"];

/**
 * Options for list.
 */
export interface ListRecordingOptions {
  /** bucket */
  bucket?: string;
  /** active|archived|trashed */
  status?: string;
  /** created_at|updated_at */
  sort?: string;
  /** asc|desc */
  direction?: string;
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
   * Trash a recording
   * @param projectId - The project ID
   * @param recordingId - The recording ID
   * @returns void
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
   * @param options - Optional parameters
   * @returns Array of Recording
   */
  async list(type: string, options?: ListRecordingOptions): Promise<Recording[]> {
    const response = await this.request(
      {
        service: "Recordings",
        operation: "ListRecordings",
        resourceType: "recording",
        isMutation: false,
      },
      () =>
        this.client.GET("/projects/recordings.json", {
          params: {
            query: { type: type, bucket: options?.bucket, status: options?.status, sort: options?.sort, direction: options?.direction },
          },
        })
    );
    return response ?? [];
  }
}