package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for EventFeed operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class EventFeedService(client: AccountClient) : BaseService(client) {

    /**
     * Poll the account event feed for events after a position (oldest first, strict event-id order, up to 100 per page).
     * @param options Optional query parameters and pagination control
     */
    suspend fun pollEvents(options: PollEventsOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "EventFeed",
            operation = "PollEvents",
            resourceType = "feed_event",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "since" to options?.since,
            "position" to options?.position,
            "types" to options?.types,
            "buckets" to options?.buckets,
            "creators" to options?.creators,
            "performers" to options?.performers,
            "exclude_performers" to options?.excludePerformers,
            "actor_types" to options?.actorTypes,
        )
        return request(info, {
            httpGet("/events.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Mint a short-lived stream ticket and the exact WebSocket URL to open a live event stream with.
     */
    suspend fun createStreamTicket(): JsonElement {
        val info = OperationInfo(
            service = "EventFeed",
            operation = "CreateStreamTicket",
            resourceType = "stream_ticket",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/events/stream_ticket.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Poll the authenticated agent's inbox for the items that addressed it (oldest first, strict item-id order); people receive 403.
     * @param options Optional query parameters and pagination control
     */
    suspend fun pollInbox(options: PollInboxOptions? = null): JsonElement {
        val info = OperationInfo(
            service = "EventFeed",
            operation = "PollInbox",
            resourceType = "inbox_item",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "since" to options?.since,
            "position" to options?.position,
            "reasons" to options?.reasons,
            "types" to options?.types,
            "buckets" to options?.buckets,
        )
        return request(info, {
            httpGet("/inbox.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }
}
