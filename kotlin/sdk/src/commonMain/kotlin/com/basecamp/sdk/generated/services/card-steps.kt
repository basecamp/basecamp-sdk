package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for CardSteps operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CardStepsService(client: AccountClient) : BaseService(client) {

    /**
     * Reposition a step within a card
     * @param cardId The card ID
     * @param body Request body
     */
    suspend fun reposition(cardId: Long, body: RepositionCardStepBody): Unit {
        val info = OperationInfo(
            service = "CardSteps",
            operation = "RepositionCardStep",
            resourceType = "card_step",
            isMutation = true,
            projectId = null,
            resourceId = cardId,
        )
        request(info, {
            httpPost("/card_tables/cards/${cardId}/positions.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("source_id", kotlinx.serialization.json.JsonPrimitive(body.sourceId))
                put("position", kotlinx.serialization.json.JsonPrimitive(body.position))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Create a step on a card
     * @param cardId The card ID
     * @param body Request body
     */
    suspend fun create(cardId: Long, body: CreateCardStepBody): CardStep {
        val info = OperationInfo(
            service = "CardSteps",
            operation = "CreateCardStep",
            resourceType = "card_step",
            isMutation = true,
            projectId = null,
            resourceId = cardId,
        )
        return request(info, {
            httpPost("/card_tables/cards/${cardId}/steps.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("title", kotlinx.serialization.json.JsonPrimitive(body.title))
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.assignees?.let { put("assignees", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardStep>(body)
        }
    }

    /**
     * Update an existing step
     * @param stepId The step ID
     * @param body Request body
     */
    suspend fun update(stepId: Long, body: UpdateCardStepBody): CardStep {
        val info = OperationInfo(
            service = "CardSteps",
            operation = "UpdateCardStep",
            resourceType = "card_step",
            isMutation = true,
            projectId = null,
            resourceId = stepId,
        )
        return request(info, {
            httpPut("/card_tables/steps/${stepId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.dueOn?.let { put("due_on", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.assignees?.let { put("assignees", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardStep>(body)
        }
    }

    /**
     * Set card step completion status (PUT with completion: "on" to complete, "" to uncomplete)
     * @param stepId The step ID
     * @param body Request body
     */
    suspend fun setCompletion(stepId: Long, body: SetCardStepCompletionBody): CardStep {
        val info = OperationInfo(
            service = "CardSteps",
            operation = "SetCardStepCompletion",
            resourceType = "card_step_completion",
            isMutation = true,
            projectId = null,
            resourceId = stepId,
        )
        return request(info, {
            httpPut("/card_tables/steps/${stepId}/completions.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("completion", kotlinx.serialization.json.JsonPrimitive(body.completion))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CardStep>(body)
        }
    }
}
