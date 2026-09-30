package com.basecamp.sdk.generator

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue
import kotlinx.serialization.json.*

/**
 * Pins the one strict test for a deprecated alias component (#955): a component
 * that is nothing but a `$ref` whose `deprecated` is the JSON boolean true.
 *
 * `booleanOrNull` alone is not that test: it reads the STRING "true" as true,
 * so `"deprecated": "true"` would have made an alias that Go, TypeScript, Ruby,
 * Python, Swift and Rust all decline. The refusals for aliases the generator
 * cannot place run against the real binary in scripts/test-compiled-generator-refusal.
 */
class DeprecatedAliasTest {

    private fun alias(deprecated: JsonElement, extra: Map<String, JsonElement> = emptyMap()) = buildJsonObject {
        put("\$ref", "#/components/schemas/Subtask")
        put("deprecated", deprecated)
        extra.forEach { (key, value) -> put(key, value) }
    }

    private val api = OpenApiParser(
        buildJsonObject {
            putJsonObject("components") {
                putJsonObject("schemas") {
                    putJsonObject("Subtask") {
                        put("type", "object")
                        putJsonObject("properties") { putJsonObject("id") { put("type", "integer") } }
                    }
                    put("CardStep", alias(JsonPrimitive(true), mapOf("x-deprecated-reason" to JsonPrimitive("renamed"))))
                    put("StringTrue", alias(JsonPrimitive("true")))
                    put("NumberOne", alias(JsonPrimitive(1)))
                    put("StringFalse", alias(JsonPrimitive("false")))
                    put("WithType", alias(JsonPrimitive(true), mapOf("type" to JsonPrimitive("object"))))
                }
            }
            putJsonObject("paths") {}
        }
    )

    @Test
    fun onlyTheJsonBooleanTrueMarksAnAlias() {
        assertEquals(listOf("CardStep" to "Subtask"), api.deprecatedAliasSchemas())
        assertTrue(api.isDeprecatedAlias(api.getSchema("CardStep")))
        for (name in listOf("StringTrue", "NumberOne", "StringFalse")) {
            assertFalse(api.isDeprecatedAlias(api.getSchema(name)), "$name must not be a deprecated alias")
        }
    }

    @Test
    fun aComponentWithOtherSchemaKeywordsIsNotAnAlias() {
        assertFalse(api.isDeprecatedAlias(api.getSchema("WithType")))
    }

    @Test
    fun anObjectModelHasProperties() {
        assertTrue(api.isObjectModel(api.getSchema("Subtask")))
        assertFalse(api.isObjectModel(api.getSchema("CardStep")))
        assertFalse(api.isObjectModel(null))
    }
}
