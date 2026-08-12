package com.basecamp.sdk

import com.basecamp.sdk.generated.services.CreateToolBody
import com.basecamp.sdk.generated.tools
import io.ktor.client.engine.mock.*
import io.ktor.client.request.HttpRequestData
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ToolsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    /**
     * bc3's real dock-tool projection.
     *
     * `app/views/api/docks/tools/show.json.jbuilder` is one line — it renders
     * the bare `recordings/recording` partial and adds nothing — so a tool
     * response carries no `name` (unlike Todoset/Questionnaire, whose own
     * recordable partials emit `json.name recording.recordable.name`; tools
     * have no such partial) and no `enabled` at any layer. Both were `@required`
     * until #650, so this helper used to fabricate a body bc3 cannot produce.
     *
     * Conditional keys follow the partial's branches: `subscription_url` only
     * when the recordable is subscribable (Message::Board is not),
     * `position` only when `recording.positioned?`, and `parent` only when
     * `!recording.docked?`. The full shapes live in `spec/fixtures/tools/`;
     * they are inline here because KMP `commonTest` has no filesystem.
     */
    private fun toolJson(id: Long, title: String) = """{
        "id": $id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-05-28T17:23:17.384Z",
        "updated_at": "2026-07-21T00:01:05.529Z",
        "title": "$title",
        "inherits_status": true,
        "type": "Message::Board",
        "url": "https://3.basecampapi.com/12345/buckets/456/message_boards/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/456/message_boards/$id",
        "bookmark_url": "https://3.basecampapi.com/12345/my/bookmarks/BAh7Bkki--7e5f099c.json",
        "position": 3,
        "bucket": { "id": 456, "name": "The Leto Laptop", "type": "Project" },
        "creator": {
            "id": 1049715913,
            "name": "Victor Cooper",
            "personable_type": "User",
            "title": "Chief Strategist",
            "email_address": "victor@honchodesign.com",
            "admin": true,
            "owner": true,
            "client": false,
            "employee": true,
            "time_zone": "America/Chicago",
            "avatar_url": "https://3.basecampapi.com/12345/people/BAhpBMlkkT4=--5fe7b70f/avatar",
            "company": { "id": 1033447817, "name": "Honcho Design" }
        }
    }"""

    /** `GET /dock/tools/1069479832`, matching `spec/fixtures/tools/get.json`. */
    private val chatProjection = """{
        "id": 1069479832,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-05-28T17:23:17.384Z",
        "updated_at": "2026-07-21T00:01:05.529Z",
        "title": "Chat",
        "inherits_status": true,
        "type": "Chat::Transcript",
        "url": "https://3.basecampapi.com/12345/buckets/2085958505/chats/1069479832.json",
        "app_url": "https://3.basecamp.com/12345/buckets/2085958505/chats/1069479832",
        "bookmark_url": "https://3.basecampapi.com/12345/my/bookmarks/BAh7Bkki--7e5f099c.json",
        "subscription_url": "https://3.basecampapi.com/12345/buckets/2085958505/recordings/1069479832/subscription.json",
        "position": 5,
        "bucket": { "id": 2085958505, "name": "The Leto Laptop", "type": "Project" },
        "creator": { "id": 1049715913, "name": "Victor Cooper", "title": "Chief Strategist" }
    }"""

    private fun respondingClient(body: String): BasecampClient = mockClient {
        respond(
            content = body,
            status = HttpStatusCode.OK,
            headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
        )
    }

    @Test
    fun createToolPostsToBucketScopedDockPath() = runTest {
        var capturedRequest: HttpRequestData? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedRequest = request
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = toolJson(800, "Message Board (Copy)"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val tool = account.tools.create(
            bucketId = 456,
            body = CreateToolBody(toolType = "Message::Board", title = "Message Board (Copy)"),
        )

        assertEquals(800L, tool.id)
        assertEquals(HttpMethod.Post, capturedRequest!!.method)
        assertTrue(capturedRequest!!.url.encodedPath.endsWith("/buckets/456/dock/tools.json"))

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Message::Board", bodyJson["tool_type"]!!.jsonPrimitive.content)
        assertEquals("Message Board (Copy)", bodyJson["title"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun createToolOmitsTitleWhenNotProvided() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = toolJson(801, "Message Board"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        account.tools.create(
            bucketId = 456,
            body = CreateToolBody(toolType = "Message::Board"),
        )

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Message::Board", bodyJson["tool_type"]!!.jsonPrimitive.content)
        assertFalse(bodyJson.containsKey("title"))

        client.close()
    }

    // visibleToClients is tri-state: null omits the key, true/false are sent
    // verbatim. An explicit false must reach the wire. Only Chat::Transcript and
    // Kanban::Board honor it; all other tool types ignore it.
    @Test
    fun createToolSendsVisibleToClientsTriState() = runTest {
        var capturedBody: String? = null
        val client = mockClient { request ->
            capturedBody = request.body.toByteArray().decodeToString()
            respond(
                content = toolJson(802, "Campfire"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val account = client.forAccount("12345")

        account.tools.create(bucketId = 456, body = CreateToolBody(toolType = "Chat::Transcript"))
        assertFalse(json.parseToJsonElement(capturedBody!!).jsonObject.containsKey("visible_to_clients"))

        account.tools.create(bucketId = 456, body = CreateToolBody(toolType = "Chat::Transcript", visibleToClients = true))
        val trueObj = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals(true, trueObj["visible_to_clients"]!!.jsonPrimitive.content.toBoolean())

        account.tools.create(bucketId = 456, body = CreateToolBody(toolType = "Chat::Transcript", visibleToClients = false))
        val falseObj = json.parseToJsonElement(capturedBody!!).jsonObject
        assertTrue(falseObj.containsKey("visible_to_clients"))
        assertEquals(false, falseObj["visible_to_clients"]!!.jsonPrimitive.content.toBoolean())

        client.close()
    }

    // The #650 regression. `Tool` is a typed @Serializable data class, so
    // `name` and `enabled` being @required made every real dock-tool response
    // fail to decode with MissingFieldException — bc3 emits neither key on any
    // response, because docks/tools/show.json.jbuilder renders only the bare
    // recordings/recording partial. Both must decode to null, not throw.
    @Test
    fun getToolDecodesAResponseCarryingNeitherNameNorEnabled() = runTest {
        val client = respondingClient(chatProjection)

        val tool = client.forAccount("12345").tools.get(1069479832)

        assertNull(tool.name, "docks/tools/show.json.jbuilder emits no `name`")
        assertNull(tool.enabled, "no layer of the tool projection emits `enabled`")

        assertEquals(1069479832L, tool.id)
        assertEquals("Chat", tool.title)
        assertEquals("Chat::Transcript", tool.type)
        assertEquals("active", tool.status)
        assertEquals(false, tool.visibleToClients)
        assertEquals(true, tool.inheritsStatus)
        assertEquals("Victor Cooper", tool.creator.name)
        // Chat::Transcript is subscribable and a docked tool is positioned, so
        // both conditional keys are present; `parent` is emitted only when
        // !recording.docked?, so a docked tool has none.
        assertEquals(5, tool.position)
        assertTrue(tool.subscriptionUrl!!.endsWith("/recordings/1069479832/subscription.json"))
        assertNull(tool.parent, "a docked tool is docked, so the partial emits no `parent`")

        client.close()
    }

    // Disabling a tool removes it from the dock without deleting it, so
    // `recording.positioned?` is false and `position` is absent entirely —
    // absence of `position`, not `enabled: false`, is the disabled signal. This
    // one is a Vault, which does not override Recordable#subscribable? (default
    // false), so `subscription_url` is absent too.
    @Test
    fun getToolDecodesADisabledToolWithNoPositionAndNoSubscriptionUrl() = runTest {
        val client = respondingClient(
            """{
                "id": 1069479343,
                "status": "active",
                "visible_to_clients": false,
                "created_at": "2026-05-28T17:23:17.021Z",
                "updated_at": "2026-07-19T11:02:44.310Z",
                "title": "Docs & Files",
                "inherits_status": true,
                "type": "Vault",
                "url": "https://3.basecampapi.com/12345/buckets/2085958505/vaults/1069479343.json",
                "app_url": "https://3.basecamp.com/12345/buckets/2085958505/vaults/1069479343",
                "bookmark_url": "https://3.basecampapi.com/12345/my/bookmarks/BAh7Bkki--9a8b7c6d.json",
                "bucket": { "id": 2085958505, "name": "The Leto Laptop", "type": "Project" },
                "creator": { "id": 1049715913, "name": "Victor Cooper" }
            }""",
        )

        val tool = client.forAccount("12345").tools.get(1069479343)

        assertNull(tool.position)
        assertNull(tool.subscriptionUrl)
        assertNull(tool.enabled)
        assertNull(tool.name)
        assertEquals("Vault", tool.type)

        client.close()
    }

    // `parent` is emitted only when !recording.docked?. The dock-tool lookup
    // scopes by recordable TYPE (Recordable::CORE_GROUPS["dock_tools"] includes
    // Vault) rather than by dock membership, so a vault nested inside another
    // vault resolves through GET /dock/tools/:id and does carry a parent.
    @Test
    fun getToolDecodesANestedVaultCarryingAParent() = runTest {
        val client = respondingClient(
            """{
                "id": 1069479562,
                "status": "active",
                "visible_to_clients": false,
                "created_at": "2026-06-02T14:08:51.744Z",
                "updated_at": "2026-07-20T08:19:33.612Z",
                "title": "Contracts",
                "inherits_status": true,
                "type": "Vault",
                "url": "https://3.basecampapi.com/12345/buckets/2085958505/vaults/1069479562.json",
                "app_url": "https://3.basecamp.com/12345/buckets/2085958505/vaults/1069479562",
                "bookmark_url": "https://3.basecampapi.com/12345/my/bookmarks/BAh7Bkki--11223344.json",
                "parent": {
                    "id": 1069479343,
                    "title": "Docs & Files",
                    "type": "Vault",
                    "url": "https://3.basecampapi.com/12345/buckets/2085958505/vaults/1069479343.json",
                    "app_url": "https://3.basecamp.com/12345/buckets/2085958505/vaults/1069479343"
                },
                "bucket": { "id": 2085958505, "name": "The Leto Laptop", "type": "Project" },
                "creator": { "id": 1049715913, "name": "Victor Cooper" }
            }""",
        )

        val tool = client.forAccount("12345").tools.get(1069479562)

        assertEquals(1069479343L, tool.parent!!.id)
        assertEquals("Docs & Files", tool.parent!!.title)
        assertEquals("Vault", tool.parent!!.type)
        assertNull(tool.name)
        assertNull(tool.enabled)

        client.close()
    }
}
