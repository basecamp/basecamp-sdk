package com.basecamp.sdk

import com.basecamp.sdk.generated.services.UpdateTodolistOrGroupBody
import com.basecamp.sdk.generated.todolists
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

class TodolistsServiceTest {

    private val todolistJson = """{
        "id": 42,
        "name": "Launch list",
        "description": "<p>Things to do before launch</p>",
        "description_attachments": [],
        "completed": false,
        "completed_ratio": "0/5",
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z"
    }"""

    @Test
    fun getEmitsTodolistIdAsResourceId() = runTest {
        var capturedInfo: OperationInfo? = null
        val hooks = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                if (info.operation == "GetTodolistOrGroup") capturedInfo = info
            }
        }
        val engine = MockEngine { _ ->
            respond(
                content = todolistJson,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("12345").todolists.get(id = 42)

        assertNotNull(capturedInfo)
        assertEquals(42L, capturedInfo!!.resourceId)

        client.close()
    }

    @Test
    fun updateEmitsTodolistIdAsResourceId() = runTest {
        var capturedInfo: OperationInfo? = null
        val hooks = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                if (info.operation == "UpdateTodolistOrGroup") capturedInfo = info
            }
        }
        val engine = MockEngine { _ ->
            respond(
                content = todolistJson,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("12345").todolists.update(
            id = 42,
            body = UpdateTodolistOrGroupBody(name = "Updated list"),
        )

        assertNotNull(capturedInfo)
        assertEquals(42L, capturedInfo!!.resourceId)

        client.close()
    }
}
