package com.basecamp.sdk.services

import com.basecamp.sdk.*
import com.basecamp.sdk.http.BasecampHttpClient
import com.basecamp.sdk.http.currentTimeMillis
import com.basecamp.sdk.http.millisToDuration
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import com.basecamp.sdk.serialization.normalizePersonIds
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlin.coroutines.cancellation.CancellationException

/**
 * Abstract base class for all Basecamp API services.
 *
 * Provides shared functionality for making API requests, handling errors,
 * and integrating with the hooks system. Generated service classes extend this.
 *
 * The example below shows the shape the GENERATOR emits. Wire methods are never
 * written by hand (see AGENTS.md — paths and verbs come from the generator; the
 * only sanctioned hand-written methods are SPEC §18 composites over generated
 * wire methods):
 *
 * ```kotlin
 * // Generator output (illustrative):
 * class TodosService(client: AccountClient) : BaseService(client) {
 *     suspend fun list(todolistId: Long): ListResult<Todo> =
 *         requestPaginated(
 *             OperationInfo("Todos", "ListTodos", "todo", false, resourceId = todolistId),
 *             null,
 *             { httpGet("/todolists/$todolistId/todos.json") },
 *         ) { body -> json.decodeFromString<List<Todo>>(body) }
 * }
 * ```
 */
