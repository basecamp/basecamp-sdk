package com.basecamp.sdk

import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * Configuration for a [BasecampClient].
 *
 * Use the builder DSL via [BasecampClient] factory function rather than
 * constructing this directly.
 */
data class BasecampConfig(
    /** Base URL for the Basecamp API. */
    val baseUrl: String = DEFAULT_BASE_URL,
    /** User-Agent header value. */
    val userAgent: String = DEFAULT_USER_AGENT,
    /** Enable ETag-based HTTP caching. */
    val enableCache: Boolean = false,
    /** Enable automatic retry on 429/503 with exponential backoff. */
    val enableRetry: Boolean = true,
    /** Request timeout. */
    val timeout: Duration = 30.seconds,
    /** Maximum retry attempts for GET requests. */
    val maxRetries: Int = DEFAULT_MAX_RETRIES,
    /** Maximum pages to follow for pagination (safety cap). */
    val maxPages: Int = DEFAULT_MAX_PAGES,
    /** Base delay for exponential backoff. */
    val baseRetryDelay: Duration = 1.seconds,
) {
    companion object {
        const val VERSION = "0.2.1"
        const val API_VERSION = "2026-01-26"
        const val DEFAULT_BASE_URL = "https://3.basecampapi.com"
        const val DEFAULT_USER_AGENT = "basecamp-sdk-kotlin/$VERSION (api:$API_VERSION)"
        const val DEFAULT_MAX_RETRIES = 3
        const val DEFAULT_MAX_PAGES = 10_000
    }
}
