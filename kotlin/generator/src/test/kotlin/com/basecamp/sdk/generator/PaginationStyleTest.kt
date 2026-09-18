package com.basecamp.sdk.generator

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlinx.serialization.json.*

/**
 * Pins Kotlin's refusal of a pagination style it does not implement.
 *
 * `hasPagination` keys off `style == "link"`, which makes an unrecognised value
 * MORE dangerous than the old presence check was: read as "not paginated" it
 * ships a method that never walks, silently, with nothing downstream to catch
 * it. So every unimplemented spelling has to fail generation by name.
 *
 * The malformed-trait cases below are the ones that actually bite. Gating the
 * refusal on `as? JsonObject` answers Kotlin null for a present-but-non-object
 * trait — a bare `"x-basecamp-pagination": "page"` — and skips the refusal
 * entirely, which is the exact silent pass the refusal exists to close. Swift
 * had the same hole through `as? [String: Any]` and closed it by testing
 * presence before the cast; these pin that Kotlin does the same.
 */
class PaginationStyleTest {

    private val api = OpenApiParser(
        buildJsonObject {
            putJsonObject("components") { putJsonObject("schemas") {} }
            putJsonObject("paths") {}
        }
    )

    private fun parse(pagination: JsonElement?): ParsedOperation {
        val operation = buildJsonObject {
            put("operationId", "ListWidgets")
            if (pagination != null) put("x-basecamp-pagination", pagination)
        }
        return OperationParser(api, listOf("get")).parseOperation("/widgets", "get", operation)
    }

    private fun styleFailure(pagination: JsonElement?): String =
        assertFailsWith<IllegalArgumentException> { parse(pagination) }.message ?: ""

    // A trait with no readable style refuses by naming `null` as the style.
    //
    // Asserting the MESSAGE, not just the exception type, is the whole point of
    // this helper. `jsonPrimitive` throws IllegalArgumentException too, so a
    // test that checked only the type would stay green if the parser went back
    // to the cast that this file exists to rule out -- it would be catching the
    // cast error and calling it a refusal.
    private fun assertRefusedWithNoReadableStyle(pagination: JsonElement?) {
        assertEquals(
            "ListWidgets: unsupported pagination style null (expected \"link\" or \"cursor\")",
            styleFailure(pagination)
        )
    }

    @Test
    fun `the link style paginates and keeps its key`() {
        val parsed = parse(
            buildJsonObject {
                put("style", "link")
                put("key", "widgets")
            }
        )

        assertEquals(true, parsed.hasPagination)
        assertEquals("widgets", parsed.paginationKey)
    }

    @Test
    fun `the cursor style does not paginate and withholds its key`() {
        // The key is withheld at the source rather than gated at each consumer:
        // findUnderlyingEntitySchema unwraps an envelope whenever the key is set
        // and never consults hasPagination, so a cursor operation carrying a key
        // would be typed as the item under it instead of the envelope the wire
        // actually sends — wrong public return type, and it would not decode.
        val parsed = parse(
            buildJsonObject {
                put("style", "cursor")
                put("key", "events")
            }
        )

        assertEquals(false, parsed.hasPagination)
        assertNull(parsed.paginationKey)
    }

    @Test
    fun `an absent trait is unpaginated`() {
        val parsed = parse(null)

        assertEquals(false, parsed.hasPagination)
        assertNull(parsed.paginationKey)
    }

    @Test
    fun `a literal null trait is unpaginated, as it is in the other five generators`() {
        // `?.jsonObject` throws on JsonNull, which is not Kotlin null. Kotlin was
        // once the only generator that crashed here where the rest read absent.
        val parsed = parse(JsonNull)

        assertEquals(false, parsed.hasPagination)
        assertNull(parsed.paginationKey)
    }

    @Test
    fun `the retired page style is refused by name`() {
        assertEquals(
            "ListWidgets: unsupported pagination style page (expected \"link\" or \"cursor\")",
            styleFailure(buildJsonObject { put("style", "page") })
        )
    }

    @Test
    fun `a typo is refused rather than read as unpaginated`() {
        assertEquals(
            "ListWidgets: unsupported pagination style linkk (expected \"link\" or \"cursor\")",
            styleFailure(buildJsonObject { put("style", "linkk") })
        )
    }

    @Test
    fun `the style match is case sensitive`() {
        assertEquals(
            "ListWidgets: unsupported pagination style Link (expected \"link\" or \"cursor\")",
            styleFailure(buildJsonObject { put("style", "Link") })
        )
    }

    @Test
    fun `a declared trait with no style at all is refused`() {
        assertEquals(
            "ListWidgets: unsupported pagination style null (expected \"link\" or \"cursor\")",
            styleFailure(buildJsonObject { put("maxPageSize", 50) })
        )
    }

    @Test
    fun `a bare string trait is refused, not read as unpaginated`() {
        // The hole: `as? JsonObject` answers null here, so a presence check
        // written against the CAST would exempt this and ship an unpaginated
        // method for a spec that plainly said "page".
        assertRefusedWithNoReadableStyle(JsonPrimitive("page"))
    }

    @Test
    fun `an array trait is refused`() {
        assertRefusedWithNoReadableStyle(buildJsonArray { add("link") })
    }

    @Test
    fun `a boolean trait is refused`() {
        assertRefusedWithNoReadableStyle(JsonPrimitive(false))
    }

    @Test
    fun `a numeric trait is refused`() {
        assertRefusedWithNoReadableStyle(JsonPrimitive(0))
    }

    @Test
    fun `a non-primitive style is refused by name rather than throwing a cast error`() {
        assertRefusedWithNoReadableStyle(buildJsonObject { putJsonObject("style") { put("name", "link") } })
    }
}
