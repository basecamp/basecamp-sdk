package com.basecamp.sdk

import com.basecamp.sdk.generated.models.CardStep
import com.basecamp.sdk.generated.models.Subtask
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * `CardStep` is the deprecated former name of `Subtask`: a typealias, so the
 * old spelling names the same class, decodes through the same serializer, and
 * passes wherever a `Subtask` is taken.
 */
@Suppress("DEPRECATION")
class SubtaskRenameTest {
    private val json = Json { ignoreUnknownKeys = true }

    private val wire = """
        {
          "id": 100,
          "status": "active",
          "visible_to_clients": false,
          "created_at": "2026-07-02T00:23:00Z",
          "updated_at": "2026-07-02T00:23:00Z",
          "title": "Hero shot on the desk",
          "inherits_status": true,
          "type": "Kanban::Step",
          "url": "https://3.basecampapi.com/999/buckets/1/subtasks/100.json",
          "app_url": "https://3.basecamp.com/999/buckets/1/todos/200#__recording_100",
          "parent": {"id": 200, "title": "Shot list", "type": "Todo", "url": "u", "app_url": "a"},
          "bucket": {"id": 1, "name": "The Leto Laptop", "type": "Project"},
          "creator": {"id": 7, "name": "Matt Donahue"}
        }
    """.trimIndent()

    private fun idOf(subtask: Subtask): Long = subtask.id

    @Test
    fun cardStepDecodesAsTheSameSubtask() {
        val step: CardStep = json.decodeFromString<CardStep>(wire)
        assertEquals("Kanban::Step", step.type)
        assertEquals(100L, idOf(step))
        assertEquals(json.decodeFromString<Subtask>(wire), step)
        assertEquals(Subtask::class, CardStep::class)
    }
}
