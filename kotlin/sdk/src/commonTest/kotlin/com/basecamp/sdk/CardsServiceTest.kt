package com.basecamp.sdk

import com.basecamp.sdk.generated.cards
import com.basecamp.sdk.generated.services.UpdateCardBody
import io.ktor.client.engine.mock.*
import io.ktor.client.request.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * BC3 builds a card's update params as `{ due_on: nil }.merge(card_params)`
 * (`kanban/cards_controller.rb`), so any update whose body omits `due_on`
 * erases the card's due date. `update` reads first and resends it;
 * `updateVerbatim` is the raw single PUT.
 */
class CardsServiceTest {

    private val cardJson = """{
        "id": 42,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "Ship it",
        "inherits_status": true,
        "type": "Kanban::Card",
        "url": "https://3.basecampapi.com/12345/card_tables/cards/42",
        "app_url": "https://3.basecamp.com/12345/card_tables/cards/42",
        "due_on": "2024-02-01",
        "description_attachments": [],
        "parent": {
            "id": 2,
            "title": "In Progress",
            "type": "Kanban::Column",
            "url": "https://3.basecampapi.com/12345/card_tables/columns/2.json",
            "app_url": "https://3.basecamp.com/12345/card_tables/columns/2"
        },
        "bucket": { "id": 1, "name": "The Leto Laptop", "type": "Project" },
        "creator": {
            "id": 3,
            "name": "Victor Cooper",
            "email_address": "victor@honchodesign.com",
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z"
        }
    }"""

    private class Exchange {
        val methods = mutableListOf<String>()
        var lastPutBody: String? = null
    }

    private fun clientFor(exchange: Exchange): AccountClient {
        val engine = MockEngine { request ->
            exchange.methods.add(request.method.value)
            if (request.method == HttpMethod.Put) {
                exchange.lastPutBody = (request.body as? io.ktor.http.content.TextContent)?.text
            }
            respond(
                content = cardJson,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }.forAccount("12345")
    }

    @Test
    fun updatePreservesDueOnWhenUnaddressed() = runTest {
        val exchange = Exchange()
        clientFor(exchange).cards.update(42, title = "Renamed")

        assertEquals(listOf("GET", "PUT"), exchange.methods, "the composite must read before writing")
        val body = exchange.lastPutBody!!
        assertContains(body, "\"due_on\":\"2024-02-01\"")
        assertContains(body, "\"title\":\"Renamed\"")
        // Never echoed back: BC3 filters ids through reachable_people.
        assertFalse(body.contains("assignee_ids"), "assignees must stay absent when unaddressed")
    }

    @Test
    fun updateExplicitClearOmitsDueOnAndSkipsTheRead() = runTest {
        val exchange = Exchange()
        clientFor(exchange).cards.update(42, dueOn = "")

        assertEquals(listOf("PUT"), exchange.methods, "an explicit clear needs no read")
        // Clearing is omission, never a literal null (SPEC section 18).
        assertFalse(exchange.lastPutBody!!.contains("due_on"))
    }

    @Test
    fun updateExplicitDateSkipsTheRead() = runTest {
        val exchange = Exchange()
        clientFor(exchange).cards.update(42, dueOn = "2026-09-01")

        assertEquals(listOf("PUT"), exchange.methods)
        assertContains(exchange.lastPutBody!!, "\"due_on\":\"2026-09-01\"")
    }

    @Test
    fun updateSendsExplicitEmptyContentAndAssignees() = runTest {
        val exchange = Exchange()
        clientFor(exchange).cards.update(42, content = "", assigneeIds = emptyList<Long>(), dueOn = "")

        val body = exchange.lastPutBody!!
        assertContains(body, "\"content\":\"\"")
        assertContains(body, "\"assignee_ids\":[]")
    }

    @Test
    fun updateVerbatimSendsOnePutWithNoRead() = runTest {
        val exchange = Exchange()
        clientFor(exchange).cards.updateVerbatim(42, UpdateCardBody(title = "Renamed"))

        assertEquals(listOf("PUT"), exchange.methods, "verbatim must not read before writing")
        assertFalse(exchange.lastPutBody!!.contains("due_on"))
    }
}
