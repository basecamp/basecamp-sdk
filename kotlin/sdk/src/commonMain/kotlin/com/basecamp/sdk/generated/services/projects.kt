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
     * Join the current person to an all-access project (returns 201 Created with
     * @param projectId The project ID
     */
    suspend fun joinProject(projectId: Long): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "JoinProject",
            resourceType = "project",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpPost("/buckets/${projectId}/admissions.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List the projects the current user has most recently visited, most recent visit first.
     */
    suspend fun listRecentProjects(): List<Project> {
        val info = OperationInfo(
            service = "Projects",
            operation = "ListRecentProjects",
            resourceType = "project",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/my/recent_projects.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Project>>(body)
        }
    }

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
            "page" to options?.page,
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
                body.wallpaperMode?.let { put("wallpaper_mode", kotlinx.serialization.json.JsonPrimitive(it)) }
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

    /**
     * Upload or replace the project's logo.
     * @param projectId The project ID
     * @param data Raw bytes of the file to upload
     * @param filename Display name for the uploaded file
     * @param contentType MIME type of the file (e.g., "image/png")
     */
    suspend fun updateProjectLogo(projectId: Long, data: ByteArray, filename: String, contentType: String): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "UpdateProjectLogo",
            resourceType = "project_logo",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpPutMultipart("/projects/${projectId}/logo.json", "logo", data, filename, contentType, operationName = info.operation)
        }) { Unit }
    }

    /**
     * Remove the project's logo. Wallpaper is unchanged.
     * @param projectId The project ID
     */
    suspend fun removeProjectLogo(projectId: Long): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "RemoveProjectLogo",
            resourceType = "project_logo",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpDelete("/projects/${projectId}/logo.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Record that the current user visited a project, moving it to the front of ListRecentProjects.
     * @param projectId The project ID
     */
    suspend fun recordProjectVisit(projectId: Long): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "RecordProjectVisit",
            resourceType = "project",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpPost("/projects/${projectId}/recent_visit.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Restore a project to active status from trash as well as from the archive.
     * @param projectId The project ID
     */
    suspend fun unarchive(projectId: Long): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "UnarchiveProject",
            resourceType = "project",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpPut("/projects/${projectId}/status/active.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Archive a project, removing it from the active project list.
     * @param projectId The project ID
     */
    suspend fun archive(projectId: Long): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "ArchiveProject",
            resourceType = "project",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpPut("/projects/${projectId}/status/archived.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Upload or replace the project's wallpaper.
     * @param projectId The project ID
     * @param data Raw bytes of the file to upload
     * @param filename Display name for the uploaded file
     * @param contentType MIME type of the file (e.g., "image/png")
     * @param options Optional query parameters and pagination control
     */
    suspend fun updateProjectWallpaper(projectId: Long, data: ByteArray, filename: String, contentType: String, options: UpdateProjectWallpaperOptions? = null): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "UpdateProjectWallpaper",
            resourceType = "project_wallpaper",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        val qs = buildQueryString(
            "mode" to options?.mode,
        )
        request(info, {
            httpPutMultipart("/projects/${projectId}/wallpaper.json" + qs, "wallpaper", data, filename, contentType, operationName = info.operation)
        }) { Unit }
    }

    /**
     * Remove the project's wallpaper.
     * @param projectId The project ID
     */
    suspend fun removeProjectWallpaper(projectId: Long): Unit {
        val info = OperationInfo(
            service = "Projects",
            operation = "RemoveProjectWallpaper",
            resourceType = "project_wallpaper",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        request(info, {
            httpDelete("/projects/${projectId}/wallpaper.json", operationName = info.operation)
        }) { Unit }
    }
}