abstract class BaseService(
    private val accountClient: AccountClient,
) {
    private val http: BasecampHttpClient get() = accountClient.httpClient
    private val hooks: BasecampHooks get() = accountClient.parent.hooks
    protected val json: Json get() = http.json

    /** Maximum pages to follow as a safety cap against infinite loops. */
    private val maxPages: Int get() = accountClient.parent.config.maxPages

    /**
     * Builds the full API URL for a path relative to the account.
     * E.g., "/projects.json" -> "https://3.basecampapi.com/{accountId}/projects.json"
     */
    protected fun accountUrl(path: String): String {
        if (path.startsWith("http://", ignoreCase = true) || path.startsWith("https://", ignoreCase = true)) {
            // Absolute URL: require same-origin so we never build a request that
            // would attach the bearer token to a foreign host.
            if (!isLocalhost(path) && !isSameOrigin(path, accountClient.parent.config.baseUrl)) {
                throw BasecampException.Usage(
                    "Refusing to build a request URL for a different origin than base URL: " +
                        BasecampException.truncateMessage(path)
                )
            }
            return path
        }
        val base = accountClient.parent.config.baseUrl.trimEnd('/')
        val accountId = accountClient.accountId
        val normalizedPath = if (path.startsWith("/")) path else "/$path"
        return "$base/$accountId$normalizedPath"
    }

    /**
     * Builds a query string from key-value pairs, URL-encoding values.
     * Null values are omitted. Returns "" if no params, or "?k1=v1&k2=v2".
     */
    protected fun buildQueryString(vararg params: Pair<String, Any?>): String {
        val parts = params.flatMap { (key, value) ->
            val encodedKey = key.encodeURLParameter()
            when (value) {
                null -> emptyList()
                // Array wire names arrive as an Iterable keyed by the raw
                // bracketed name (e.g. "bucket_ids[]"); expand into one
                // repeated `key=elem` pair per element rather than stringifying
                // the whole list. The key is encoded (`[]` → `%5B%5D`).
                is Iterable<*> -> value.filterNotNull()
                    .map { "$encodedKey=${it.toString().encodeURLParameter()}" }
                else -> listOf("$encodedKey=${value.toString().encodeURLParameter()}")
            }
        }
        return if (parts.isEmpty()) "" else "?" + parts.joinToString("&")
    }

    /**
     * Executes a GET request for the given account-relative path.
     */
    protected suspend fun httpGet(path: String, operationName: String? = null): HttpResponse =
        http.requestWithRetry(HttpMethod.Get, accountUrl(path), operationName = operationName)

    /**
     * Executes a POST request with a JSON body.
     */
    protected suspend fun httpPost(path: String, body: String? = null, operationName: String? = null): HttpResponse =
        http.requestWithRetry(HttpMethod.Post, accountUrl(path), body, operationName = operationName)

    /**
     * Executes a PUT request with a JSON body.
     */
    protected suspend fun httpPut(path: String, body: String? = null, operationName: String? = null): HttpResponse =
        http.requestWithRetry(HttpMethod.Put, accountUrl(path), body, operationName = operationName)

    /**
     * Executes a DELETE request.
     */
    protected suspend fun httpDelete(path: String, operationName: String? = null): HttpResponse =
        http.requestWithRetry(HttpMethod.Delete, accountUrl(path), operationName = operationName)

    /**
     * Executes a POST request with binary body data.
     */
    protected suspend fun httpPostBinary(path: String, data: ByteArray, contentType: String): HttpResponse =
        http.requestBinaryWithRetry(HttpMethod.Post, accountUrl(path), data, contentType)

    /**
     * Executes a PUT request with multipart/form-data body.
     * Builds the MIME multipart envelope with proper header sanitization.
     */
    protected suspend fun httpPutMultipart(
        path: String,
        fieldName: String,
        data: ByteArray,
        filename: String,
        contentType: String,
    ): HttpResponse {
        val safeFieldName = fieldName.replace("\r", "").replace("\n", "").replace("\"", "\\\"")
        val safeFilename = filename.replace("\r", "").replace("\n", "").replace("\"", "\\\"")
        val safeContentType = contentType.replace("\r", "").replace("\n", "")
        val boundary = "----BasecampSDK${com.basecamp.sdk.http.currentTimeMillis()}"
        val preamble = buildString {
            append("--$boundary\r\n")
            append("Content-Disposition: form-data; name=\"$safeFieldName\"; filename=\"$safeFilename\"\r\n")
            append("Content-Type: $safeContentType\r\n")
            append("\r\n")
        }
        val epilogue = "\r\n--$boundary--\r\n"

        val preambleBytes = preamble.encodeToByteArray()
        val epilogueBytes = epilogue.encodeToByteArray()
        val body = ByteArray(preambleBytes.size + data.size + epilogueBytes.size)
        preambleBytes.copyInto(body, 0)
        data.copyInto(body, preambleBytes.size)
        epilogueBytes.copyInto(body, preambleBytes.size + data.size)

        return http.requestBinaryWithRetry(
            HttpMethod.Put,
            accountUrl(path),
            body,
            "multipart/form-data; boundary=$boundary",
        )
    }

    /**
     * Executes an API request with error handling and hooks integration.
     *
     * @param info Operation metadata for hooks.
     * @param fn The suspend function that performs the actual HTTP call.
     * @param parse Deserializes the response body string into the result type.
     * @return The parsed response.
     */
    protected suspend fun <T> request(
        info: OperationInfo,
        fn: suspend () -> HttpResponse,
        parse: (String) -> T,
    ): T {
        val startTime = currentTimeMillis()

        hooks.safeOnOperationStart(info)

        try {
            val response = fn()
            val duration = (currentTimeMillis() - startTime).millisToDuration()

            if (!response.status.isSuccess()) {
                val error = errorFromResponse(response)
                hooks.safeOnOperationEnd(info, OperationResult(duration, error))
                throw error
            }

            // 204 No Content needs no special case. Every void operation the
            // generator emits parses with `{ Unit }`, which ignores the body and
            // so answers an empty one correctly — all 48 of them. The shortcut
            // this replaces returned `Unit as T` for ANY operation, so a
            // value-returning one that unexpectedly got a 204 reported no
            // failure at all: the unchecked cast hands back Unit wearing the
            // model's type, and the ClassCastException surfaces later, at
            // whatever site first uses the value — or never, if the caller
            // discards it. Letting the empty body reach the decoder makes it
            // what it is, where it happens: a malformed response.
            val bodyText = normalizePersonIds(response.bodyAsText(), json)
            val result = decodeOrApiError(info.operation) { parse(bodyText) }
            hooks.safeOnOperationEnd(info, OperationResult(duration))
            return result
        } catch (e: BasecampException) {
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration, e))
            throw e
        } catch (e: Exception) {
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration, e))
            throw e
        }
    }

    /**
     * Executes a paginated API request, automatically following Link headers.
     *
     * Returns a [ListResult] with all items across pages, plus [ListMeta]
     * with `totalCount` and `truncated` information.
     *
     * @param info Operation metadata for hooks.
     * @param options Pagination control (maxItems).
     * @param fn The suspend function that performs the initial HTTP call.
     * @param parseItems Parses a page's response body into a list of items.
     */
    protected suspend fun <T> requestPaginated(
        info: OperationInfo,
        options: PaginationOptions? = null,
        fn: suspend () -> HttpResponse,
        parseItems: (String) -> List<T>,
    ): ListResult<T> {
        val startTime = currentTimeMillis()
        val maxItems = options?.maxItems

        hooks.safeOnOperationStart(info)

        try {
            val response = fn()

            if (!response.status.isSuccess()) {
                val error = errorFromResponse(response)
                val duration = (currentTimeMillis() - startTime).millisToDuration()
                hooks.safeOnOperationEnd(info, OperationResult(duration, error))
                throw error
            }

            val bodyText = normalizePersonIds(response.bodyAsText(), json)
            val firstPageItems = decodeOrApiError(info.operation) { parseItems(bodyText) }
            val totalCount = parseTotalCount(response.headers.toMap())

            // A pinned page is the whole answer: return it without following
            // links. `truncated` still reports whether more items existed —
            // dropped by the cap, or reachable through the next link we
            // deliberately did not follow.
            if ((options?.page ?: 0L) > 0L) {
                val cap = maxItems?.takeIf { it > 0 && firstPageItems.size > it }
                val hasMore = cap != null || parseNextLink(response.headers["Link"]) != null
                val items = if (cap != null) firstPageItems.take(cap) else firstPageItems
                val duration = (currentTimeMillis() - startTime).millisToDuration()
                val result = ListResult(items, ListMeta(totalCount, hasMore))
                hooks.safeOnOperationEnd(info, OperationResult(duration))
                return result
            }

            // Check if maxItems is satisfied by the first page
            if (maxItems != null && maxItems > 0 && firstPageItems.size >= maxItems) {
                val hasMore = firstPageItems.size > maxItems
                    || parseNextLink(response.headers["Link"]) != null
                val duration = (currentTimeMillis() - startTime).millisToDuration()
                val result = ListResult(firstPageItems.take(maxItems), ListMeta(totalCount, hasMore))
                hooks.safeOnOperationEnd(info, OperationResult(duration))
                return result
            }

            // Follow pagination
            val allItems = firstPageItems.toMutableList()
            var currentResponse = response
            val initialUrl = response.request.url.toString()

            for (page in 1 until maxPages) {
                val rawNextUrl = parseNextLink(currentResponse.headers["Link"]) ?: break
                val nextUrl = resolveUrl(currentResponse.request.url.toString(), rawNextUrl)

                // Validate same-origin to prevent SSRF / token leakage
                if (!isSameOrigin(nextUrl, initialUrl)) {
                    throw BasecampException.Api(
                        "Pagination Link header points to different origin: $nextUrl",
                        httpStatus = 0,
                    )
                }

                currentResponse = http.requestWithRetry(HttpMethod.Get, nextUrl, operationName = info.operation)

                if (!currentResponse.status.isSuccess()) {
                    throw errorFromResponse(currentResponse)
                }

                val pageBody = normalizePersonIds(currentResponse.bodyAsText(), json)
                val pageItems = decodeOrApiError(info.operation) { parseItems(pageBody) }
                allItems.addAll(pageItems)

                // Check maxItems cap
                if (maxItems != null && maxItems > 0 && allItems.size >= maxItems) {
                    val hasMore = allItems.size > maxItems
                        || parseNextLink(currentResponse.headers["Link"]) != null
                    val duration = (currentTimeMillis() - startTime).millisToDuration()
                    val result = ListResult(allItems.take(maxItems), ListMeta(totalCount, hasMore))
                    hooks.safeOnOperationEnd(info, OperationResult(duration))
                    return result
                }
            }

            val hasMore = parseNextLink(currentResponse.headers["Link"]) != null
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            val result = ListResult(allItems, ListMeta(totalCount, hasMore))
            hooks.safeOnOperationEnd(info, OperationResult(duration))
            return result
        } catch (e: BasecampException) {
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration, e))
            throw e
        } catch (e: Exception) {
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration, e))
            throw e
        }
    }

    /**
     * Executes a paginated request for wrapped responses, assembling the result
     * from the paginated items plus the first page's remaining wrapper members.
     *
     * [buildResult] runs **inside this primitive's decode isolation**, and that
     * is the whole point of its being a parameter rather than the caller's next
     * statement. A wrapped response is decoded in two halves — the items array
     * on every page, and the wrapper's other members off the first page — and
     * for as long as this function handed back the raw first page body, the
     * second half was decoded by generated code *after* the primitive returned,
     * so a malformed wrapper surfaced as whatever the generated accessors threw
     * rather than as SPEC §6's statusless `api_error` (#728). Threading the
     * lambda makes that isolation structural: there is no way to reach the
     * wrapper body without going through [decodeOrApiError], so a wrapped
     * operation added later inherits the guarantee instead of having to
     * remember it.
     *
     * @param info Operation metadata for hooks.
     * @param options Pagination control (maxItems).
     * @param fn The suspend function that performs the initial HTTP call.
     * @param parseItems Parses a page's response body into a list of items.
     * @param buildResult Decodes the first page's wrapper members and combines
     *   them with the accumulated items. Raise [SerializationException] for a
     *   member that is absent or wrong-typed; it is mapped like any other
     *   decode failure.
     * @return Whatever [buildResult] returns.
     */
    protected suspend fun <T, R> requestPaginatedWrapped(
        info: OperationInfo,
        options: PaginationOptions? = null,
        fn: suspend () -> HttpResponse,
        parseItems: (String) -> List<T>,
        buildResult: (String, ListResult<T>) -> R,
    ): R {
        val startTime = currentTimeMillis()
        val maxItems = options?.maxItems

        hooks.safeOnOperationStart(info)

        try {
            val response = fn()

            if (!response.status.isSuccess()) {
                val error = errorFromResponse(response)
                val duration = (currentTimeMillis() - startTime).millisToDuration()
                hooks.safeOnOperationEnd(info, OperationResult(duration, error))
                throw error
            }

            val firstPageBody = normalizePersonIds(response.bodyAsText(), json)
            val firstPageItems = decodeOrApiError(info.operation) { parseItems(firstPageBody) }
            val totalCount = parseTotalCount(response.headers.toMap())

            // A pinned page is the whole answer: return it without following
            // links. `truncated` still reports whether more items existed —
            // dropped by the cap, or reachable through the next link we
            // deliberately did not follow.
            if ((options?.page ?: 0L) > 0L) {
                val cap = maxItems?.takeIf { it > 0 && firstPageItems.size > it }
                val hasMore = cap != null || parseNextLink(response.headers["Link"]) != null
                val items = if (cap != null) firstPageItems.take(cap) else firstPageItems
                return finishWrapped(
                    info, startTime, firstPageBody,
                    ListResult(items, ListMeta(totalCount, hasMore)), buildResult,
                )
            }

            // Check if maxItems is satisfied by the first page
            if (maxItems != null && maxItems > 0 && firstPageItems.size >= maxItems) {
                val hasMore = firstPageItems.size > maxItems
                    || parseNextLink(response.headers["Link"]) != null
                return finishWrapped(
                    info, startTime, firstPageBody,
                    ListResult(firstPageItems.take(maxItems), ListMeta(totalCount, hasMore)), buildResult,
                )
            }

            // Follow pagination
            val allItems = firstPageItems.toMutableList()
            var currentResponse = response
            val initialUrl = response.request.url.toString()

            for (page in 1 until maxPages) {
                val rawNextUrl = parseNextLink(currentResponse.headers["Link"]) ?: break
                val nextUrl = resolveUrl(currentResponse.request.url.toString(), rawNextUrl)

                if (!isSameOrigin(nextUrl, initialUrl)) {
                    throw BasecampException.Api(
                        "Pagination Link header points to different origin: $nextUrl",
                        httpStatus = 0,
                    )
                }

                currentResponse = http.requestWithRetry(HttpMethod.Get, nextUrl, operationName = info.operation)

                if (!currentResponse.status.isSuccess()) {
                    throw errorFromResponse(currentResponse)
                }

                val pageBody = normalizePersonIds(currentResponse.bodyAsText(), json)
                val pageItems = decodeOrApiError(info.operation) { parseItems(pageBody) }
                allItems.addAll(pageItems)

                if (maxItems != null && maxItems > 0 && allItems.size >= maxItems) {
                    val hasMore = allItems.size > maxItems
                        || parseNextLink(currentResponse.headers["Link"]) != null
                    return finishWrapped(
                        info, startTime, firstPageBody,
                        ListResult(allItems.take(maxItems), ListMeta(totalCount, hasMore)), buildResult,
                    )
                }
            }

            val hasMore = parseNextLink(currentResponse.headers["Link"]) != null
            return finishWrapped(
                info, startTime, firstPageBody,
                ListResult(allItems, ListMeta(totalCount, hasMore)), buildResult,
            )
        } catch (e: BasecampException) {
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration, e))
            throw e
        } catch (e: Exception) {
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration, e))
            throw e
        }
    }

    /**
     * The single exit of [requestPaginatedWrapped]: decode the wrapper, then
     * report the operation as ended.
     *
     * Ordering is the reason this is a function rather than four repetitions.
     * The wrapper decode has to happen BEFORE `onOperationEnd` fires, or a
     * malformed wrapper would be reported to hooks as a success and then thrown
     * — which is exactly what happened while generated code did this decode
     * after the primitive had returned. Raising here instead leaves the throw
     * inside the caller's `try`, so the existing `catch` reports the operation
     * as failed, once, with the error.
     */
    private fun <T, R> finishWrapped(
        info: OperationInfo,
        startTime: Long,
        firstPageBody: String,
        items: ListResult<T>,
        buildResult: (String, ListResult<T>) -> R,
    ): R {
        val result = decodeOrApiError(info.operation) { buildResult(firstPageBody, items) }
        val duration = (currentTimeMillis() - startTime).millisToDuration()
        hooks.safeOnOperationEnd(info, OperationResult(duration))
        return result
    }

    /**
     * Streaming paginated request that emits items as each page arrives.
     *
     * Unlike [requestPaginated] which eagerly loads all pages, this returns
     * a cold [Flow] that fetches pages lazily as the collector consumes items.
     * Useful for processing large datasets without loading everything into memory.
     *
     * No generated service currently emits a method on this primitive; it is
     * reserved for a future generated streaming surface. If the generator gains
     * one, its output would take this shape (wire methods are never written by
     * hand — see AGENTS.md):
     *
     * ```kotlin
     * // Generator output (illustrative):
     * fun listAsFlow(todolistId: Long): Flow<Todo> =
     *     requestPaginatedAsFlow(
     *         OperationInfo("Todos", "ListTodos", "todo", false, resourceId = todolistId),
     *         { httpGet("/todolists/$todolistId/todos.json") },
     *     ) { body -> json.decodeFromString<List<Todo>>(body) }
     *
     * // Collectors consume pages lazily:
     * service.listAsFlow(todolistId).collect { todo -> println(todo.content) }
     * ```
     *
     * @param info Operation metadata for hooks.
     * @param fn The suspend function that performs the initial HTTP call.
     * @param parseItems Parses a page's response body into a list of items.
     */
    protected fun <T> requestPaginatedAsFlow(
        info: OperationInfo,
        fn: suspend () -> HttpResponse,
        parseItems: (String) -> List<T>,
    ): Flow<T> = flow {
        val startTime = currentTimeMillis()
        hooks.safeOnOperationStart(info)

        try {
            var currentResponse = fn()

            if (!currentResponse.status.isSuccess()) {
                throw errorFromResponse(currentResponse)
            }

            val bodyText = currentResponse.bodyAsText()
            val firstPageItems = decodeOrApiError(info.operation) { parseItems(bodyText) }
            for (item in firstPageItems) emit(item)

            val initialUrl = currentResponse.request.url.toString()

            for (page in 1 until maxPages) {
                val rawNextUrl = parseNextLink(currentResponse.headers["Link"]) ?: break
                val nextUrl = resolveUrl(currentResponse.request.url.toString(), rawNextUrl)

                if (!isSameOrigin(nextUrl, initialUrl)) {
                    throw BasecampException.Api(
                        "Pagination Link header points to different origin: $nextUrl",
                        httpStatus = 0,
                    )
                }

                currentResponse = http.requestWithRetry(HttpMethod.Get, nextUrl, operationName = info.operation)

                if (!currentResponse.status.isSuccess()) {
                    throw errorFromResponse(currentResponse)
                }

                val pageBody = normalizePersonIds(currentResponse.bodyAsText(), json)
                val pageItems = decodeOrApiError(info.operation) { parseItems(pageBody) }
                for (item in pageItems) emit(item)
            }

            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration))
        } catch (e: Exception) {
            val duration = (currentTimeMillis() - startTime).millisToDuration()
            hooks.safeOnOperationEnd(info, OperationResult(duration, e))
            throw e
        }
    }

    /**
     * Runs the response decoder — and nothing else — mapping a decode failure
     * into the SPEC §6 statusless `api_error` shape.
     *
     * Statusless because the request succeeded, so no HTTP status describes the
     * failure; non-retryable because re-requesting cannot repair a malformed
     * body. The decoder's own exception is kept as `cause`, so a caller that
     * wants its account of what was wrong still has it.
     *
     * **One mapped type, deliberately.** [SerializationException] is the type
     * kotlinx uses to say "this body is not what the model expects", and the
     * `cause` it becomes here is a contract other code reads: the §18
     * composites and the conformance runner both tell a decoder rejection from a
     * real `api_error` through [BasecampException.Api.decodeFailure], the slot
     * [malformedBody] alone fills, and read the exception itself back out of
     * it. A second mapped type would be a second cause type they would
     * each have to learn, so anything that is a decode failure is made to speak
     * this one *where it is raised* instead — see
     * [com.basecamp.sdk.serialization.FlexibleLongSerializer], whose numeric
     * conversion would otherwise leak a [NumberFormatException].
     *
     * Two classes stay unmapped on purpose: [NullPointerException] and
     * [IllegalArgumentException]. Catching either would swallow every `!!` and
     * every `require()` in reach, turning a programming error into a report
     * about the server's body. The wrapped-pagination generator used to emit
     * `["events"]!!.jsonArray`, whose failures those two names cover exactly,
     * and the repair was to stop emitting them — the emitted accessor now
     * raises [SerializationException] through
     * [com.basecamp.sdk.serialization.requiredMember] and
     * `decodeFromJsonElement`, so the failure arrives speaking the one mapped
     * type rather than asking this catch to learn two more (#728).
     *
     * **Wrap the decode expression, never the block.** Each primitive above runs
     * encode → URL build → auth → transport → status check → decode inside one
     * `try` whose `catch` maps nothing, which is why a malformed 2xx body used
     * to surface as a raw [SerializationException], indistinguishable from the
     * auth strategy throwing or the socket dropping. Only the decode call is
     * wrapped: the generated `fn` lambda serializes the *request* body
     * (`json.encodeToString(...)`) inside the same `try` and throws the very
     * same [SerializationException] type, so wrapping the block instead of the
     * expression would relabel a request-encoding fault as a malformed
     * response — the same conflation in a new shape.
     */
    private fun <T> decodeOrApiError(operation: String, decode: () -> T): T =
        try {
            decode()
        } catch (e: SerializationException) {
            throw malformedBody(operation, e)
        }

    /**
     * The one place a decode failure is first rendered — and, through the
     * internal factory it calls, one of the two producers of
     * [BasecampException.Api.decodeFailure]; the §18 composites are the other,
     * restating this exception through the same factory. That slot is how the §18
     * composites and the conformance runner tell this exception from any other
     * `api_error` that happens to carry a [SerializationException] as its
     * `cause`, which an auth strategy's already-classified failure can (#730).
     */
    private fun malformedBody(operation: String, cause: SerializationException): BasecampException.Api =
        BasecampException.Api.malformedBody(
            message = BasecampException.truncateMessage(
                "$operation returned a body that does not decode: ${cause.message}"
            ),
            decodeFailure = cause,
        )

    /**
     * Converts an HTTP error response to a [BasecampException] via the shared
     * SPEC §6 parser ([exceptionFromErrorBody]) used by every SDK surface.
     */
    private suspend fun errorFromResponse(response: HttpResponse): BasecampException {
        val bodyText = try {
            normalizePersonIds(response.bodyAsText(), json)
        } catch (e: CancellationException) {
            throw e
        } catch (_: Exception) {
            null
        }
        return exceptionFromErrorBody(
            status = response.status.value,
            statusDescription = response.status.description,
            bodyText = bodyText,
            requestId = response.headers["X-Request-Id"],
            retryAfter = parseRetryAfter(response.headers["Retry-After"]),
            json = json,
        )
    }

    companion object {
        /** Resolve a potentially relative URL against a base URL. */
        internal fun resolveUrl(base: String, relative: String): String {
            // If it's already absolute, return as-is
            if (relative.startsWith("http://") || relative.startsWith("https://")) {
                return relative
            }
            // Extract origin from base
            val schemeEnd = base.indexOf("://")
            if (schemeEnd < 0) return relative
            val afterScheme = schemeEnd + 3
            val pathStart = base.indexOf('/', afterScheme)
            val origin = if (pathStart < 0) base else base.substring(0, pathStart)
            val normalizedPath = if (relative.startsWith("/")) relative else "/$relative"
            return "$origin$normalizedPath"
        }
    }
}

/** Safely invoke onOperationStart, catching hook exceptions. */
private fun BasecampHooks.safeOnOperationStart(info: OperationInfo) {
    runCatching { onOperationStart(info) }
}

/** Safely invoke onOperationEnd, catching hook exceptions. */
private fun BasecampHooks.safeOnOperationEnd(info: OperationInfo, result: OperationResult) {
    runCatching { onOperationEnd(info, result) }
}

/** Convert Ktor headers to a simple map for pagination utilities. */
private fun io.ktor.http.Headers.toMap(): Map<String, List<String>> {
    val result = mutableMapOf<String, List<String>>()
    forEach { key, values -> result[key] = values }
    return result
}
