package com.basecamp.sdk

/**
 * Metadata about a paginated list response.
 */
data class ListMeta(
    /** Total number of items across all pages (from X-Total-Count header). */
    val totalCount: Long,
    /** True when results were truncated (by maxItems or page safety cap). */
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
)

/**
 * A list of results with pagination metadata.
 *
 * Delegates to `List<T>` so it's fully compatible with all collection operations
 * (`.forEach()`, `.map()`, `.size`, indexing, etc.). Additional metadata is
 * accessible via the [meta] property.
 *
 * ```kotlin
 * val todos = account.todos.list(projectId, todolistId)
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
internal fun parseNextLink(linkHeader: String?): String? {
    if (linkHeader.isNullOrBlank()) return null
    for (part in linkHeader.split(",")) {
        val trimmed = part.trim()
        if (trimmed.contains("""rel="next"""")) {
            val start = trimmed.indexOf('<')
            val end = trimmed.indexOf('>')
            if (start >= 0 && end > start) {
                return trimmed.substring(start + 1, end)
            }
        }
    }
    return null
}

/**
 * Validates that two URLs share the same origin (scheme + host + port).
 * Used to prevent SSRF via poisoned Link headers.
 */
internal fun isSameOrigin(url1: String, url2: String): Boolean {
    val origin1 = extractOrigin(url1) ?: return false
    val origin2 = extractOrigin(url2) ?: return false
    return origin1 == origin2
}

/** Extracts scheme://host:port from a URL string. */
private fun extractOrigin(url: String): String? {
    val schemeEnd = url.indexOf("://")
    if (schemeEnd < 0) return null
    val afterScheme = schemeEnd + 3
    // Find end of authority (host:port) — next / or end of string
    val pathStart = url.indexOf('/', afterScheme)
    val authority = if (pathStart < 0) url.substring(afterScheme) else url.substring(afterScheme, pathStart)
    return url.substring(0, schemeEnd) + "://" + authority
}

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
