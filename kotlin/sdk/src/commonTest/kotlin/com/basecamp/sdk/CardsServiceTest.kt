package com.basecamp.sdk

import com.basecamp.sdk.generated.cards
import com.basecamp.sdk.generated.services.UpdateCardBody
import io.ktor.client.engine.mock.*
import io.ktor.client.request.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull

/**
 * BC3 (basecamp/bc3#12521) updates a card from the JSON params as they arrive,
 * so an omitted `due_on` leaves the due date UNCHANGED and only an explicitly
 * sent value moves it. Clearing has to be stated: `""` casts to nil server-side.
 *
 * `update` is the named-argument face of that contract — one PUT, carrying the
 * fields the caller addressed — and `updateVerbatim` is the same PUT taking a
 * body object. [CardServer] applies a PUT the way BC3 does, so the tests below
 * pin the effect on the card and not merely the bytes.
 */
class CardsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun cardJson(dueOn: String? = "2024-02-01"): String =
        cardJsonRawDueOn(dueOn?.let { "\"$it\"" } ?: "null")

    /** [cardJson] with `due_on` spliced in as raw JSON, so it can be a non-string. */
    private fun cardJsonRawDueOn(dueOn: String): String = """{
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
        "due_on": $dueOn,
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

    /**
     * The card as the server holds it, updated the way BC3 updates it: the due
     * date moves only when `due_on` is present in the body, and a blank (or an
     * explicit null) clears it.
     */
    private class CardServer(var dueOn: String? = "2024-02-01") {
        fun applyPut(body: String, json: Json) {
            val member = json.parseToJsonElement(body).jsonObject["due_on"]
            dueOn = when {
                member == null -> dueOn
                member is JsonNull -> null
                else -> member.jsonPrimitive.content.takeIf { it.isNotEmpty() }
            }
        }
    }

    private fun clientFor(exchange: Exchange, server: CardServer = CardServer()): AccountClient {
        val engine = MockEngine { request ->
            exchange.methods.add(request.method.value)
            if (request.method == HttpMethod.Put) {
                val text = (request.body as? io.ktor.http.content.TextContent)?.text.orEmpty()
                exchange.lastPutBody = text
                server.applyPut(text, json)
            }
            respond(
                content = cardJson(server.dueOn),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }.forAccount("12345")
    }

    /** The `due_on` member of the PUT that was sent, or null if the key was absent. */
    private fun Exchange.sentDueOn() = json.parseToJsonElement(lastPutBody!!).jsonObject["due_on"]

    @Test
    fun updateLeavesAnUnaddressedDueOnOffTheWire() = runTest {
        val exchange = Exchange()
        val server = CardServer()
        clientFor(exchange, server).cards.update(42, title = "Renamed")

        assertEquals(listOf("PUT"), exchange.methods, "one request, no read-before-write")
        assertContains(exchange.lastPutBody!!, "\"title\":\"Renamed\"")
        assertNull(exchange.sentDueOn(), "an unaddressed due date must not be sent at all")
        assertEquals("2024-02-01", server.dueOn, "and so the card keeps the date it had")
        // Never echoed back: BC3 filters ids through reachable_people.
        assertFalse(
            exchange.lastPutBody!!.contains("assignee_ids"),
            "assignees must stay absent when unaddressed",
        )
    }

    @Test
    fun updateExplicitClearSendsBlankDueOn() = runTest {
        val exchange = Exchange()
        clientFor(exchange).cards.update(42, dueOn = "")

        assertEquals(listOf("PUT"), exchange.methods)
        // The key must be PRESENT and blank. Omitting it is a no-op against
        // BC3; a literal null is unavailable under body compaction (SPEC §18)
        // and would compact away to that same omission.
        val sent = assertNotNull(exchange.sentDueOn(), "an explicit clear must send due_on")
        assertFalse(sent is JsonNull, "the clear is a blank string, never a literal null")
        assertEquals("", sent.jsonPrimitive.content)
        assertContains(exchange.lastPutBody!!, "\"due_on\":\"\"")
    }

    /**
     * The behavioural half: against a server that only touches the due date when
     * the key is present, an explicit clear must actually leave the card with no
     * due date — and a title-only update must leave it alone.
     */
    @Test
    fun explicitClearClearsAndOmissionDoesNot() = runTest {
        val server = CardServer()
        val cards = clientFor(Exchange(), server).cards

        // The model really is presence-aware: a PUT with no due_on changes
        // nothing. Without this, the clear below could pass for free.
        cards.update(42, title = "Renamed")
        assertEquals("2024-02-01", server.dueOn, "an omitted due_on must not clear the date")

        val cleared = cards.update(42, dueOn = "")
        assertNull(server.dueOn, "an explicit clear must land as a clear on the server")
        assertNull(cleared.dueOn, "and the card handed back must carry the cleared date")

        // Nor does a later update resurrect what was cleared.
        cards.update(42, title = "Renamed again")
        assertNull(server.dueOn, "a later unrelated update must not restore a cleared date")
    }

    @Test
    fun updateExplicitDateSetsTheDueDate() = runTest {
        val exchange = Exchange()
        val server = CardServer()
        clientFor(exchange, server).cards.update(42, dueOn = "2026-09-01")

        assertEquals(listOf("PUT"), exchange.methods)
        assertContains(exchange.lastPutBody!!, "\"due_on\":\"2026-09-01\"")
        assertEquals("2026-09-01", server.dueOn)
    }

    @Test
    fun updateSendsExplicitEmptyContentAndAssignees() = runTest {
        val exchange = Exchange()
        clientFor(exchange).cards.update(42, content = "", assigneeIds = emptyList<Long>(), dueOn = "")

        val body = exchange.lastPutBody!!
        assertContains(body, "\"content\":\"\"")
        assertContains(body, "\"assignee_ids\":[]")
        assertContains(body, "\"due_on\":\"\"")
    }

    /**
     * The Cards half of #598, in the only shape it still has.
     *
     * The issue framed this as a read-modify-write hazard: a `due_on` read back
     * off a preservation GET was coerced into `"42"` and PUT forward. #647
     * deleted that GET — `update` is one PUT now, and the three Cards kill cases
     * went with it — so what is left is the response decode. `Card.dueOn` is a
     * `String?`, and under the client-wide `isLenient` a bare-scalar `due_on`
     * coming back from the server decoded into `"42"`/`"false"` and reached the
     * caller as an ordinary String, indistinguishable from a real date. The
     * shared decoder refuses it now, which is what makes the fix reach Cards and
     * not only the Todolists composite that still does a genuine read-back.
     */
    @Test
    fun updateRefusesABareScalarDueOnFromTheWire() = runTest {
        for (malformed in listOf("42", "false")) {
            val methods = mutableListOf<String>()
            val client = testBasecampClient {
                accessToken("test-token")
                engine = MockEngine { request ->
                    methods.add(request.method.value)
                    respond(
                        content = cardJsonRawDueOn(malformed),
                        status = HttpStatusCode.OK,
                        headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                    )
                }
            }

            assertFailsWith<SerializationException>(
                "a bare-scalar due_on ($malformed) must be refused, not rendered as text",
            ) {
                client.forAccount("12345").cards.update(42, title = "Renamed")
            }

            assertEquals(listOf("PUT"), methods, "the refusal is in the decode, so the PUT still happens once")
            client.close()
        }
    }

    @Test
    fun updateVerbatimSendsOnePut() = runTest {
        val exchange = Exchange()
        val server = CardServer()
        clientFor(exchange, server).cards.updateVerbatim(42, UpdateCardBody(title = "Renamed"))

        assertEquals(listOf("PUT"), exchange.methods)
        assertNull(exchange.sentDueOn(), "an unmentioned due_on stays off the wire")
        assertEquals("2024-02-01", server.dueOn)
    }
}
