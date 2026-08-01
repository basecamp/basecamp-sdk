package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for MyNotes operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class MyNotesService(client: AccountClient) : BaseService(client) {

    /**
     * Get the authenticated user's note — a per-person notebook singleton at
     */
    suspend fun getMyNote(): MyNote {
        val info = OperationInfo(
            service = "MyNotes",
            operation = "GetMyNote",
            resourceType = "my_note",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/my/notes.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<MyNote>(body)
        }
    }

    /**
     * Replace the note's content, recording a new revision server-side.
     * @param body Request body
     */
    suspend fun updateMyNote(body: UpdateMyNoteBody): MyNote {
        val info = OperationInfo(
            service = "MyNotes",
            operation = "UpdateMyNote",
            resourceType = "my_note",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPut("/my/notes.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("note", body.note)
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<MyNote>(body)
        }
    }
}
