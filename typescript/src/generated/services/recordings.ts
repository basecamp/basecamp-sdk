/**
 * Service for Recordings operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for Recordings operations
 */
export class RecordingsService extends BaseService {

  /**
   * Get a single recording by id
   */
  async get(projectId: number, recordingId: number): Promise<components["schemas"]["GetRecordingResponseContent"]> {
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
   */
  async list(type: string, options?: { bucket?: string; status?: string; sort?: string; direction?: string }): Promise<components["schemas"]["ListRecordingsResponseContent"]> {
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