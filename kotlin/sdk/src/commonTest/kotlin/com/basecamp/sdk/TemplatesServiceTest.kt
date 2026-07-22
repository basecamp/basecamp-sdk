package com.basecamp.sdk

import com.basecamp.sdk.generated.services.CreateProjectFromTemplateBody
import com.basecamp.sdk.generated.templates
import io.ktor.client.engine.mock.*
import io.ktor.client.request.HttpRequestData
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class TemplatesServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    @Test
    fun createProjectNestsBodyUnderProjectEnvelope() = runTest {
        var capturedRequest: HttpRequestData? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedRequest = request
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = """{"id": 900, "status": "completed"}""",
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        account.templates.createProject(
            templateId = 456,
            body = CreateProjectFromTemplateBody(
                project = buildJsonObject {
                    put("name", "New Project")
                    put("description", "From template")
                },
            ),
        )

        assertEquals(HttpMethod.Post, capturedRequest!!.method)
        assertTrue(capturedRequest!!.url.encodedPath.endsWith("/templates/456/project_constructions.json"))

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertFalse(bodyJson.containsKey("name"))
        val project = bodyJson["project"]!!.jsonObject
        assertEquals("New Project", project["name"]!!.jsonPrimitive.content)
        assertEquals("From template", project["description"]!!.jsonPrimitive.content)

        client.close()
    }
}
