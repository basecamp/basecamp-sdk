package com.basecamp.sdk

import com.basecamp.sdk.http.BasecampHttpClient
import io.ktor.client.*
import io.ktor.client.engine.*
import io.ktor.client.plugins.HttpTimeout
import io.ktor.http.*
import kotlinx.serialization.json.Json

/**
 * Builder DSL for configuring a [BasecampClient].
 *
 * ```kotlin
 * val client = BasecampClient {
 *     accessToken("your-token")
 *     userAgent = "my-app/1.0"
 *     enableCache = true
 *     hooks = consoleHooks()
 * }
 * ```
 */
class BasecampClientBuilder {
    /** Token provider for authentication. Set via [accessToken]. */
    var tokenProvider: TokenProvider? = null

    /** Custom authentication strategy. Mutually exclusive with [tokenProvider]. */
    var authStrategy: AuthStrategy? = null

    /** Base URL for the API. Defaults to the production Basecamp API. */
    var baseUrl: String = BasecampConfig.DEFAULT_BASE_URL

    /** User-Agent header. */
    var userAgent: String = BasecampConfig.DEFAULT_USER_AGENT

    /** Enable ETag-based HTTP caching. */
    var enableCache: Boolean = false

    /** Enable automatic retry on 429/503. */
    var enableRetry: Boolean = true

    /** Observability hooks. */
    var hooks: BasecampHooks = NoopHooks

    /** Custom Ktor [HttpClientEngine] (e.g., for testing with MockEngine). */
    var engine: HttpClientEngine? = null

    /** Pre-configured Ktor [HttpClient] to use instead of creating one internally. */
    var httpClient: HttpClient? = null

    /** Set a static access token. */
    fun accessToken(token: String) {
        tokenProvider = StaticTokenProvider(token)
    }

    /** Set a dynamic access token provider. */
    fun accessToken(provider: suspend () -> String) {
        tokenProvider = TokenProvider { provider() }
    }

    /** Set a custom authentication strategy. */
    fun auth(strategy: AuthStrategy) {
        authStrategy = strategy
    }

    internal fun build(): BasecampClient {
        require(tokenProvider == null || authStrategy == null) {
            "Cannot set both accessToken and auth. Use one or the other."
        }
        require(httpClient == null || engine == null) {
            "Cannot set both httpClient and engine. Use one or the other."
        }

        val resolvedAuth = authStrategy
            ?: tokenProvider?.let { BearerAuth(it) }
            ?: throw IllegalArgumentException(
                "Authentication must be configured. Use accessToken(\"token\") or auth(strategy)."
            )

        val config = BasecampConfig(
            baseUrl = baseUrl,
            userAgent = userAgent,
            enableCache = enableCache,
            enableRetry = enableRetry,
        )

        // Validate HTTPS (allow localhost for testing)
        if (!isLocalhost(baseUrl)) {
            val parsed = Url(baseUrl)
            require(parsed.protocol == URLProtocol.HTTPS) {
                "Base URL must use HTTPS: $baseUrl"
            }
        }

        return BasecampClient(resolvedAuth, config, hooks, engine, httpClient)
    }
}

/** Returns true if the URL points to localhost (for dev/test). */
private fun isLocalhost(url: String): Boolean {
    val hostStart = url.indexOf("://")
    if (hostStart < 0) return false
    val afterScheme = hostStart + 3
    val hostEnd = url.indexOfAny(charArrayOf('/', ':', '?'), afterScheme).let {
        if (it < 0) url.length else it
    }
    val host = url.substring(afterScheme, hostEnd)
    return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

/**
 * Creates a [BasecampClient] using the builder DSL.
 *
 * ```kotlin
 * val client = BasecampClient {
 *     accessToken("your-token")
 *     userAgent = "my-app/1.0"
 * }
 *
 * val account = client.forAccount("12345")
 * val projects = account.projects.list()
 * ```
 */
fun BasecampClient(block: BasecampClientBuilder.() -> Unit): BasecampClient =
    BasecampClientBuilder().apply(block).build()

/**
 * Root client for the Basecamp API.
 *
 * Holds shared resources (HTTP client, token provider, hooks) and creates
 * [AccountClient] instances for specific Basecamp accounts via [forAccount].
 *
 * Thread-safe after construction.
 */
class BasecampClient internal constructor(
    internal val authStrategy: AuthStrategy,
    internal val config: BasecampConfig,
    internal val hooks: BasecampHooks,
    engine: HttpClientEngine?,
    externalHttpClient: HttpClient?,
) {
    /** Whether the SDK created (and therefore owns) the underlying HttpClient. */
    private val ownsHttpClient = externalHttpClient == null

    internal val json: Json = Json {
        ignoreUnknownKeys = true
        isLenient = true
        coerceInputValues = true
    }

    internal val httpClient: BasecampHttpClient = BasecampHttpClient(
        httpClient = externalHttpClient ?: if (engine != null) {
            HttpClient(engine) { configureClient() }
        } else {
            HttpClient { configureClient() }
        },
        authStrategy = authStrategy,
        config = config,
        hooks = hooks,
        json = json,
    )

    private fun HttpClientConfig<*>.configureClient() {
        expectSuccess = false
        install(HttpTimeout) {
            requestTimeoutMillis = config.timeout.inWholeMilliseconds
            connectTimeoutMillis = config.timeout.inWholeMilliseconds
            socketTimeoutMillis = config.timeout.inWholeMilliseconds
        }
    }

    /**
     * Creates an [AccountClient] bound to the given Basecamp account.
     *
     * The returned client shares this parent's HTTP transport, token provider,
     * and hooks. Creating multiple AccountClients is lightweight.
     *
     * @param accountId Numeric account ID (found in your Basecamp URL).
     */
    fun forAccount(accountId: String): AccountClient {
        require(accountId.isNotBlank()) { "Account ID must not be blank" }
        require(accountId.all { it.isDigit() }) { "Account ID must be numeric, got: $accountId" }
        return AccountClient(this, accountId)
    }

    /**
     * Shuts down the underlying HTTP client, if the SDK created it.
     * If a caller-provided [HttpClient] was passed to the builder, the SDK
     * does not close it — the caller retains ownership.
     */
    fun close() {
        if (ownsHttpClient) {
            httpClient.httpClient.close()
        }
    }
}

/**
 * Account-scoped client for the Basecamp API.
 *
 * All service accessors are available as properties. Services are lazily
 * initialized and cached for the lifetime of this client.
 *
 * ```kotlin
 * val account = client.forAccount("12345")
 * val projects = account.projects.list()
 * val todo = account.todos.get(projectId = 123, todoId = 456)
 * ```
 *
 * **Extensibility**: External modules can add services via Kotlin extension
 * properties using the [service] function:
 * ```kotlin
 * val AccountClient.customService: CustomService
 *     get() = service("custom") { CustomService(this) }
 * ```
 */
class AccountClient internal constructor(
    val parent: BasecampClient,
    val accountId: String,
) {
    @PublishedApi
    internal val serviceCache: MutableMap<String, Any> = createServiceCache()

    internal val httpClient: BasecampHttpClient get() = parent.httpClient

    /**
     * Gets or creates a cached service instance.
     *
     * This is the extension point for external modules to add services
     * without subclassing AccountClient.
     */
    inline fun <reified T : Any> service(key: String, crossinline factory: () -> T): T =
        @Suppress("UNCHECKED_CAST")
        (serviceCache.getOrPut(key) { factory() } as T)

}
