package com.basecamp.sdk

import io.ktor.http.Url
import io.ktor.http.parseUrl

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
    val a = parseAbsoluteUrl(url1) ?: return false
    val b = parseAbsoluteUrl(url2) ?: return false
    // Url.port falls back to the protocol default, so an explicit default port
    // is the same origin as no port (https://h:443 ≡ https://h).
    return a.protocol.name == b.protocol.name &&
        a.host.lowercase() == b.host.lowercase() &&
        a.port == b.port
}

/**
 * Parses an absolute URL with Ktor's own parser — the SAME parser the
 * transport uses to dial — so the guard can never disagree with the client
 * about which host a URL targets. A hand-rolled parser here previously let
 * `http://evil.example\.localhost/x` pass the localhost carve-out while Ktor
 * treats `\` as a path separator and dials `evil.example`.
 *
 * Returns null (fail closed) when the input is malformed or not absolute:
 * Ktor parses a scheme-less string as a relative reference against
 * `http://localhost`, which must never be blessed as localhost/same-origin.
 */
private fun parseAbsoluteUrl(url: String): Url? {
    val parsed = parseUrl(url) ?: return null
    if (!url.startsWith("${parsed.protocol.name}://", ignoreCase = true)) return null
    return parsed
}

/** Returns true if the URL points to localhost over HTTP(S) (for dev/test). */
internal fun isLocalhost(url: String): Boolean {
    val parsed = parseAbsoluteUrl(url) ?: return false
    // The carve-out is limited to HTTP(S) so the credential backstop fails
    // closed on any other scheme (e.g. ws://localhost).
    when (parsed.protocol.name) {
        "http", "https" -> {}
        else -> return false
    }
    // Hostnames are case-insensitive (RFC 3986). Ktor's Url.host excludes
    // userinfo and port; strip IPv6 brackets in case the engine retains them.
    val host = parsed.host.lowercase().removePrefix("[").removeSuffix("]")
    return host == "localhost" ||
        host == "127.0.0.1" ||
        host == "::1" ||
        host.endsWith(".localhost") // RFC 6761 .localhost TLD
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
