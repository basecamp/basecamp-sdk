package com.basecamp.sdk

/**
 * Metadata about a paginated list response.
 */
data class ListMeta(
    /** Total number of items across all pages (from X-Total-Count header). */
    val totalCount: Long,
    /**
     * True only when the result is incomplete: items were dropped by the
     * maxItems cap, or more pages remained (a next Link) when collection
     * stopped — at maxItems or at the page safety cap. False means the
     * result is definitely complete.
     */
    val truncated: Boolean,
)

/**
 * Options for controlling pagination behavior.
 */
data class PaginationOptions(
    /**
     * Maximum number of items to return across all pages.
     * When null or 0, all pages are fetched.
     */
    val maxItems: Int? = null,
    /**
     * Selects a single page. A positive [page] returns exactly that page in
     * exactly one request: `Link: rel="next"` is not followed, and
     * [ListMeta.truncated] reports whether a further page existed. When null,
     * 0, or negative, auto-pagination walks the whole collection.
     *
     * Only meaningful for operations whose endpoint honors `?page=`; the few
     * list endpoints that return their whole collection at once never emit a
     * next link, so pinning a page there changes nothing. See SPEC section 8.
     */
    val page: Long? = null,
)

/**
 * A list of results with pagination metadata.
 *
 * Delegates to `List<T>` so it's fully compatible with all collection operations
 * (`.forEach()`, `.map()`, `.size`, indexing, etc.). Additional metadata is
 * accessible via the [meta] property.
 *
 * ```kotlin
 * val todos = account.todos.list(todolistId)
 * println("Showing ${todos.size} of ${todos.meta.totalCount} todos")
 * todos.forEach { println(it.content) }
 * ```
 */
class ListResult<T>(
    private val items: List<T>,
    /** Pagination metadata (total count, truncation status). */
    val meta: ListMeta,
) : List<T> by items {

    override fun toString(): String = "ListResult(size=$size, meta=$meta)"

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is ListResult<*>) return false
        return items == other.items && meta == other.meta
    }

    override fun hashCode(): Int = 31 * items.hashCode() + meta.hashCode()
}

/**
 * Parses the X-Total-Count header value.
 * Returns 0 if the header is missing or invalid.
 */
internal fun parseTotalCount(headers: Map<String, List<String>>): Long {
    val value = headers["X-Total-Count"]?.firstOrNull() ?: return 0
    return value.toLongOrNull() ?: 0
}

/**
 * Extracts the `rel="next"` URL from a Link header.
 * Returns null if no next link exists.
 *
 * Example: `<https://api.example.com/page?page=2>; rel="next"` → the URL
 */
/**
 * Returns the contents of the first non-empty `<...>` pair, or null if there is none.
 *
 * Searching for `>` from after the `<` is what makes this correct: looking for
 * both independently from index 0 means a `>` that precedes the `<` yields
 * `end < start`, and the extraction silently fails for that part. It also
 * matches the leftmost-match semantics of `<([^>]+)>`, which is what the
 * TypeScript, Ruby and Python SDKs used to spell this with.
 *
 * An empty `<>` is skipped rather than returned, because `[^>]+` requires at
 * least one character. That scan was O(1), so the loop stays linear overall.
 */
internal fun extractAngleBracketed(part: String): String? {
    var cursor = 0
    while (true) {
        val start = part.indexOf('<', cursor)
        if (start < 0) return null

        val end = part.indexOf('>', start + 1)
        if (end < 0) return null

        if (end > start + 1) return part.substring(start + 1, end)
        cursor = start + 1
    }
}

internal fun parseNextLink(linkHeader: String?): String? {
    if (linkHeader.isNullOrBlank()) return null
    for (part in linkHeader.split(",")) {
        val trimmed = part.trim()
        if (trimmed.contains("""rel="next"""")) {
            extractAngleBracketed(trimmed)?.let { return it }
        }
    }
    return null
}

/**
 * Validates that two URLs share the same origin (scheme + host + port).
 * Used to prevent SSRF via poisoned Link headers.
 */
internal fun isSameOrigin(url1: String, url2: String): Boolean {
    val a = parseAbsoluteUrl(url1) ?: return false
    val b = parseAbsoluteUrl(url2) ?: return false
    // Url.port falls back to the protocol default, so an explicit default port
    // is the same origin as no port (https://h:443 ≡ https://h).
    return a.protocol.name == b.protocol.name &&
        a.host.lowercase() == b.host.lowercase() &&
        a.port == b.port
}

// parseAbsoluteUrl and isLocalhost now live in Urls.kt alongside the shared
// origin-root / secure-endpoint guards; they remain in this same package so
// isSameOrigin below calls them unqualified.

/**
 * Parses the Retry-After header value.
 * Supports integer seconds format.
 * Returns null if the header is missing or cannot be parsed.
 */
internal fun parseRetryAfter(value: String?): Int? {
    if (value.isNullOrBlank()) return null
    val seconds = value.trim().toIntOrNull()
    return if (seconds != null && seconds > 0) seconds else null
}
