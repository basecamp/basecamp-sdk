package com.basecamp.sdk

import com.basecamp.sdk.generated.search
import com.basecamp.sdk.generated.services.SearchOptions
import io.ktor.client.engine.mock.*
import io.ktor.client.request.HttpRequestData
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class SearchServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    @Test
    fun searchEncodesArrayFiltersAsBracketedKeys() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(
                content = "[]",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        account.search.search(
            q = "hello",
            options = SearchOptions(
                typeNames = listOf("Message", "Todo"),
                bucketIds = listOf(1L, 2L),
                creatorIds = listOf(7L),
            ),
        )

        // Ktor decodes %5B%5D back to [] in parameter names. Rails' permit(
        // bucket_ids: []) only accepts this bracketed repeated form.
        val params = capturedRequest!!.url.parameters
        assertEquals(listOf("1", "2"), params.getAll("bucket_ids[]"))
        assertEquals(listOf("Message", "Todo"), params.getAll("type_names[]"))
        assertEquals(listOf("7"), params.getAll("creator_ids[]"))
        // The bare and double-bracketed forms must be absent.
        assertNull(params.getAll("bucket_ids"))
        assertNull(params.getAll("bucket_ids[][]"))
        assertEquals("hello", params["q"])

        client.close()
    }

    @Test
    fun searchEncodesFullFilterSurface() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(
                content = "[]",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        account.search.search(
            q = "hello",
            options = SearchOptions(
                typeNames = listOf("Message"),
                bucketIds = listOf(1L, 2L),
                creatorIds = listOf(7L),
                fileType = "Image",
                excludeChat = true,
                since = "last_30_days",
                sort = "recency",
                type = "Message",
                bucketId = 9L,
                creatorId = 3L,
            ),
        )

        val params = capturedRequest!!.url.parameters
        assertEquals(listOf("1", "2"), params.getAll("bucket_ids[]"))
        assertEquals(listOf("Message"), params.getAll("type_names[]"))
        assertEquals(listOf("7"), params.getAll("creator_ids[]"))
        assertEquals("hello", params["q"])
        assertEquals("Image", params["file_type"])
        assertEquals("true", params["exclude_chat"])
        assertEquals("last_30_days", params["since"])
        assertEquals("recency", params["sort"])
        assertEquals("Message", params["type"])
        assertEquals("9", params["bucket_id"])
        assertEquals("3", params["creator_id"])

        client.close()
    }

    @Test
    fun metadataDecodesFilterOptions() = runTest {
        val body = """
            {
              "recording_search_types": [
                { "key": null, "value": "Everything" },
                { "key": "Message", "value": "Messages" }
              ],
              "file_search_types": [
                { "key": null, "value": "All files" },
                { "key": "Image", "value": "Images" }
              ],
              "default_creator_label": "Anyone",
              "default_bucket_label": "All projects",
              "default_circle_label": "All pings",
              "default_file_type_label": "All files",
              "default_type_label": "Everything"
            }
        """.trimIndent()

        val client = mockClient { _ ->
            respond(
                content = body,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val metadata = account.search.metadata().jsonObject

        val recordingTypes = metadata["recording_search_types"]!!.jsonArray
        assertEquals(2, recordingTypes.size)
        // The default "everything" option carries a null key.
        assertTrue(recordingTypes[0].jsonObject["key"] is kotlinx.serialization.json.JsonNull)
        assertEquals("Messages", recordingTypes[1].jsonObject["value"]!!.jsonPrimitive.content)
        assertEquals("Anyone", metadata["default_creator_label"]!!.jsonPrimitive.content)
        assertEquals("Everything", metadata["default_type_label"]!!.jsonPrimitive.content)

        client.close()
    }
}
