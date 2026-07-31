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

    private suspend fun raise422(body: String): BasecampException.Validation {
        val client = mockClient(HttpStatusCode.UnprocessableEntity, body)
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
}
