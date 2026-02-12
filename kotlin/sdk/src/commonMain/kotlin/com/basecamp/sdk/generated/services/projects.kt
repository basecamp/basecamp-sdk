package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Projects operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class ProjectsService(client: AccountClient) : BaseService(client) {

    /**
     * List projects (active by default; optionally archived/trashed)
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: ListProjectsOptions? = null): ListResult<Project> {
        val info = OperationInfo(
            service = "Projects",
            operation = "ListProjects",
            resourceType = "project",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "status" to options?.status,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/projects.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Project>>(body)
        }
    }

    /**
     * Create a new project
     * @param body Request body
     */
    suspend fun create(body: CreateProjectBody): Project {
        val info = OperationInfo(
            service = "Projects",
            operation = "CreateProject",
            resourceType = "project",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/projects.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Project>(body)
        }
    }

    /**
     * Get a single project by id
     * @param projectId The project ID
     */
    suspend fun get(projectId: Long): Project {
        val info = OperationInfo(
            service = "Projects",
            operation = "GetProject",
            resourceType = "project",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        return request(info, {
            httpGet("/projects/${projectId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Project>(body)
        }
    }

    /**
     * Update an existing project
     * @param projectId The project ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, body: UpdateProjectBody): Project {
        val info = OperationInfo(
            service = "Projects",
            operation = "UpdateProject",
            resourceType = "project",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        return request(info, {
            httpPut("/projects/${projectId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.admissions?.let { put("admissions", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.scheduleAttributes?.let { put("schedule_attributes", it) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Project>(body)
        }
    }

    /**
     * Trash a project. Trashed items can be recovered.
     * @param projectId The project ID
     */
    suspend fun trash(projectId: Long): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "TrashProject",
            resourceType = "project",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpDelete("/projects/${projectId}", operationName = info.operation)
        }) { Unit }
    }
}
