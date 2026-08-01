package com.basecamp.sdk

import com.basecamp.sdk.generated.projects
import com.basecamp.sdk.generated.services.CreateProjectBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Field-keyed 422 bodies ({"errors": {"field": ["msg", ...]}} — the Rails
 * RecordInvalid rendering) must surface both as a flattened message and as the
 * structured [BasecampException.Validation.fieldErrors] slot.
 */
class FieldKeyed422Test {

    private fun mockClient(status: HttpStatusCode, body: String): BasecampClient {
        val engine = MockEngine {
            respond(
                content = body,
                status = status,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private suspend fun raise422(body: String): BasecampException.Validation =
        raiseValidation(HttpStatusCode.UnprocessableEntity, body)

    private suspend fun raiseValidation(
        status: HttpStatusCode,
        body: String,
    ): BasecampException.Validation {
        val client = mockClient(status, body)
        try {
            return assertFailsWith<BasecampException.Validation> {
                client.forAccount("12345").projects.create(CreateProjectBody(name = ""))
            }
        } finally {
            client.close()
        }
    }

    @Test
    fun flattensFieldErrorsIntoMessage() = runTest {
        val e = raise422("""{"errors": {"color": ["is not a valid color"]}}""")
        assertEquals("color: is not a valid color", e.message)
        assertEquals(mapOf("color" to listOf("is not a valid color")), e.fieldErrors)
    }

    @Test
    fun sortsFieldsAndJoinsMultiMessageFields() = runTest {
        val e = raise422(
            """{"errors": {"name": ["can't be blank", "is too short"], "color": ["is not a valid color"]}}"""
        )
        assertEquals("color: is not a valid color, name: can't be blank; is too short", e.message)
        assertEquals(
            mapOf(
                "color" to listOf("is not a valid color"),
                "name" to listOf("can't be blank", "is too short"),
            ),
            e.fieldErrors,
        )
    }

    @Test
    fun appendsAfterTopLevelErrorMessage() = runTest {
        val e = raise422(
            """{"error": "Validation failed", "errors": {"color": ["is not a valid color"]}}"""
        )
        assertEquals("Validation failed (color: is not a valid color)", e.message)
    }

    @Test
    fun skipsMalformedEntries() = runTest {
        val e = raise422(
            """{"errors": {"color": "not an array", "name": ["can't be blank"], "empty": [], "mixed": [42, "is invalid"]}}"""
        )
        assertEquals("mixed: is invalid, name: can't be blank", e.message)
        assertEquals(
            mapOf("mixed" to listOf("is invalid"), "name" to listOf("can't be blank")),
            e.fieldErrors,
        )
    }

    @Test
    fun unusableErrorsShapeFallsBack() = runTest {
        for (errors in listOf("""{"color": "not an array"}""", "[]", "\"nope\"", "{}")) {
            val e = raise422("""{"errors": $errors}""")
            assertNull(e.fieldErrors, "expected null fieldErrors for $errors")
            assertFalse(
                (e.message ?: "").contains("not a valid"),
                "unexpected flattening for $errors",
            )
        }
    }

    @Test
    fun truncatesAfterFlatteningButKeepsRawSlot() = runTest {
        val long = "x".repeat(600)
        val e = raise422("""{"errors": {"color": ["$long"]}}""")
        assertEquals(500, e.message?.length)
        assertTrue(e.message!!.startsWith("color: xxx"))
        assertTrue(e.message!!.endsWith("..."))
        assertEquals(mapOf("color" to listOf(long)), e.fieldErrors)
    }

    @Test
    fun survivesNonStringErrorSibling() = runTest {
        val e = raise422(
            """{"error": {"base": 1}, "error_description": 42, "errors": {"color": ["is not a valid color"]}}"""
        )
        assertEquals("color: is not a valid color", e.message)
        assertEquals(mapOf("color" to listOf("is not a valid color")), e.fieldErrors)
        assertNull(e.hint)
    }

    @Test
    fun appendsAfterMessageKeyFallback() = runTest {
        val e = raise422(
            """{"message": "Validation failed", "errors": {"color": ["is not a valid color"]}}"""
        )
        assertEquals("Validation failed (color: is not a valid color)", e.message)
        assertEquals(mapOf("color" to listOf("is not a valid color")), e.fieldErrors)
    }

    @Test
    fun errorKeyWinsOverMessageKey() = runTest {
        val e = raise422("""{"error": "from error", "message": "from message"}""")
        assertEquals("from error", e.message)
    }

    @Test
    fun plainErrorBodyUnchanged() = runTest {
        val e = raise422("""{"error": "Name can't be blank"}""")
        assertEquals("Name can't be blank", e.message)
        assertNull(e.fieldErrors)
    }

    @Test
    fun notExtractedOutsideValidationStatuses() = runTest {
        val client = mockClient(
            HttpStatusCode.Forbidden,
            """{"errors": {"color": ["is not a valid color"]}}""",
        )
        try {
            val e = assertFailsWith<BasecampException.Forbidden> {
                client.forAccount("12345").projects.create(CreateProjectBody(name = ""))
            }
            assertFalse((e.message ?: "").contains("is not a valid color"))
        } finally {
            client.close()
        }
    }

    // SPEC §6 step 2: webhooks_controller and chats/integrations_controller
    // render `json: @webhook.errors` at 400, lineup markers at 422 — the field
    // map arrives as the whole body, with no "errors" wrapper.

    @Test
    fun flattensBareFieldMapAt400() = runTest {
        val e = raiseValidation(HttpStatusCode.BadRequest, """{"payload_url": ["is not a valid URL"]}""")
        assertEquals("payload_url: is not a valid URL", e.message)
        assertEquals(mapOf("payload_url" to listOf("is not a valid URL")), e.fieldErrors)
    }

    @Test
    fun sortsAndJoinsBareFieldMap() = runTest {
        val e = raiseValidation(
            HttpStatusCode.BadRequest,
            """{"types": ["is invalid"], "payload_url": ["is not a valid URL", "is too long"]}""",
        )
        assertEquals("payload_url: is not a valid URL; is too long, types: is invalid", e.message)
        assertEquals(
            mapOf(
                "payload_url" to listOf("is not a valid URL", "is too long"),
                "types" to listOf("is invalid"),
            ),
            e.fieldErrors,
        )
    }

    @Test
    fun flattensBareFieldMapAt422() = runTest {
        val e = raiseValidation(HttpStatusCode.UnprocessableEntity, """{"name": ["can't be blank"]}""")
        assertEquals("name: can't be blank", e.message)
        assertEquals(mapOf("name" to listOf("can't be blank")), e.fieldErrors)
    }

    // All-or-nothing by design: with no "errors" key to declare intent, shape is
    // the only signal that a body is a field map.
    @Test
    fun bareFieldMapGateRejectsNonConformingBodies() = runTest {
        val bodies = listOf(
            """{"id": 1}""",
            """{"color": ["is invalid"], "count": 3}""",
            """{"color": []}""",
            """{"color": ["", "is invalid"]}""",
            """{"color": ["is invalid", 42]}""",
            """{"color": [null]}""",
            "{}",
            "[1, 2]",
            "\"nope\"",
        )
        for (body in bodies) {
            val e = raiseValidation(HttpStatusCode.BadRequest, body)
            assertNull(e.fieldErrors, "expected null fieldErrors for $body")
            assertFalse((e.message ?: "").contains("is invalid"), "expected fallback message for $body")
        }
    }

    @Test
    fun bareFieldMapYieldsToReservedKeys() = runTest {
        val e = raiseValidation(
            HttpStatusCode.BadRequest,
            """{"error": "Webhook is invalid", "payload_url": ["is bad"]}""",
        )
        assertEquals("Webhook is invalid", e.message)
        assertNull(e.fieldErrors)

        val withMessage = raiseValidation(
            HttpStatusCode.BadRequest,
            """{"message": "Webhook is invalid", "payload_url": ["is bad"]}""",
        )
        assertEquals("Webhook is invalid", withMessage.message)
        assertNull(withMessage.fieldErrors)

        val withEmptyErrors = raiseValidation(
            HttpStatusCode.BadRequest,
            """{"errors": {}, "payload_url": ["is bad"]}""",
        )
        assertNull(withEmptyErrors.fieldErrors)
    }

    // Only "errors" is reserved by name. A record whose validated attribute is
    // called "message" or "error" still gets its field map recognized: the flat
    // shape carries those keys as strings, which the gate rejects on shape alone.
    @Test
    fun bareFieldMapAllowsReservedFieldNames() = runTest {
        val named = raiseValidation(HttpStatusCode.BadRequest, """{"message": ["can't be blank"]}""")
        assertEquals("message: can't be blank", named.message)
        assertEquals(mapOf("message" to listOf("can't be blank")), named.fieldErrors)

        val alongside = raiseValidation(
            HttpStatusCode.BadRequest,
            """{"error": ["is invalid"], "name": ["can't be blank"]}""",
        )
        assertEquals("error: is invalid, name: can't be blank", alongside.message)
        assertEquals(
            mapOf("error" to listOf("is invalid"), "name" to listOf("can't be blank")),
            alongside.fieldErrors,
        )
    }

    @Test
    fun bareFieldMapNotExtractedOutsideValidationStatuses() = runTest {
        val client = mockClient(HttpStatusCode.Forbidden, """{"payload_url": ["is not a valid URL"]}""")
        try {
            val e = assertFailsWith<BasecampException.Forbidden> {
                client.forAccount("12345").projects.create(CreateProjectBody(name = ""))
            }
            assertFalse((e.message ?: "").contains("is not a valid URL"))
        } finally {
            client.close()
        }
    }
}
