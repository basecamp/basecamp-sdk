package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Templates operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class TemplatesService(client: AccountClient) : BaseService(client) {

    /**
     * Get the account's to-do list template library
     */
    suspend fun getLibrary(): TemplateLibrary {
        val info = OperationInfo(
            service = "Templates",
            operation = "GetTemplateLibrary",
            resourceType = "template_library",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/template_library.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<TemplateLibrary>(body)
        }
    }

    /**
     * Start copying a to-do list template into a project
     * @param body Request body
     */
    suspend fun createLibraryCopy(body: CreateTemplateLibraryCopyBody): TemplateLibraryCopy {
        val info = OperationInfo(
            service = "Templates",
            operation = "CreateTemplateLibraryCopy",
            resourceType = "template_library_copy",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/template_library/copies.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("template_recording_id", kotlinx.serialization.json.JsonPrimitive(body.templateRecordingId))
                put("destination_parent_id", kotlinx.serialization.json.JsonPrimitive(body.destinationParentId))
                body.addingPeopleConfirmed?.let { put("adding_people_confirmed", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<TemplateLibraryCopy>(body)
        }
    }

    /**
     * Get the current status of a to-do list template copy
     * @param copyId The copy ID
     */
    suspend fun getLibraryCopy(copyId: Long): TemplateLibraryCopy {
        val info = OperationInfo(
            service = "Templates",
            operation = "GetTemplateLibraryCopy",
            resourceType = "template_library_copy",
            isMutation = false,
            projectId = null,
            resourceId = copyId,
        )
        return request(info, {
            httpGet("/template_library/copies/${copyId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<TemplateLibraryCopy>(body)
        }
    }

    /**
     * List all templates visible to the current user
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: ListTemplatesOptions? = null): ListResult<Template> {
        val info = OperationInfo(
            service = "Templates",
            operation = "ListTemplates",
            resourceType = "template",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "status" to options?.status,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/templates.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Template>>(body)
        }
    }

    /**
     * Create a new template
     * @param body Request body
     */
    suspend fun create(body: CreateTemplateBody): Template {
        val info = OperationInfo(
            service = "Templates",
            operation = "CreateTemplate",
            resourceType = "template",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/templates.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Template>(body)
        }
    }

    /**
     * Get a single template by id
     * @param templateId The template ID
     */
    suspend fun get(templateId: Long): Template {
        val info = OperationInfo(
            service = "Templates",
            operation = "GetTemplate",
            resourceType = "template",
            isMutation = false,
            projectId = null,
            resourceId = templateId,
        )
        return request(info, {
            httpGet("/templates/${templateId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Template>(body)
        }
    }

    /**
     * Update an existing template
     * @param templateId The template ID
     * @param body Request body
     */
    suspend fun update(templateId: Long, body: UpdateTemplateBody): Template {
        val info = OperationInfo(
            service = "Templates",
            operation = "UpdateTemplate",
            resourceType = "template",
            isMutation = true,
            projectId = null,
            resourceId = templateId,
        )
        return request(info, {
            httpPut("/templates/${templateId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.name?.let { put("name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Template>(body)
        }
    }

    /**
     * Delete a template (trash it)
     * @param templateId The template ID
     */
    suspend fun delete(templateId: Long): Unit {
        val info = OperationInfo(
            service = "Templates",
            operation = "DeleteTemplate",
            resourceType = "template",
            isMutation = true,
            projectId = null,
            resourceId = templateId,
        )
        request(info, {
            httpDelete("/templates/${templateId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * Create a project from a template (asynchronous)
     * @param templateId The template ID
     * @param body Request body
     */
    suspend fun createProject(templateId: Long, body: CreateProjectFromTemplateBody): JsonElement {
        val info = OperationInfo(
            service = "Templates",
            operation = "CreateProjectFromTemplate",
            resourceType = "project_from_template",
            isMutation = true,
            projectId = null,
            resourceId = templateId,
        )
        return request(info, {
            httpPost("/templates/${templateId}/project_constructions.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("project", body.project)
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get the status of a project construction
     * @param templateId The template ID
     * @param constructionId The construction ID
     */
    suspend fun getConstruction(templateId: Long, constructionId: Long): JsonElement {
        val info = OperationInfo(
            service = "Templates",
            operation = "GetProjectConstruction",
            resourceType = "project_construction",
            isMutation = false,
            projectId = null,
            resourceId = constructionId,
        )
        return request(info, {
            httpGet("/templates/${templateId}/project_constructions/${constructionId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
