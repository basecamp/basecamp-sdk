package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Recordings operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class RecordingsService(client: AccountClient) : BaseService(client) {

    /**
     * Get a single recording by id
     * @param projectId The project ID
     * @param recordingId The recording ID
     */
    suspend fun get(projectId: Long, recordingId: Long): Recording {
        val info = OperationInfo(
            service = "Recordings",
            operation = "GetRecording",
            resourceType = "recording",
            isMutation = false,
            projectId = projectId,
            resourceId = recordingId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/recordings/${recordingId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Recording>(body)
        }
    }

    /**
     * Unarchive a recording (restore to active status)
     * @param projectId The project ID
     * @param recordingId The recording ID
     */
    suspend fun unarchive(projectId: Long, recordingId: Long): Unit {
        val info = OperationInfo(
            service = "Recordings",
            operation = "UnarchiveRecording",
            resourceType = "recording",
            isMutation = true,
            projectId = projectId,
            resourceId = recordingId,
        )
        request(info, {
            httpPut("/buckets/${projectId}/recordings/${recordingId}/status/active.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Archive a recording
     * @param projectId The project ID
     * @param recordingId The recording ID
     */
    suspend fun archive(projectId: Long, recordingId: Long): Unit {
        val info = OperationInfo(
            service = "Recordings",
            operation = "ArchiveRecording",
            resourceType = "recording",
            isMutation = true,
            projectId = projectId,
            resourceId = recordingId,
        )
        request(info, {
            httpPut("/buckets/${projectId}/recordings/${recordingId}/status/archived.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Trash a recording. Trashed items can be recovered.
     * @param projectId The project ID
     * @param recordingId The recording ID
     */
    suspend fun trash(projectId: Long, recordingId: Long): Unit {
        val info = OperationInfo(
            service = "Recordings",
            operation = "TrashRecording",
            resourceType = "recording",
            isMutation = true,
            projectId = projectId,
            resourceId = recordingId,
        )
        request(info, {
            httpPut("/buckets/${projectId}/recordings/${recordingId}/status/trashed.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List recordings of a given type across projects
     * @param type Comment|Document|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(type: String, options: ListRecordingsOptions? = null): ListResult<Recording> {
        val info = OperationInfo(
            service = "Recordings",
            operation = "ListRecordings",
            resourceType = "recording",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "type" to type,
            "bucket" to options?.bucket,
            "status" to options?.status,
            "sort" to options?.sort,
            "direction" to options?.direction,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/projects/recordings.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Recording>>(body)
        }
    }
}
