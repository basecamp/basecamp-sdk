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
     * Templatify a to-do list or card table
     * @param bucketId The bucket ID
     * @param recordingId The to-do list or card table to templatify. Anything else is a 403.
     * @param body Request body
     */
    suspend fun createTemplatification(bucketId: Long, recordingId: Long, body: CreateTemplatificationBody): JsonElement {
        val info = OperationInfo(
            service = "Templates",
            operation = "CreateTemplatification",
            resourceType = "templatification",
            isMutation = true,
            projectId = bucketId,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/recordings/${recordingId}/templatifications.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.templateName?.let { put("template_name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.copyComments?.let { put("copy_comments", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.copyAssignments?.let { put("copy_assignments", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.moveCardsToTriage?.let { put("move_cards_to_triage", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get a templatification
     * @param bucketId The bucket ID
     * @param recordingId The recording ID
     * @param templatificationId The templatification ID
     */
    suspend fun getTemplatification(bucketId: Long, recordingId: Long, templatificationId: Long): JsonElement {
        val info = OperationInfo(
            service = "Templates",
            operation = "GetTemplatification",
            resourceType = "templatification",
            isMutation = false,
            projectId = bucketId,
            resourceId = templatificationId,
        )
        return request(info, {
            httpGet("/buckets/${bucketId}/recordings/${recordingId}/templatifications/${templatificationId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Get the account's card table templates
     */
    suspend fun getLibraryCardTables(): TemplateLibraryCardTables {
        val info = OperationInfo(
            service = "Templates",
            operation = "GetTemplateLibraryCardTables",
            resourceType = "template_library_card_table",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/template_library/card_tables.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<TemplateLibraryCardTables>(body)
        }
    }

    /**
     * Create a card table template with the default columns
     * @param body Request body
     */
    suspend fun createLibraryCardTable(body: CreateTemplateLibraryCardTableBody): CardTable {
        val info = OperationInfo(
            service = "Templates",
            operation = "CreateTemplateLibraryCardTable",
            resourceType = "template_library_card_table",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/template_library/card_tables.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardTable>(body)
        }
    }

    /**
     * Start copying a to-do list or card table template into a project
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
                body.destinationProjectId?.let { put("destination_project_id", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.destinationParentId?.let { put("destination_parent_id", kotlinx.serialization.json.JsonPrimitive(it)) }
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
     * Get the account's to-do list templates
     */
    suspend fun getLibraryTodolists(): TemplateLibraryTodolists {
        val info = OperationInfo(
            service = "Templates",
            operation = "GetTemplateLibraryTodolists",
            resourceType = "template_library_todolist",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/template_library/todolists.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<TemplateLibraryTodolists>(body)
        }
    }

    /**
     * Create an empty to-do list template
     * @param body Request body
     */
    suspend fun createLibraryTodolist(body: CreateTemplateLibraryTodolistBody): Todolist {
        val info = OperationInfo(
            service = "Templates",
            operation = "CreateTemplateLibraryTodolist",
            resourceType = "template_library_todolist",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/template_library/todolists.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("name", kotlinx.serialization.json.JsonPrimitive(body.name))
                body.description?.let { put("description", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Todolist>(body)
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
