package com.basecamp.sdk.conformance

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonPrimitive
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue
import kotlin.test.fail

/**
 * Decoder-map coverage and shape-binding for the wire-replay runner.
 *
 * `ReplayRunner.coverageGate()` already asserts coverage, but it only runs
 * during a live canary — and the scheduled canary skips whenever its secrets
 * are unconfigured. That is how this map sat twenty operations behind the live
 * fixture with CI fully green (#553). These run on every `:conformance:test`.
 *
 * `scripts/check-replay-decoder-parity` makes the same coverage assertion
 * statically across all five dispatch tables. This file adds what a text scan
 * cannot: that each registered decoder is bound to the response SHAPE the SDK
 * actually decodes, so a `Recording.serializer()` where the service does
 * `decodeFromString<List<Recording>>` is caught rather than counted as covered.
 */
class ReplayDecodersTest {
    // Gradle runs tests with workingDir = rootProject.projectDir (kotlin/),
    // matching the `run` and `runReplay` tasks — so repo paths resolve the same
    // way Main.kt and ReplayRunner.main() resolve them.
    private val liveFixture = File("../conformance/tests/live-my-surface.json")

    private fun liveOperations(): Set<String> {
        val all = Json.parseToJsonElement(liveFixture.readText()).jsonArray
        val ops = all.mapNotNull { el ->
            val obj = el as? JsonObject ?: return@mapNotNull null
            if (obj["mode"]?.jsonPrimitive?.content != "live") null
            else obj["operation"]?.jsonPrimitive?.content
        }.toSet()
        // Fail closed: an empty set would make every assertion below vacuous.
        assertTrue(ops.isNotEmpty(), "live fixture declared no live operations — the reader is broken")
        return ops
    }

    @Test
    fun `every live fixture operation has a decoder`() {
        val missing = liveOperations().filterNot { it in decoders }.sorted()
        assertEquals(emptyList(), missing, "live operations with no decoder in ReplayRunner.kt")
    }

    @Test
    fun `no decoder is registered for an unknown operation`() {
        val live = liveOperations()
        val extra = decoders.keys.filterNot { it in live }.sorted()
        assertEquals(emptyList(), extra, "decoders registered for non-live operations")
    }

    /**
     * The four `My*` operations decode as bare `JsonElement` — the SDK's own
     * return type for them — so they accept any JSON value and have no shape to
     * bind. Listed by name so the shape rules below stay strict for everyone
     * else instead of being weakened to accommodate four exceptions.
     */
    private val untypedByDesign = setOf(
        "GetMyAssignments",
        "GetMyCompletedAssignments",
        "GetMyDueAssignments",
        "GetMyNotifications",
    )

    /**
     * The single-resource GETs. Everything else on the live surface is a
     * collection: the generated service ends in `decodeFromString<List<T>>`,
     * so the replay decoder must be `ListSerializer(T.serializer())`.
     */
    private val objectShaped = setOf("GetProject", "GetMyProfile", "GetTodoset", "GetCalendar")

    /**
     * kotlinx rejects a JSON array for a class serializer unconditionally, and
     * accepts `[]` for a list serializer (no elements to validate), so
     * "`[]` decodes" is an exact test of list-vs-object binding.
     *
     * This is what neither the runtime coverage gate nor the static parity
     * check can see: registering `Recording.serializer()` where the service
     * does `decodeFromString<List<Recording>>` compiles, populates the map, and
     * counts as covered — while testing a decode the SDK never performs.
     */
    @Test
    fun `collection operations decode an empty array`() {
        val wrong = decoders.keys
            .filterNot { it in untypedByDesign || it in objectShaped }
            .filterNot { op -> runCatching { decoders.getValue(op)("[]") }.isSuccess }
            .sorted()
        assertEquals(emptyList(), wrong, "collection decoders bound to the element type instead of List<T>")
    }

    @Test
    fun `collection operations reject a JSON object`() {
        val wrong = decoders.keys
            .filterNot { it in untypedByDesign || it in objectShaped }
            .filter { op -> runCatching { decoders.getValue(op)("{}") }.isSuccess }
            .sorted()
        assertEquals(emptyList(), wrong, "collection decoders that accept an object body are not list-bound")
    }

    @Test
    fun `single-resource operations reject a JSON array`() {
        for (op in objectShaped) {
            val decode = decoders[op] ?: fail("$op is not registered")
            assertTrue(
                runCatching { decode("[]") }.isFailure,
                "$op decodes an array; it should be bound to its object model",
            )
        }
    }

    /** The excluded four really are shape-free, not just currently passing. */
    @Test
    fun `the untyped My operations accept both shapes`() {
        for (op in untypedByDesign) {
            val decode = decoders[op] ?: fail("$op is not registered")
            assertTrue(runCatching { decode("[]") }.isSuccess, "$op should decode an array")
            assertTrue(runCatching { decode("{}") }.isSuccess, "$op should decode an object")
        }
    }

    /** Both exception sets must name real registrations, or they mask drift. */
    @Test
    fun `the exception sets name registered operations`() {
        for (op in untypedByDesign + objectShaped) {
            assertTrue(op in decoders, "$op is excluded from a shape rule but is not registered")
        }
    }
}
