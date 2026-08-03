package com.basecamp.sdk

import com.basecamp.sdk.generated.services.CreateTodolistGroupBody
import com.basecamp.sdk.generated.todolistGroups
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * `GET /todolists/{todolistId}/groups.json` and its create sibling.
 *
 * BC3 has no group model: `todolists/groups/{index,show}.json.jbuilder` render
 * `todolists/_todolist.json.jbuilder`, so these operations answer with the same
 * flat `Todolist` shape a list does — including `description` and
 * `description_attachments`, which the pre-#544 group projection modelled away.
 * A group is discriminated structurally, by `group_position_url` standing in
 * for `groups_url`, never by the `type` string (it is `"Todolist"` for both).
 */
class TodolistGroupsServiceTest {

    private val api = "https://3.basecampapi.com/12345/buckets/1"

    /** A group's parent is a Todolist. That, not `type`, is what differs. */
    private fun groupBody(
        id: Long,
        name: String,
        description: String,
    ): String = """{
        "id": $id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "$name",
        "inherits_status": true,
        "type": "Todolist",
        "url": "$api/todolists/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/todolists/$id",
        "bubble_up_url": "$api/recordings/$id/bubble_up.json",
        "parent": {"id": 2, "title": "Hardware", "type": "Todolist",
                   "url": "$api/todolists/2.json",
                   "app_url": "https://3.basecamp.com/12345/buckets/1/todolists/2"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test User"},
        "description": "$description",
        "description_attachments": [],
        "completed": false,
        "completed_ratio": "1/3",
        "position": 1,
        "name": "$name",
        "color": null,
        "group_position_url": "$api/todolists/groups/$id/position.json",
        "todos_url": "$api/todolists/$id/todos.json",
        "comments_app_url": "https://3.basecamp.com/12345/buckets/1/recordings/$id/comments"
    }"""

    private fun mockClient(body: String, totalCount: String? = null): BasecampClient {
        val engine = MockEngine { _ ->
            respond(
                content = body,
                status = HttpStatusCode.OK,
                headers = headers {
                    append(HttpHeaders.ContentType, ContentType.Application.Json.toString())
                    totalCount?.let { append("X-Total-Count", it) }
                },
            )
        }
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    /**
     * The list response is an ARRAY of the flat to-do list shape, and every
     * element carries its own description. Before #544 this operation was typed
     * to a group-only projection that modelled no description at all, so the
     * field was unreachable through the SDK's own model.
     */
    @Test
    fun listDecodesAnArrayOfTheFlatTodolistShape() = runTest {
        val body = "[${groupBody(7, "Phase 1", "<div>Phase one hardware work</div>")}," +
            "${groupBody(8, "Phase 2", "")}]"
        val client = mockClient(body, totalCount = "2")

        val groups = client.forAccount("12345").todolistGroups.list(todolistId = 2)

        assertEquals(2, groups.size)
        assertEquals(2L, groups.meta.totalCount)

        val first = groups[0]
        assertEquals("Phase 1", first.name)
        assertEquals("<div>Phase one hardware work</div>", first.description)
        assertEquals(emptyList(), first.descriptionAttachments)
        assertEquals("Todolist", first.type)
        assertEquals("$api/todolists/groups/7/position.json", first.groupPositionUrl)
        assertNull(first.groupsUrl, "a group's parent is a Todolist, so it has no groups_url")
        assertNull(first.color)

        // "" is a real value, not an absence: BC3's format_api_content renders
        // a blank rich text as "" and never as JSON null.
        assertEquals("", groups[1].description)
        assertEquals("Phase 2", groups[1].name)

        client.close()
    }

    /** Create answers with the same flat shape the list does. */
    @Test
    fun createDecodesIntoTheTodolistShape() = runTest {
        val client = mockClient(groupBody(7, "Phase 1", "<div>Phase one hardware work</div>"))

        val group = client.forAccount("12345").todolistGroups
            .create(todolistId = 2, body = CreateTodolistGroupBody(name = "Phase 1"))

        assertEquals(7L, group.id)
        assertEquals("Phase 1", group.name)
        assertEquals("<div>Phase one hardware work</div>", group.description)
        assertEquals("$api/todolists/groups/7/position.json", group.groupPositionUrl)
        assertNull(group.groupsUrl)

        client.close()
    }
}
