package com.basecamp.sdk.conformance

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.generated.services.*
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import io.ktor.http.content.*
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.MissingFieldException
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.*
import java.io.File
import java.io.IOException
import java.util.concurrent.atomic.AtomicInteger

/** Default account ID for conformance tests. */
private const val TEST_ACCOUNT_ID = "999"

/**
 * The #555 stop-on-mismatch message for a decoder rejection, split the way the
 * fixture author has to act on it: an absent required field is a different
 * repair from a wrong-typed one. Shared by the three arms that can now see such
 * a rejection so the wording cannot drift between them.
 */
private fun decodeFailureMessage(e: SerializationException): String =
    if (e is MissingFieldException) {
        "Mock body lacks required Kotlin model fields: ${e.message}"
    } else {
        "Mock body does not decode into the Kotlin model: ${e.message}"
    }

/** Tests where the Kotlin runner's operation dispatcher has no implementation yet. */
private val KOTLIN_SKIPS: Map<String, String> = emptyMap()

/**
 * The date window every GetUpcomingSchedule case is dispatched with. Fixed in
 * the runner because no mock runner consumes queryParams and no assertion type
 * can pin a query string — every runner records the path with the query
 * stripped.
 */
private const val UPCOMING_WINDOW_START = "2026-06-01"
private const val UPCOMING_WINDOW_END = "2026-06-30"

/**
 * The query every Search case is dispatched with, fixed for the same reason. It
 * is required, and the mock returns its queued body regardless of what is asked
 * for.
 */
private const val SEARCH_QUERY = "Leto"

/**
 * Flattens the upcoming-schedule envelope into top-level scalars.
 *
 * Go and TypeScript resolve a responseBody path as a top-level key only, so the
 * assertions read scalars rather than walk into the arrays. Every value here
 * comes off the decoded model, which is what makes the case a decode test.
 */
private fun summarizeUpcoming(envelope: UpcomingScheduleResult): JsonElement = buildJsonObject {
    put("schedule_entries_count", envelope.scheduleEntries.size)
    put("recurring_occurrences_count", envelope.recurringScheduleEntryOccurrences.size)
    put("assignables_count", envelope.assignables.size)
    envelope.scheduleEntries.firstOrNull()?.let { entry ->
        put("entry_summary", entry.summary)
        put("entry_recurring", entry.recurring)
        put("entry_bucket_name", entry.bucket.name)
    }
    envelope.recurringScheduleEntryOccurrences.firstOrNull()?.let { occurrence ->
        put("occurrence_recurring", occurrence.recurring)
        put("occurrence_all_day", occurrence.allDay)
        put("occurrence_starts_at", occurrence.startsAt)
    }
    envelope.assignables.firstOrNull()?.let { assignable ->
        put("assignable_content", assignable.content)
        put("assignable_type", assignable.type)
        put("assignable_parent_title", assignable.parent.title)
        put("assignable_completion_url", assignable.completionUrl)
    }
}

/**
 * Flattens an accumulated project list into top-level scalars.
 *
 * Flat and scalar because that is the only path form every runner can resolve:
 * Go and TypeScript read a responseBody path as a top-level key with no dot
 * splitting, and this runner's navigator (like Swift's) descends through
 * JsonObjects only, so neither a dotted path nor an array index is portable.
 *
 * It exists so a fixture can prove the items of a followed page were
 * ACCUMULATED, not merely fetched. requestCount only sees that the second
 * request happened, and meta.totalCount is the X-Total-Count header rather than
 * the item count, so an SDK that fetched page 2 and discarded its body
 * satisfies both.
 */
private fun summarizeProjects(projects: List<Project>): JsonElement = buildJsonObject {
    put("project_count", projects.size)
    put("first_project_id", projects.firstOrNull()?.id ?: 0L)
    put("last_project_id", projects.lastOrNull()?.id ?: 0L)
}

/**
 * Flattens the versions array into top-level scalars.
 *
 * GET /uploads/{id}/versions.json returns an ARRAY and a responseBody path
 * resolves as a top-level key only. Every value comes off the DECODED model, so
 * this is a decode test of the retype that closes #649 and not a transport test
 * — kotlinx.serialization rejects a body missing any non-nullable member.
 */
private fun summarizeUploadVersions(versions: List<UploadVersion>): JsonElement = buildJsonObject {
    put("versions_count", versions.size)
    put("current_count", versions.count { it.upload?.current == true })
    versions.firstOrNull()?.let { first ->
        put("first_action", first.action)
        first.upload?.let { file ->
            put("first_filename", file.filename)
            file.contentType?.let { put("first_content_type", it) }
            file.byteSize?.let { put("first_byte_size", it) }
            put("first_current", file.current)
        }
    }
    versions.lastOrNull()?.let { last ->
        put("last_action", last.action)
        // A version whose recordable no longer resolves omits the upload object
        // entirely — the optionality UploadVersion.upload declares.
        put("last_has_upload", last.upload != null)
    }
}


/**
 * Flattens a search result list into top-level scalars, one group per branch of
 * BC3's polymorphic search projection.
 *
 * Flat and scalar for the reason summarizeProjects gives. Boolean for a second
 * reason: the response is an ARRAY and no assertion type expresses absence
 * inside one — there is headerAbsent and requestBodyAbsent, but no
 * responseBodyAbsent — and the file-attachment branch is recognized precisely BY
 * the absence of the five envelope keys. Encoding that as a boolean is the
 * established idiom (last_has_upload).
 *
 * Each hit is selected by predicate rather than by index, so a fixture can
 * present one branch alone and still assert the others report honestly.
 *
 * Every value comes off the DECODED model, so this is a decode test of the
 * retype that closes #717 and not a transport test — before it, `search`
 * returned `ListResult<JsonElement>` and no search body could fail here.
 */
private fun summarizeSearch(results: List<SearchResult>): JsonElement = buildJsonObject {
    val generic = results.firstOrNull { it.type != null }
    val attachment = results.firstOrNull { it.type == null }
    val uploadLine = results.firstOrNull { it.type == "Chat::Lines::Upload" }
    val needle = results.firstOrNull { it.type == "Gauge::Needle" }
    val kanban = results.firstOrNull { it.type == "Kanban::Column" }
    val uploadAttachment = uploadLine?.attachments?.firstOrNull()
    val needleAttachment = needle?.attachments?.firstOrNull()

    put("result_count", results.size)
    put("bubble_up_url_count", results.count { it.bubbleUpUrl != null })

    // The generic recording envelope — the control group.
    put("generic_type", generic?.type ?: "")
    put("generic_has_id", generic?.id != null)
    put("generic_has_title", generic?.title != null)
    put("generic_has_type", generic?.type != null)
    put("generic_has_url", generic?.url != null)
    put("generic_has_app_url", generic?.appUrl != null)

    // The file-attachment branch: searches/_attachment.json.jbuilder writes its
    // own projection, so the absence of a type IS the discriminator.
    put("attachment_has_id", attachment?.id != null)
    put("attachment_has_title", attachment?.title != null)
    put("attachment_has_type", attachment?.type != null)
    put("attachment_has_url", attachment?.url != null)
    put("attachment_has_app_url", attachment?.appUrl != null)
    put("attachment_has_content", attachment?.content != null)
    put("attachment_has_description", attachment?.description != null)
    put("attachment_filename", attachment?.filename ?: "")
    put("attachment_content_type", attachment?.contentType ?: "")
    put("attachment_byte_size", attachment?.byteSize ?: 0L)
    put("attachment_previewable", attachment?.previewable ?: false)
    // Float-spelled on the wire (1920.0); FlexibleIntSerializer narrows it. A
    // plain Int property throws here.
    put("attachment_width", attachment?.width ?: 0)
    put("attachment_height", attachment?.height ?: 0)

    // The chat upload line: a bespoke six-key attachments aggregate carrying
    // title/url and NONE of the rich-text id/sgid/preview keys.
    put("upload_line_type", uploadLine?.type ?: "")
    put("upload_boosts_count", uploadLine?.boostsCount ?: 0)
    put("upload_attachment_filename", uploadAttachment?.filename ?: "")
    put("upload_attachment_has_title", uploadAttachment?.title != null)
    put("upload_attachment_has_id", uploadAttachment?.id != null)
    put("upload_attachment_has_sgid", uploadAttachment?.sgid != null)

    // The gauge needle: the same attachments key carrying the OTHER variant —
    // the rich-text one, with id and sgid populated.
    put("needle_type", needle?.type ?: "")
    put("needle_color", needle?.color ?: "")
    put("needle_position", needle?.position ?: 0)
    put("needle_comments_count", needle?.commentsCount ?: 0)
    put("needle_comment_count", needle?.commentCount ?: 0)
    put("needle_boosts_count", needle?.boostsCount ?: 0)
    put("needle_attachment_has_id", needleAttachment?.id != null)
    put("needle_attachment_has_sgid", needleAttachment?.sgid != null)
    put("needle_attachment_width", needleAttachment?.width ?: 0)

    // The kanban list: list-partial keys over the envelope, on_hold nested, and
    // a color emitted unconditionally with a null value.
    put("kanban_type", kanban?.type ?: "")
    put("kanban_position", kanban?.position ?: 0)
    put("kanban_cards_count", kanban?.cardsCount ?: 0)
    put("kanban_comment_count", kanban?.commentCount ?: 0)
    put("kanban_subscriber_count", kanban?.subscribers?.size ?: 0)
    put("kanban_has_color", kanban?.color != null)
    put("kanban_has_on_hold", kanban?.onHold != null)
    put("kanban_on_hold_cards_count", kanban?.onHold?.cardsCount ?: 0)
}


fun main() {
    val testsDir = File("../conformance/tests")

    // Case census (#602) — see CaseCensus. Taken up front, by its own walk, so a
    // fixture tree this runner's listFiles cannot see is reported before the run
    // rather than inferred from a short count afterwards.
    val expectedCases = try {
        CaseCensus.nonLiveCaseCount(testsDir)
    } catch (e: CaseCensus.CensusException) {
        System.err.println("Error taking fixture census: ${e.message}")
        System.exit(1)
        return
    }

    // No early return on an empty listing. The census walks recursively and
    // this listing does not, so "the census found fixtures but this runner
    // listed none" is exactly the nested-fixture under-count the census exists
    // to reject — and returning success here would step over the comparison
    // that rejects it. Falling through runs zero cases and lets the count check
    // fail, which is the correct answer.
    val testFiles = testsDir.listFiles { f -> f.extension == "json" }
        ?.sorted()
        .orEmpty()

    if (testFiles.isEmpty()) {
        println("No test files found in ${testsDir.absolutePath}")
    }

    val json = Json { ignoreUnknownKeys = true }
    var passed = 0
    var failed = 0
    var skipped = 0
    // Recorded from the same branches that increment `skipped`, so the manifest
    // cannot claim a different set than the run took. All THREE exclusion paths
    // record: the tag branch, the named roster, and a runtime skip.
    val excluded = mutableListOf<ExecutionManifest.Exclusion>()

    for (file in testFiles) {
        // Live tests are TS-only (canonical wire-capturer). Filter them out
        // here so the offline Kotlin runner doesn't see live entries with
        // unresolved ${PROJECT_ID} fixtures or unknown operations.
        val testCases = json.decodeFromString<List<TestCase>>(file.readText())
            .filter { CaseCensus.isMockMode(it.mode) }
        if (testCases.isEmpty()) continue
        println("\n=== ${file.name} ===")

        for (tc in testCases) {
            // The Kotlin SDK auto-paginates list operations (like the TS SDK),
            // so tests that assert requestCount=1 with Link headers are not applicable.
            if ("link-header" in tc.tags) {
                skipped++
                excluded.add(ExecutionManifest.Exclusion(
                    file.name, tc.name,
                    "Kotlin SDK auto-paginates (follows Link headers by design)"))
                println("  SKIP: ${tc.name}")
                println("        Kotlin SDK auto-paginates (follows Link headers by design)")
                continue
            }
            val skipReason = KOTLIN_SKIPS[tc.name]
            if (skipReason != null) {
                skipped++
                excluded.add(ExecutionManifest.Exclusion(file.name, tc.name, skipReason))
                println("  SKIP: ${tc.name}")
                println("        $skipReason")
                continue
            }
            // Note: MissingFieldException from kotlinx.serialization (when mock
            // bodies lack required model fields) is caught at runtime in runTest()
            // and reported as FAIL: Kotlin is the strictest fixture consumer, so
            // an under-specified mock body is a fixture bug to fix, not a reason
            // to silently shed coverage.
            val result = runTest(tc)
            when {
                result.skipped -> {
                    skipped++
                    excluded.add(ExecutionManifest.Exclusion(file.name, tc.name, result.message))
                    println("  SKIP: ${tc.name}")
                    println("        ${result.message}")
                }
                result.passed -> {
                    passed++
                    println("  PASS: ${tc.name}")
                }
                else -> {
                    failed++
                    println("  FAIL: ${tc.name}")
                    println("        ${result.message}")
                }
            }
        }
    }

    println("\n=== Summary ===")
    println(
        "Passed: $passed, Failed: $failed, Skipped: $skipped, Total: ${passed + failed + skipped} " +
            "(fixtures declare $expectedCases non-live case(s))"
    )

    val countFailure = CaseCensus.countFailure(passed + failed + skipped, expectedCases)
    if (countFailure != null) {
        System.err.println("\nFAIL: $countFailure")
    }

    // Written even when the run failed: a failing runner still has a truthful
    // exclusion set, and a missing manifest reads to the gate as "this runner
    // did not report", turning one failure into two.
    var manifestFailure: String? = null
    try {
        ExecutionManifest.write("kotlin", expectedCases, passed + failed, excluded)
    } catch (e: ExecutionManifest.Error) {
        manifestFailure = e.message
        System.err.println("\nFAIL: could not write execution manifest: ${e.message}")
    }

    if (failed > 0 || countFailure != null || manifestFailure != null) {
        System.exit(1)
    }
}

@kotlinx.serialization.Serializable
data class TestCase(
    val name: String,
    val description: String = "",
    val operation: String,
    val method: String = "",
    val path: String = "",
    val pathParams: JsonObject? = null,
    val queryParams: JsonObject? = null,
    val requestBody: JsonObject? = null,
    val mockResponses: List<MockResponse> = emptyList(),
    val assertions: List<Assertion> = emptyList(),
    val tags: List<String> = emptyList(),
    val configOverrides: ConfigOverrides? = null,
    /**
     * Execution mode. Defaults to "mock". Live tests are owned by the TS
     * runner only (canonical wire-capturer); other-language runners filter
     * them out at load time.
     */
    val mode: String = "mock",
)

@kotlinx.serialization.Serializable
data class ConfigOverrides(
    val baseUrl: String? = null,
    val maxPages: Int? = null,
    val maxItems: Int? = null,
    /** Pins the list operation to a single page (SPEC §8). */
    val page: Long? = null,
    /** Overrides the client-wide retry cap as a TOTAL attempt count (SPEC §2). */
    val maxRetries: Int? = null,
)

@kotlinx.serialization.Serializable
data class MockResponse(
    val status: Int? = null,
    val networkError: Boolean = false,
    val headers: Map<String, String> = emptyMap(),
    val body: JsonElement? = null,
    val delay: Int = 0,
)

@kotlinx.serialization.Serializable
data class Assertion(
    val type: String,
    val expected: JsonElement? = null,
    val min: Double = 0.0,
    val max: Double = 0.0,
    val path: String = "",
    /** Request index for per-request assertions (0-based; negative = from end). */
    val index: Int? = null,
)

data class TestResult(
    val passed: Boolean,
    val message: String,
    val skipped: Boolean = false,
)

/** Captures SDK-observed values from a dispatched operation. */
data class DispatchResult(
    /** X-Total-Count as parsed by the SDK into ListResult.meta.totalCount */
    val totalCount: Long? = null,
    /** True when the SDK truncated results (maxPages/maxItems cap hit). */
    val truncated: Boolean? = null,
    /** The deserialized SDK response re-serialized to JSON (for responseBody assertions). */
    val resultJson: JsonElement? = null,
)

private fun runTest(tc: TestCase): TestResult {
    // Defense-in-depth backstop for the operationally-harmful mockResponses
    // shapes: neither mode set (would be served as `status ?: 200`, a false
    // positive) or both active. The AUTHORITATIVE oneOf enforcement is
    // `make conformance-fixtures-check` (check-jsonschema against
    // conformance/schema.json), which runs before the runners and rejects
    // {status, networkError:false} / non-true networkError that this truthiness
    // backstop intentionally lets through for cross-runner parity.
    tc.mockResponses.forEachIndexed { i, mr ->
        if ((mr.status != null) == mr.networkError) {
            return TestResult(false, "mockResponses[$i] must set exactly one of status or networkError (got status=${mr.status}, networkError=${mr.networkError})")
        }
    }

    // Track requests
    val requestCounter = AtomicInteger(0)
    val requestTimes = mutableListOf<Long>()
    val requestPaths = mutableListOf<String>()
    val requestMethods = mutableListOf<String>()
    val requestBodies = mutableListOf<JsonObject?>()
    val requestHeadersList = mutableListOf<Headers>()
    val requestContentTypes = mutableListOf<String?>()
    val responseIndex = AtomicInteger(0)

    // Detect if test uses Link next headers (SDK will auto-paginate)
    val autoPaginates = tc.mockResponses.any { mr ->
        mr.headers.any { (k, v) -> k.equals("Link", ignoreCase = true) && "rel=\"next\"" in v }
    }

    val engine = MockEngine { request ->
        synchronized(requestTimes) {
            requestCounter.incrementAndGet()
            requestTimes.add(System.currentTimeMillis())
            requestPaths.add(request.url.encodedPath)
            requestMethods.add(request.method.value.uppercase())
            requestBodies.add(parseRequestBody(request.body))
            requestHeadersList.add(request.headers)
            requestContentTypes.add(request.body.contentType?.toString())
        }

        val idx = responseIndex.getAndIncrement()
        if (idx >= tc.mockResponses.size) {
            if (autoPaginates) {
                respond(
                    content = "[]",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, "application/json"),
                )
            } else {
                respond(
                    content = """{"error": "No more mock responses"}""",
                    status = HttpStatusCode.InternalServerError,
                    headers = headersOf(HttpHeaders.ContentType, "application/json"),
                )
            }
        } else {
            val mockResp = tc.mockResponses[idx]

            if (mockResp.delay > 0) {
                Thread.sleep(mockResp.delay.toLong())
            }

            // Genuine transport failure for this queued entry: throw from the
            // engine lambda, which the SDK observes as a network error and maps
            // to BasecampException.Network. The request is already counted above.
            if (mockResp.networkError) {
                throw IOException("simulated network error")
            }

            val responseHeaders = HeadersBuilder().apply {
                append(HttpHeaders.ContentType, ContentType.Application.Json.toString())
                for ((key, value) in mockResp.headers) {
                    append(key, value)
                }
            }

            val bodyContent = if (mockResp.body != null) {
                Json.encodeToString(JsonElement.serializer(), normalizeBody(mockResp.body, mockResp.status))
            } else {
                ""
            }

            respond(
                content = bodyContent,
                // networkError entries throw above; the schema guarantees a
                // status on every non-networkError entry.
                status = HttpStatusCode.fromValue(mockResp.status ?: 200),
                headers = responseHeaders.build(),
            )
        }
    }

    // Handle configOverrides.baseUrl for HTTPS enforcement tests
    var caughtException: BasecampException? = null
    var httpStatusCode: Int? = null
    var dispatchResult = DispatchResult()

    // Set when the dispatch failed by ANY mechanism, including a decoder
    // rejection that never becomes a BasecampException. Only `errorRaised`
    // reads it; errorType, errorCode and errorMessage still read
    // caughtException, so a fixture pinning a canonical code cannot be
    // satisfied by a raw kotlinx.serialization throw.
    var dispatchFailed = false
    // A fixture declaring errorRaised is asserting that the body is
    // DELIBERATELY malformed and that refusing it is the behaviour under test
    // (#576). That flips the usual reading of a decoder rejection here: it is
    // the point of the case, not an under-specified mock body to repair.
    val expectsFailure = tc.assertions.any { it.type == "errorRaised" }

    val overrideBaseUrl = tc.configOverrides?.baseUrl

    try {
        val client = BasecampClient {
            accessToken("conformance-test-token")
            baseUrl = overrideBaseUrl ?: "http://localhost:3000"
            this.engine = engine
            tc.configOverrides?.maxPages?.let { maxPages = it }
            // Kotlin's transport floors the cap at one attempt on every path
            // (computeMaxAttempts), so a 0 here is "no retries, exactly one
            // attempt" rather than "no request" — the contract SPEC §2
            // validation step 4 states. enableRetry stays true: the cap is what
            // the fixture is pinning, and routing 0 through the on/off knob
            // instead would test a different mechanism than the one named.
            tc.configOverrides?.maxRetries?.let { maxRetries = it }
        }

        val account = client.forAccount(TEST_ACCOUNT_ID)

        try {
            runBlocking {
                dispatchResult = dispatchOperation(tc, account)
            }
            val lastIdx = responseIndex.get() - 1
            if (lastIdx >= 0 && lastIdx < tc.mockResponses.size) {
                httpStatusCode = tc.mockResponses[lastIdx].status
            }
        } catch (e: BasecampException) {
            // Since #604 the SDK maps a body the model refuses into the SPEC §6
            // malformed-2xx-body shape — a statusless Api over the decoder's own
            // exception — rather than letting it out raw. That arrives here, not
            // in the two arms below, so the policy is re-applied at this seam:
            // routing it onward would both lose the loud "fix the fixture body"
            // failure and let a fixture pinning `errorCode: api_error` be
            // satisfied by a decoder rejection, which is exactly what
            // `caughtException` is withheld to prevent.
            //
            // The SDK is asked which shape this is rather than told: see
            // malformedBodyFailure (MalformedBody.kt) for why inspecting `cause`
            // is not the same question, and for why the branch lives there.
            val decodeFailure = malformedBodyFailure(e)
            if (decodeFailure != null) {
                if (!expectsFailure) {
                    client.close()
                    return TestResult(passed = false, message = decodeFailureMessage(decodeFailure))
                }
                dispatchFailed = true
            } else {
                caughtException = e
                dispatchFailed = true
                httpStatusCode = e.httpStatus
            }
        } catch (e: MissingFieldException) {
            // Reached only by a decode the SDK's primitives do not own — the
            // wrapper-field decode a generated wrapped-list method runs after
            // the primitive returns. Everything the primitives decode arrives
            // above, and both paths apply the same policy.
            //
            // A mock body that fails the model's required-field validation is a
            // fixture bug, not a runner limitation: fail loudly so it gets fixed
            // (canonical bodies live in spec/fixtures/) instead of silently
            // degrading coverage. Was SKIP until every rider was repaired.
            if (!expectsFailure) {
                client.close()
                return TestResult(passed = false, message = decodeFailureMessage(e))
            }
            dispatchFailed = true
        } catch (e: SerializationException) {
            // A wrong-TYPED field, as opposed to a missing one. Same policy:
            // normally a fixture bug, but the refusal itself when the fixture
            // declares errorRaised. This is how Kotlin satisfies the #576 kill
            // cases — kotlinx.serialization is its guard, where TypeScript,
            // Python and Ruby need a hand-written one.
            if (!expectsFailure) {
                client.close()
                return TestResult(passed = false, message = decodeFailureMessage(e))
            }
            dispatchFailed = true
        } catch (e: Exception) {
            client.close()
            return TestResult(false, "Unexpected exception: ${e::class.simpleName}: ${e.message}")
        }

        client.close()
    } catch (e: IllegalArgumentException) {
        // SDK's require() throws IllegalArgumentException for HTTPS enforcement.
        // Map to BasecampException.Usage for assertion compatibility.
        caughtException = BasecampException.Usage(e.message ?: "HTTPS required")
        dispatchFailed = true
    }

    // Run assertions
    val requestCount = requestCounter.get()

    // Implicit method invariant: MockEngine answers any verb, so a
    // wrong-verb request (e.g. a PUT regressing to POST) would consume a
    // queued response silently. When the fixture declares a method and
    // carries no explicit requestMethod assertions, the first request must
    // use the fixture method.
    val fixtureMethod = tc.method.uppercase()
    if (fixtureMethod.isNotEmpty() && tc.assertions.none { it.type == "requestMethod" } &&
        requestMethods.isNotEmpty() && requestMethods[0] != fixtureMethod
    ) {
        return TestResult(false, "Expected first request method $fixtureMethod, got ${requestMethods[0]}")
    }

    for (assertion in tc.assertions) {
        when (assertion.type) {
            "requestCount" -> {
                val expected = assertion.expected?.asInt()
                    ?: return TestResult(false, "requestCount assertion missing expected value")
                checkRequestCount(requestCount, expected)?.let { return TestResult(false, it) }
            }

            "statusCode" -> {
                val expected = assertion.expected?.asInt()
                    ?: return TestResult(false, "statusCode assertion missing expected value")
                val actual = httpStatusCode
                if (actual == null) {
                    return TestResult(false, "Expected status code $expected, but got no response")
                }
                if (actual != expected) {
                    return TestResult(false, "Expected status code $expected, got $actual")
                }
            }

            "responseStatus" -> {
                val expected = assertion.expected?.asInt()
                    ?: return TestResult(false, "responseStatus assertion missing expected value")
                val actual = httpStatusCode
                if (actual == null) {
                    return TestResult(false, "Expected response status $expected, but got no response")
                }
                if (actual != expected) {
                    return TestResult(false, "Expected response status $expected, got $actual")
                }
            }

            "responseBody" -> {
                val fieldPath = assertion.path
                val resultElement = dispatchResult.resultJson
                    ?: return TestResult(false, "responseBody.$fieldPath: no result captured from operation")
                val actual = navigateJsonPath(resultElement, fieldPath)
                    ?: return TestResult(false, "responseBody.$fieldPath: field not found in result")
                val result = compareJsonValues("responseBody.$fieldPath", assertion.expected, actual)
                if (result != null) return result
            }

            "noError" -> {
                if (caughtException != null) {
                    return TestResult(false, "Expected no error, got: ${caughtException.message}")
                }
            }

            // The inverse of noError, and deliberately code-agnostic. See
            // errorRaisedFailure (ErrorRaised.kt) for the contract and for why
            // the branch lives there rather than inline: no committed fixture
            // can reach its failing side, so it is unit-tested instead.
            //
            // Read from BOTH signals: every path that records caughtException
            // also sets dispatchFailed today, and the union keeps that true by
            // construction rather than by call-site discipline.
            "errorRaised" -> {
                errorRaisedFailure(dispatchFailed || caughtException != null)?.let {
                    return TestResult(false, it)
                }
            }

            "requestPath" -> {
                val requestIndex = assertion.index ?: 0
                val expected = assertion.expected?.asString()
                    ?: return TestResult(false, "requestPath assertion missing expected value")
                val idx = resolveRequestIndex(requestIndex, requestPaths.size)
                    ?: return TestResult(false, "requestPath[$requestIndex]: no request recorded at that index (${requestPaths.size} requests)")
                if (requestPaths[idx] != expected) {
                    return TestResult(false, "Expected request path \"$expected\" at index $requestIndex, got \"${requestPaths[idx]}\"")
                }
            }

            "requestMethod" -> {
                val requestIndex = assertion.index ?: 0
                val expected = assertion.expected?.asString()?.uppercase()
                    ?: return TestResult(false, "requestMethod assertion missing expected value")
                val idx = resolveRequestIndex(requestIndex, requestMethods.size)
                    ?: return TestResult(false, "requestMethod[$requestIndex]: no request recorded at that index (${requestMethods.size} requests)")
                if (requestMethods[idx] != expected) {
                    return TestResult(false, "Expected request method $expected at index $requestIndex, got ${requestMethods[idx]}")
                }
            }

            "requestBody" -> {
                val requestIndex = assertion.index ?: 0
                val key = assertion.path
                val idx = resolveRequestIndex(requestIndex, requestBodies.size)
                    ?: return TestResult(false, "requestBody.$key[$requestIndex]: no request recorded at that index (${requestBodies.size} requests)")
                val body = requestBodies[idx]
                    ?: return TestResult(false, "requestBody.$key[$requestIndex]: request has no JSON body")
                val actual = navigateJsonPath(body, key)
                    ?: return TestResult(false, "requestBody.$key[$requestIndex]: key not present in request body")
                val result = compareJsonValues("requestBody.$key[$requestIndex]", assertion.expected, actual)
                if (result != null) return result
            }

            "requestBodyAbsent" -> {
                val requestIndex = assertion.index ?: 0
                val key = assertion.path
                val idx = resolveRequestIndex(requestIndex, requestBodies.size)
                    ?: return TestResult(false, "requestBodyAbsent.$key[$requestIndex]: no request recorded at that index (${requestBodies.size} requests)")
                val body = requestBodies[idx]
                if (body != null && navigateJsonPath(body, key) != null) {
                    return TestResult(false, "requestBodyAbsent.$key[$requestIndex]: key unexpectedly present in request body")
                }
            }

            "delayBetweenRequests" -> {
                // Not all gaps are retry gaps — the download flow's final gap
                // is the redirect hop to the signed URL, which is deliberately
                // un-delayed — so those fixtures name a gap with an index. See
                // checkDelayGaps for the contract.
                checkDelayGaps(requestTimes, assertion.min.toLong(), assertion.index)
                    ?.let { return TestResult(false, it) }
            }

            "headerValue" -> {
                val headerName = assertion.path
                val expected = assertion.expected?.asString()
                    ?: return TestResult(false, "headerValue assertion missing expected value")
                when (headerName.lowercase()) {
                    "x-total-count" -> {
                        val actual = dispatchResult.totalCount?.toString()
                        if (actual != expected) {
                            return TestResult(false, "SDK meta.totalCount: expected $expected, got $actual")
                        }
                    }
                    else -> {
                        if (tc.mockResponses.isEmpty()) {
                            return TestResult(false, "Expected response header $headerName=$expected, but no mock responses defined")
                        }
                        val actual = tc.mockResponses[0].headers[headerName]
                        if (actual != expected) {
                            return TestResult(false, "Expected response header $headerName=$expected, got $actual")
                        }
                    }
                }
            }

            "errorType" -> {
                val expectedType = assertion.expected?.asString()
                    ?: return TestResult(false, "errorType assertion missing expected value")
                if (caughtException == null) {
                    return TestResult(false, "Expected error type \"$expectedType\", but got no error")
                }
                val codeMap = mapOf(
                    "not_found" to BasecampException.CODE_NOT_FOUND,
                    "auth_required" to BasecampException.CODE_AUTH,
                    "forbidden" to BasecampException.CODE_FORBIDDEN,
                    "rate_limit" to BasecampException.CODE_RATE_LIMIT,
                    "validation" to BasecampException.CODE_VALIDATION,
                    "api_error" to BasecampException.CODE_API,
                    "usage" to BasecampException.CODE_USAGE,
                    "network" to BasecampException.CODE_NETWORK,
                )
                val expectedCode = codeMap[expectedType]
                if (expectedCode == null) {
                    return TestResult(false, "Unknown conformance error type \"$expectedType\" (add to codeMap)")
                }
                if (caughtException.code != expectedCode) {
                    return TestResult(false, "Expected error code \"$expectedCode\", got \"${caughtException.code}\"")
                }
            }

            "errorCode" -> {
                val expected = assertion.expected?.asString()
                    ?: return TestResult(false, "errorCode assertion missing expected value")
                if (caughtException == null) {
                    return TestResult(false, "Expected error code \"$expected\", but got no error")
                }
                if (caughtException.code != expected) {
                    return TestResult(false, "Expected error code \"$expected\", got \"${caughtException.code}\"")
                }
            }

            "errorMessage" -> {
                val expected = assertion.expected?.asString()
                    ?: return TestResult(false, "errorMessage assertion missing expected value")
                if (caughtException == null) {
                    return TestResult(false, "Expected error message containing \"$expected\", but got no error")
                }
                if (expected !in (caughtException.message ?: "")) {
                    return TestResult(false, "Expected error message containing \"$expected\", got \"${caughtException.message}\"")
                }
            }

            "errorField" -> {
                val fieldPath = assertion.path
                if (caughtException == null) {
                    return TestResult(false, "Expected error field $fieldPath, but got no error")
                }
                val actual: Any? = when (fieldPath) {
                    "httpStatus" -> caughtException.httpStatus
                    "retryable" -> caughtException.retryable
                    "code" -> caughtException.code
                    "message" -> caughtException.message
                    "requestId" -> caughtException.requestId
                    else -> return TestResult(false, "Unknown error field: $fieldPath")
                }
                val result = compareValues("error.$fieldPath", assertion.expected, actual)
                if (result != null) return result
            }

            "headerInjected" -> {
                val headerName = assertion.path
                val expected = assertion.expected?.asString()
                    ?: return TestResult(false, "headerInjected assertion missing expected value")
                if (requestHeadersList.isEmpty()) {
                    return TestResult(false, "Expected header $headerName=\"$expected\", but no requests were recorded")
                }
                var actual = requestHeadersList[0][headerName]
                // Ktor stores Content-Type on the body OutgoingContent, not in headers
                if (actual == null && headerName.equals("Content-Type", ignoreCase = true)) {
                    actual = requestContentTypes.firstOrNull()
                }
                // Content-Type may include charset (e.g., "application/json; charset=UTF-8")
                val matches = if (headerName.equals("Content-Type", ignoreCase = true)) {
                    actual != null && actual.startsWith(expected, ignoreCase = true)
                } else {
                    actual == expected
                }
                if (!matches) {
                    return TestResult(false, "Expected header $headerName=\"$expected\", got \"$actual\"")
                }
            }

            "headerPresent" -> {
                val requestIndex = assertion.index ?: 0
                val headerName = assertion.path
                val idx = resolveRequestIndex(requestIndex, requestHeadersList.size)
                    ?: return TestResult(false, "headerPresent $headerName[$requestIndex]: no request recorded at that index (${requestHeadersList.size} requests)")
                val actual = requestHeadersList[idx][headerName]
                if (actual.isNullOrEmpty()) {
                    return TestResult(false, "Expected header $headerName present on request index $idx, but it was empty or missing")
                }
            }

            "headerAbsent" -> {
                val requestIndex = assertion.index ?: 0
                val headerName = assertion.path
                val idx = resolveRequestIndex(requestIndex, requestHeadersList.size)
                    ?: return TestResult(false, "headerAbsent $headerName[$requestIndex]: no request recorded at that index (${requestHeadersList.size} requests)")
                // Use getAll (not indexed get): a present-but-empty header must
                // fail an absence assertion, same as the Go runner's Values check.
                val values = requestHeadersList[idx].getAll(headerName)
                if (!values.isNullOrEmpty()) {
                    return TestResult(false, "Expected header $headerName absent on request index $idx, got $values")
                }
            }

            "requestScheme" -> {
                val expected = assertion.expected?.asString()
                if (expected == "https" && caughtException == null) {
                    return TestResult(false, "Expected HTTPS enforcement error, but request succeeded over HTTP")
                }
            }

            "urlOrigin" -> {
                val expected = assertion.expected?.asString()
                if (expected == "rejected" && requestCount > 1) {
                    return TestResult(false, "Expected cross-origin URL rejection (1 request), but $requestCount requests were made")
                }
            }

            "responseMeta" -> {
                val fieldPath = assertion.path
                val actual: Any? = when (fieldPath) {
                    "totalCount" -> dispatchResult.totalCount
                    "truncated" -> dispatchResult.truncated
                    else -> return TestResult(false, "Unknown response meta field: $fieldPath")
                }
                val result = compareValues("meta.$fieldPath", assertion.expected, actual)
                if (result != null) return result
            }

            else -> {
                return TestResult(false, "Unknown assertion type: ${assertion.type}")
            }
        }
    }

    return TestResult(true, "All assertions passed")
}

/** Compare an expected JSON value against an actual Kotlin value. */
private fun compareValues(label: String, expected: JsonElement?, actual: Any?): TestResult? {
    if (expected == null) return TestResult(false, "$label: expected value is null in assertion")
    when (expected) {
        is JsonPrimitive -> {
            if (expected.isString) {
                val exp = expected.content
                if (actual?.toString() != exp) {
                    return TestResult(false, "Expected $label = \"$exp\", got \"$actual\"")
                }
            } else if (expected.booleanOrNull != null) {
                val exp = expected.boolean
                if (actual != exp) {
                    return TestResult(false, "Expected $label = $exp, got $actual")
                }
            } else {
                val expInt = expected.intOrNull
                if (expInt != null) {
                    val actualInt = when (actual) {
                        is Int -> actual
                        is Long -> actual.toInt()
                        is Number -> actual.toInt()
                        else -> null
                    }
                    if (actualInt != expInt) {
                        return TestResult(false, "Expected $label = $expInt, got $actual")
                    }
                } else {
                    val expLong = expected.longOrNull
                    if (expLong != null) {
                        val actualLong = when (actual) {
                            is Long -> actual
                            is Int -> actual.toLong()
                            is Number -> actual.toLong()
                            else -> null
                        }
                        if (actualLong != expLong) {
                            return TestResult(false, "Expected $label = $expLong, got $actual")
                        }
                    }
                }
            }
        }
        else -> {
            if (actual?.toString() != expected.toString()) {
                return TestResult(false, "Expected $label = $expected, got $actual")
            }
        }
    }
    return null
}

/**
 * Dispatches the test operation against the SDK and returns observed metadata.
 */
private suspend fun dispatchOperation(tc: TestCase, account: AccountClient): DispatchResult {
    return when (tc.operation) {
        "ListProjects" -> {
            val maxItems = tc.configOverrides?.maxItems
            val page = tc.configOverrides?.page
            val opts = if ((maxItems != null && maxItems > 0) || (page != null && page > 0)) {
                ListProjectsOptions(maxItems = maxItems, page = page)
            } else null
            val result = account.projects.list(opts)
            DispatchResult(
                totalCount = result.meta.totalCount,
                truncated = result.meta.truncated,
                resultJson = summarizeProjects(result),
            )
        }

        "Search" -> {
            val result = account.search.search(q = SEARCH_QUERY)
            DispatchResult(resultJson = summarizeSearch(result))
        }

        "GetProject" -> {
            val projectId = tc.pathParams.longParam("projectId")
            val project = account.projects.get(projectId)
            val resultJson = Json.encodeToJsonElement(Project.serializer(), project)
            DispatchResult(resultJson = resultJson)
        }

        "ListRecentProjects" -> {
            val recentProjects = account.projects.listRecentProjects()
            DispatchResult(resultJson = summarizeProjects(recentProjects))
        }

        "RecordProjectVisit" -> {
            account.projects.recordProjectVisit(tc.pathParams.longParam("projectId"))
            DispatchResult()
        }

        "GetTemplateLibrary" -> {
            DispatchResult(resultJson = account.templates.getLibrary())
        }

        "CreateTemplateLibraryCopy" -> {
            val rb = tc.requestBody
            val libraryCopy = account.templates.createLibraryCopy(
                CreateTemplateLibraryCopyBody(
                    templateRecordingId = rb.longParam("template_recording_id"),
                    destinationParentId = rb.longParam("destination_parent_id"),
                    addingPeopleConfirmed = rb?.get("adding_people_confirmed")?.jsonPrimitive?.booleanOrNull,
                ),
            )
            DispatchResult(resultJson = libraryCopy)
        }

        "GetTemplateLibraryCopy" -> {
            DispatchResult(resultJson = account.templates.getLibraryCopy(tc.pathParams.longParam("copyId")))
        }

        "CreateProject" -> {
            val name = tc.requestBody.stringParam("name")
            account.projects.create(CreateProjectBody(name = name))
            DispatchResult()
        }

        "UpdateProject" -> {
            val projectId = tc.pathParams.longParam("projectId")
            val name = tc.requestBody.stringParam("name")
            account.projects.update(projectId, UpdateProjectBody(name = name))
            DispatchResult()
        }

        "TrashProject" -> {
            val projectId = tc.pathParams.longParam("projectId")
            account.projects.trash(projectId)
            DispatchResult()
        }

        "ListTodos" -> {
            val todolistId = tc.pathParams.longParam("todolistId")
            val result = account.todos.list(todolistId)
            DispatchResult(totalCount = result.meta.totalCount, truncated = result.meta.truncated)
        }

        "UpdateTodo" -> {
            val todoId = tc.pathParams.longParam("todoId")
            val rb = tc.requestBody
            account.todos.update(todoId, UpdateTodoBody(
                content = rb?.get("content")?.jsonPrimitive?.contentOrNull,
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
                assigneeIds = rb?.get("assignee_ids")?.jsonArray?.map { it.jsonPrimitive.long },
                completionSubscriberIds = rb?.get("completion_subscriber_ids")?.jsonArray?.map { it.jsonPrimitive.long },
                notify = rb?.get("notify")?.jsonPrimitive?.booleanOrNull,
                dueOn = rb?.get("due_on")?.jsonPrimitive?.contentOrNull,
                startsOn = rb?.get("starts_on")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        // Raw single PUT, no read-before-write. Presence-bearing: only the keys
        // the fixture carries may reach the wire. participant_ids, url and
        // highlighted are the operation's preservedOnOmission carve-out, so an
        // absent key must not become [] / "" / false on the wire — that would
        // clear the value BC3 is holding — while an explicit empty must be sent.
        // `url`, `highlighted` and `status` are the three #641 members. The
        // write spelling is `url`; `join_url` is read-only and BC3 drops it from
        // a write body without complaining.
        "CreateScheduleEntry" -> {
            val scheduleId = tc.pathParams.longParam("scheduleId")
            val rb = tc.requestBody
            account.schedules.createEntry(scheduleId, CreateScheduleEntryBody(
                summary = tc.requestBody.stringParam("summary"),
                startsAt = tc.requestBody.stringParam("starts_at"),
                endsAt = tc.requestBody.stringParam("ends_at"),
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
                participantIds = rb?.get("participant_ids")?.jsonArray
                    ?.map { element -> element.jsonPrimitive.long },
                allDay = rb?.get("all_day")?.jsonPrimitive?.booleanOrNull,
                notify = rb?.get("notify")?.jsonPrimitive?.booleanOrNull,
                url = rb?.get("url")?.jsonPrimitive?.contentOrNull,
                highlighted = rb?.get("highlighted")?.jsonPrimitive?.booleanOrNull,
                status = rb?.get("status")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        "ReplaceScheduleEntry" -> {
            val entryId = tc.pathParams.longParam("entryId")
            val rb = tc.requestBody
            account.schedules.replaceEntry(entryId, ReplaceScheduleEntryBody(
                summary = rb?.get("summary")?.jsonPrimitive?.contentOrNull,
                startsAt = tc.requestBody.stringParam("starts_at"),
                endsAt = tc.requestBody.stringParam("ends_at"),
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
                participantIds = rb?.get("participant_ids")?.jsonArray
                    ?.map { element -> element.jsonPrimitive.long },
                allDay = rb?.get("all_day")?.jsonPrimitive?.booleanOrNull,
                notify = rb?.get("notify")?.jsonPrimitive?.booleanOrNull,
                url = rb?.get("url")?.jsonPrimitive?.contentOrNull,
                highlighted = rb?.get("highlighted")?.jsonPrimitive?.booleanOrNull,
            ))
            DispatchResult()
        }

        // Synthetic scenario key (not a wire operation): the merge-safe
        // composite, GET then a full PUT of the five full-state fields plus
        // whichever carve-outs the caller addressed. A null argument is "not
        // addressed"; an explicit "", emptyList() or false is an address.
        "UpdateScheduleEntry" -> {
            val entryId = tc.pathParams.longParam("entryId")
            val rb = tc.requestBody
            account.schedules.updateEntry(
                entryId,
                summary = rb?.get("summary")?.jsonPrimitive?.contentOrNull,
                startsAt = rb?.get("starts_at")?.jsonPrimitive?.contentOrNull,
                endsAt = rb?.get("ends_at")?.jsonPrimitive?.contentOrNull,
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
                allDay = rb?.get("all_day")?.jsonPrimitive?.booleanOrNull,
                participantIds = rb?.get("participant_ids")?.jsonArray
                    ?.map { element -> element.jsonPrimitive.long },
                url = rb?.get("url")?.jsonPrimitive?.contentOrNull,
                highlighted = rb?.get("highlighted")?.jsonPrimitive?.booleanOrNull,
                notify = rb?.get("notify")?.jsonPrimitive?.booleanOrNull,
            )
            DispatchResult()
        }

        // Synthetic scenario key (not a wire operation): exercises the
        // read-modify-write edit closure by assigning each fixture key onto the
        // same-named ScheduleEntryFields member, so a key the fixture omits is
        // never assigned and the carve-out stays untouched.
        "EditScheduleEntry" -> {
            val entryId = tc.pathParams.longParam("entryId")
            val rb = tc.requestBody
            account.schedules.editEntry(entryId) {
                rb?.get("summary")?.jsonPrimitive?.contentOrNull?.let { summary = it }
                rb?.get("starts_at")?.jsonPrimitive?.contentOrNull?.let { startsAt = it }
                rb?.get("ends_at")?.jsonPrimitive?.contentOrNull?.let { endsAt = it }
                rb?.get("description")?.jsonPrimitive?.contentOrNull?.let { description = it }
                rb?.get("all_day")?.jsonPrimitive?.booleanOrNull?.let { allDay = it }
                rb?.get("participant_ids")?.jsonArray
                    ?.map { element -> element.jsonPrimitive.long }
                    ?.let { participantIds = it }
                rb?.get("notify")?.jsonPrimitive?.booleanOrNull?.let { notify = it }
                rb?.get("url")?.jsonPrimitive?.contentOrNull?.let { url = it }
                rb?.get("highlighted")?.jsonPrimitive?.booleanOrNull?.let { highlighted = it }
            }
            DispatchResult()
        }

        // Merge-safe composite: GET then PUT, resending the fetched due_on.
        "UpdateCard" -> {
            val cardId = tc.pathParams.longParam("cardId")
            val rb = tc.requestBody
            account.cards.update(
                cardId,
                title = rb?.get("title")?.jsonPrimitive?.contentOrNull,
                content = rb?.get("content")?.jsonPrimitive?.contentOrNull,
                dueOn = rb?.get("due_on")?.jsonPrimitive?.contentOrNull,
                assigneeIds = rb?.get("assignee_ids")?.jsonArray?.map { it.jsonPrimitive.long },
            )
            DispatchResult()
        }

        // Raw single PUT, no read-before-write.
        "UpdateCardVerbatim" -> {
            val cardId = tc.pathParams.longParam("cardId")
            val rb = tc.requestBody
            account.cards.updateVerbatim(cardId, UpdateCardBody(
                title = rb?.get("title")?.jsonPrimitive?.contentOrNull,
                content = rb?.get("content")?.jsonPrimitive?.contentOrNull,
                dueOn = rb?.get("due_on")?.jsonPrimitive?.contentOrNull,
                assigneeIds = rb?.get("assignee_ids")?.jsonArray?.map { it.jsonPrimitive.long },
            ))
            DispatchResult()
        }

        // Synthetic scenario key (not a wire operation): exercises the
        // read-modify-write edit closure by assigning each fixture key
        // onto the corresponding TodoFields member.
        "EditTodo" -> {
            val todoId = tc.pathParams.longParam("todoId")
            val rb = tc.requestBody
            account.todos.edit(todoId) {
                rb?.get("content")?.jsonPrimitive?.content?.let { content = it }
                rb?.get("description")?.jsonPrimitive?.content?.let { description = it }
                rb?.get("assignee_ids")?.jsonArray?.let { arr -> assigneeIds = arr.map { it.jsonPrimitive.long } }
                rb?.get("completion_subscriber_ids")?.jsonArray?.let { arr -> completionSubscriberIds = arr.map { it.jsonPrimitive.long } }
                rb?.get("due_on")?.jsonPrimitive?.content?.let { dueOn = it }
                rb?.get("starts_on")?.jsonPrimitive?.content?.let { startsOn = it }
                rb?.get("notify")?.jsonPrimitive?.booleanOrNull?.let { notify = it }
            }
            DispatchResult()
        }

        "ReplaceTodo" -> {
            val todoId = tc.pathParams.longParam("todoId")
            val rb = tc.requestBody
            account.todos.replace(todoId, ReplaceTodoBody(
                content = tc.requestBody.stringParam("content"),
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
                assigneeIds = rb?.get("assignee_ids")?.jsonArray?.map { it.jsonPrimitive.long },
                completionSubscriberIds = rb?.get("completion_subscriber_ids")?.jsonArray?.map { it.jsonPrimitive.long },
                notify = rb?.get("notify")?.jsonPrimitive?.booleanOrNull,
                dueOn = rb?.get("due_on")?.jsonPrimitive?.contentOrNull,
                startsOn = rb?.get("starts_on")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        // Decode-only read of the polymorphic route. The result is the decoded
        // Todolist re-serialized, so the fixture's responseBody assertions
        // (name, description, type, groups_url / group_position_url) read the
        // SDK's own model rather than the raw body: a decoder that yields
        // nothing fails here instead of silently returning it.
        //
        // One arm, both variants. BC3 renders a group through
        // todolists/_todolist.json.jbuilder, so a group IS a Todolist —
        // discriminated by group_position_url standing in for groups_url,
        // never by the type string, which is "Todolist" either way (#544).
        "GetTodolistOrGroup" -> {
            val id = tc.pathParams.longParam("id")
            val todolist = account.todolists.get(id)
            DispatchResult(resultJson = Json.encodeToJsonElement(Todolist.serializer(), todolist))
        }

        // The group list decodes into an array of that same flat shape.
        // Dispatch convention (documented in the fixture): the runner returns
        // the FIRST decoded element as the result, so the responseBody
        // assertions read element 0. An empty decode has no element 0 — fail
        // loudly rather than report a missing field.
        "ListTodolistGroups" -> {
            val todolistId = tc.pathParams.longParam("todolistId")
            val groups = account.todolistGroups.list(todolistId)
            val first = groups.firstOrNull()
                ?: error(
                    "ListTodolistGroups decoded an empty list; the responseBody " +
                        "assertions read the first element"
                )
            DispatchResult(
                totalCount = groups.meta.totalCount,
                truncated = groups.meta.truncated,
                resultJson = Json.encodeToJsonElement(Todolist.serializer(), first),
            )
        }

        // Synthetic scenario key (not a wire operation, which is
        // UpdateTodolistOrGroup): the merge-safe composite. GET then PUT,
        // resending the fetched description — a name-only sparse PUT would
        // erase it. The same key covers the todolist-group variant; the
        // composite reads {name, description} out of either projection with
        // no type sniffing.
        "UpdateTodolist" -> {
            val id = tc.pathParams.longParam("id")
            val rb = tc.requestBody
            account.todolists.update(id, UpdateTodolistBody(
                name = rb?.get("name")?.jsonPrimitive?.contentOrNull,
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        // Synthetic scenario key (not a wire operation): exercises the
        // read-modify-write edit closure by assigning each fixture key onto
        // the corresponding TodolistFields member.
        "EditTodolist" -> {
            val id = tc.pathParams.longParam("id")
            val rb = tc.requestBody
            account.todolists.edit(id) {
                rb?.get("name")?.jsonPrimitive?.content?.let { name = it }
                rb?.get("description")?.jsonPrimitive?.content?.let { description = it }
            }
            DispatchResult()
        }

        // Raw single PUT, no read-before-write: name is required, and an
        // omitted description is omitted on the wire (the server clears it).
        "ReplaceTodolist" -> {
            val id = tc.pathParams.longParam("id")
            val rb = tc.requestBody
            account.todolists.replace(id, UpdateTodolistOrGroupBody(
                name = tc.requestBody.stringParam("name"),
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        // Synthetic scenario key (not a wire operation): the merge-safe
        // composite, GET then a full PUT of {title, content}.
        "UpdateDocument" -> {
            val documentId = tc.pathParams.longParam("documentId")
            val rb = tc.requestBody
            account.documents.update(documentId, UpdateDocumentBody(
                title = rb?.get("title")?.jsonPrimitive?.contentOrNull,
                content = rb?.get("content")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        // Synthetic scenario key (not a wire operation): exercises the
        // read-modify-write edit closure by assigning each fixture key onto
        // the corresponding DocumentFields member.
        "EditDocument" -> {
            val documentId = tc.pathParams.longParam("documentId")
            val rb = tc.requestBody
            account.documents.edit(documentId) {
                rb?.get("title")?.jsonPrimitive?.content?.let { title = it }
                rb?.get("content")?.jsonPrimitive?.content?.let { content = it }
            }
            DispatchResult()
        }

        // Raw single PUT, no read-before-write: neither field is required, and
        // an omitted one is omitted on the wire (the server clears it).
        "ReplaceDocument" -> {
            val documentId = tc.pathParams.longParam("documentId")
            val rb = tc.requestBody
            account.documents.replace(documentId, ReplaceDocumentBody(
                title = rb?.get("title")?.jsonPrimitive?.contentOrNull,
                content = rb?.get("content")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        "CreateTodo" -> {
            val todolistId = tc.pathParams.longParam("todolistId")
            val content = tc.requestBody.stringParam("content")
            account.todos.create(todolistId, CreateTodoBody(content = content))
            DispatchResult()
        }

        "CreateTodosetTodo" -> {
            val bucketId = tc.pathParams.longParam("bucketId")
            val todosetId = tc.pathParams.longParam("todosetId")
            val content = tc.requestBody.stringParam("content")
            account.todos.createTodosetTodo(bucketId, todosetId, CreateTodosetTodoBody(content = content))
            DispatchResult()
        }

        "CompleteTodo" -> {
            account.todos.complete(tc.pathParams.longParam("todoId"))
            DispatchResult()
        }

        "Subscribe" -> {
            account.subscriptions.subscribe(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "ListMyBookmarks" -> {
            account.bookmarks.listMyBookmarks()
            DispatchResult()
        }

        "ListMyDrafts" -> {
            account.drafts.listMyDrafts()
            DispatchResult()
        }

        "GetMyNote" -> {
            account.myNotes.getMyNote()
            DispatchResult()
        }

        "PrioritizeAssignment" -> {
            account.myAssignments.prioritizeAssignment(PrioritizeAssignmentBody(id = tc.requestBody.longParam("id")))
            DispatchResult()
        }

        "DeprioritizeAssignment" -> {
            account.myAssignments.deprioritizeAssignment(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "ReorderUpNext" -> {
            account.myAssignments.reorderUpNext(
                ReorderUpNextBody(
                    sourceId = tc.requestBody.longParam("source_id"),
                    position = tc.requestBody.longParam("position").toInt(),
                )
            )
            DispatchResult()
        }

        "GetCalendar" -> {
            account.calendars.getCalendar(tc.pathParams.longParam("calendarId"))
            DispatchResult()
        }

        "UpdateCalendar" -> {
            val cal = tc.requestBody?.get("calendar")?.jsonObject ?: JsonObject(emptyMap())
            account.calendars.updateCalendar(tc.pathParams.longParam("calendarId"), UpdateCalendarBody(calendar = cal))
            DispatchResult()
        }

        "UpdateMyNote" -> {
            val note = tc.requestBody?.get("note")?.jsonObject ?: JsonObject(emptyMap())
            account.myNotes.updateMyNote(UpdateMyNoteBody(note = note))
            DispatchResult()
        }

        "GetBookmark" -> {
            account.bookmarks.getBookmark(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "CreateBookmark" -> {
            account.bookmarks.createBookmark(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "DeleteBookmark" -> {
            account.bookmarks.deleteBookmark(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "CreateBubbleUp" -> {
            val rb = tc.requestBody
            account.bubbleUps.createBubbleUp(
                tc.pathParams.longParam("recordingId"),
                CreateBubbleUpBody(
                    at = rb?.get("at")?.jsonPrimitive?.contentOrNull,
                ),
            )
            DispatchResult()
        }

        "DeleteBubbleUp" -> {
            account.bubbleUps.deleteBubbleUp(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "SpotlightRecording" -> {
            account.recordings.spotlight(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "UnspotlightRecording" -> {
            account.recordings.unspotlight(tc.pathParams.longParam("recordingId"))
            DispatchResult()
        }

        "ListFolders" -> {
            account.folders.listFolders()
            DispatchResult()
        }

        "GetFolder" -> {
            account.folders.getFolder(tc.pathParams.longParam("folderId"))
            DispatchResult()
        }

        "CreateFolder" -> {
            val rb = tc.requestBody
            account.folders.createFolder(
                CreateFolderBody(
                    name = rb?.get("name")?.jsonPrimitive?.contentOrNull,
                    projectIds = rb?.get("project_ids")?.jsonArray?.map { it.jsonPrimitive.long },
                )
            )
            DispatchResult()
        }

        "UpdateFolder" -> {
            account.folders.updateFolder(
                tc.pathParams.longParam("folderId"),
                UpdateFolderBody(name = tc.requestBody.stringParam("name")),
            )
            DispatchResult()
        }

        "DeleteFolder" -> {
            account.folders.deleteFolder(tc.pathParams.longParam("folderId"))
            DispatchResult()
        }

        "GetTimesheetEntry" -> {
            val entryId = tc.pathParams.longParam("timesheetEntryId")
                .let { if (it != 0L) it else tc.pathParams.longParam("entryId") }
            account.timesheets.get(entryId)
            DispatchResult()
        }

        "DestroyTimesheetEntry" -> {
            val entryId = tc.pathParams.longParam("timesheetEntryId")
                .let { if (it != 0L) it else tc.pathParams.longParam("entryId") }
            account.timesheets.destroy(entryId)
            DispatchResult()
        }

        "CreateTimesheetEntry" -> {
            val recordingId = tc.pathParams.longParam("recordingId")
            val date = tc.requestBody.stringParam("date")
            val hours = tc.requestBody.stringParam("hours")
            val description = tc.requestBody?.get("description")?.jsonPrimitive?.contentOrNull
            account.timesheets.create(recordingId,
                CreateTimesheetEntryBody(date = date, hours = hours, description = description))
            DispatchResult()
        }

        "UpdateTimesheetEntry" -> {
            val entryId = tc.pathParams.longParam("entryId")
                .let { if (it != 0L) it else tc.pathParams.longParam("timesheetEntryId") }
            val date = tc.requestBody?.get("date")?.jsonPrimitive?.contentOrNull
            val hours = tc.requestBody?.get("hours")?.jsonPrimitive?.contentOrNull
            val description = tc.requestBody?.get("description")?.jsonPrimitive?.contentOrNull
            account.timesheets.update(entryId,
                UpdateTimesheetEntryBody(date = date, hours = hours, description = description))
            DispatchResult()
        }

        "GetProjectTimeline" -> {
            val projectId = tc.pathParams.longParam("projectId")
            account.timeline.projectTimeline(projectId)
            DispatchResult()
        }

        "GetProgressReport" -> {
            account.reports.progress()
            DispatchResult()
        }

        "GetPersonProgress" -> {
            val personId = tc.pathParams.longParam("personId")
            account.reports.personProgress(personId)
            DispatchResult()
        }

        // The window is fixed here rather than read from the case: no mock
        // runner consumes queryParams, and no assertion type can pin a query
        // string. Both bounds are required, so the call cannot be made without
        // them.
        //
        // Until #635 this operation returned a bare JsonElement, so Kotlin
        // enforced no contract on it at all — a case here would have passed
        // against any body. It now decodes into UpcomingScheduleResult, whose
        // members are @Serializable data classes, so a missing required key
        // raises MissingFieldException and fails the case.
        "GetUpcomingSchedule" -> {
            val upcoming = account.reports.upcoming(UPCOMING_WINDOW_START, UPCOMING_WINDOW_END)
            DispatchResult(resultJson = summarizeUpcoming(upcoming))
        }

        "GetProjectTimesheet" -> {
            val projectId = tc.pathParams.longParam("projectId")
            account.timesheets.forProject(projectId)
            DispatchResult()
        }

        "ListWebhooks" -> {
            val bucketId = tc.pathParams.longParam("bucketId")
            account.webhooks.list(bucketId)
            DispatchResult()
        }

        "CreateWebhook" -> {
            val bucketId = tc.pathParams.longParam("bucketId")
            val payloadUrl = tc.requestBody!!["payload_url"]!!.jsonPrimitive.content
            val types = tc.requestBody!!["types"]!!.jsonArray.map { it.jsonPrimitive.content }
            account.webhooks.create(bucketId,
                CreateWebhookBody(payloadUrl = payloadUrl, types = types))
            DispatchResult()
        }

        "GetTool" -> {
            val toolId = tc.pathParams.longParam("toolId")
            account.tools.get(toolId)
            DispatchResult()
        }

        "CreateTool" -> {
            val bucketId = tc.pathParams!!["bucketId"]!!.jsonPrimitive.long
            val toolType = tc.requestBody!!["tool_type"]!!.jsonPrimitive.content
            val title = tc.requestBody?.get("title")?.jsonPrimitive?.contentOrNull
            account.tools.create(bucketId, CreateToolBody(toolType = toolType, title = title))
            DispatchResult()
        }

        "EnableTool" -> {
            val toolId = tc.pathParams.longParam("toolId")
            account.tools.enable(toolId)
            DispatchResult()
        }

        "GetEverythingMessages" -> {
            account.everything.everythingMessages()
            DispatchResult()
        }

        "GetEverythingComments" -> {
            account.everything.everythingComments()
            DispatchResult()
        }

        "GetEverythingCheckins" -> {
            account.everything.everythingCheckins()
            DispatchResult()
        }

        "GetEverythingForwards" -> {
            account.everything.everythingForwards()
            DispatchResult()
        }

        "GetEverythingFiles" -> {
            account.everything.everythingFiles()
            DispatchResult()
        }

        "GetEverythingOverdueTodos" -> {
            account.everything.everythingOverdueTodos()
            DispatchResult()
        }

        "GetEverythingOverdueCards" -> {
            account.everything.everythingOverdueCards()
            DispatchResult()
        }

        "GetEverythingOpenTodos" -> {
            account.everything.everythingOpenTodos()
            DispatchResult()
        }

        "GetEverythingCompletedTodos" -> {
            account.everything.everythingCompletedTodos()
            DispatchResult()
        }

        "GetEverythingUnassignedTodos" -> {
            account.everything.everythingUnassignedTodos()
            DispatchResult()
        }

        "GetEverythingNoDueDateTodos" -> {
            account.everything.everythingNoDueDateTodos()
            DispatchResult()
        }

        "GetEverythingOpenCards" -> {
            account.everything.everythingOpenCards()
            DispatchResult()
        }

        "GetEverythingCompletedCards" -> {
            account.everything.everythingCompletedCards()
            DispatchResult()
        }

        "GetEverythingUnassignedCards" -> {
            account.everything.everythingUnassignedCards()
            DispatchResult()
        }

        "GetEverythingNoDueDateCards" -> {
            account.everything.everythingNoDueDateCards()
            DispatchResult()
        }

        "GetEverythingNotNowCards" -> {
            account.everything.everythingNotNowCards()
            DispatchResult()
        }

        "DownloadURL" -> {
            // Construct an absolute URL the SDK will accept. downloadURL
            // rewrites the scheme+host to the configured baseUrl, so the
            // synthetic host here is never actually hit — only tc.path
            // matters for mock routing. Same shape as the Go runner.
            account.downloadURL("https://storage.3.basecamp.com" + tc.path)
            DispatchResult()
        }

        "UploadsDownload" -> {
            val uploadId = tc.pathParams.longParam("uploadId")
            account.uploads.download(uploadId)
            DispatchResult()
        }

        // Presence-bearing, like ReplaceScheduleEntry: a key the fixture omits
        // stays null and `?.let` keeps it off the wire, so an unaddressed
        // description carries forward while an explicit "" is sent and clears.
        "CreateUploadVersion" -> {
            val uploadId = tc.pathParams.longParam("uploadId")
            val rb = tc.requestBody
            account.uploads.createVersion(uploadId, CreateUploadVersionBody(
                attachableSgid = tc.requestBody.stringParam("attachable_sgid"),
                baseName = rb?.get("base_name")?.jsonPrimitive?.contentOrNull,
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
                notify = rb?.get("notify")?.jsonPrimitive?.contentOrNull,
                subscriptions = rb?.get("subscriptions")?.jsonArray
                    ?.map { element -> element.jsonPrimitive.long },
            ))
            DispatchResult()
        }

        "UpdateUpload" -> {
            val uploadId = tc.pathParams.longParam("uploadId")
            val rb = tc.requestBody
            account.uploads.update(uploadId, UpdateUploadBody(
                baseName = rb?.get("base_name")?.jsonPrimitive?.contentOrNull,
                description = rb?.get("description")?.jsonPrimitive?.contentOrNull,
            ))
            DispatchResult()
        }

        "ListUploadVersions" -> {
            val uploadId = tc.pathParams.longParam("uploadId")
            val result = account.uploads.listVersions(uploadId)
            // ListResult<T> delegates to List<T>, so it IS the item list.
            DispatchResult(resultJson = summarizeUploadVersions(result))
        }

        "ListForwards" -> {
            val inboxId = tc.pathParams.longParam("inboxId")
            val result = account.forwards.list(inboxId)
            DispatchResult(totalCount = result.meta.totalCount, truncated = result.meta.truncated)
        }

        // #588: nine flat spellings bc3 only draws bucket-scoped. Each pins the
        // bucketId segment on the wire — the segment whose absence made them 404.
        "ListChatbots" -> {
            val result = account.campfires.listChatbots(
                tc.pathParams.longParam("bucketId"),
                tc.pathParams.longParam("campfireId"),
            )
            DispatchResult(totalCount = result.meta.totalCount, truncated = result.meta.truncated)
        }

        "GetChatbot" -> {
            account.campfires.getChatbot(
                tc.pathParams.longParam("bucketId"),
                tc.pathParams.longParam("campfireId"),
                tc.pathParams.longParam("chatbotId"),
            )
            DispatchResult()
        }

        "CreateChatbot" -> {
            account.campfires.createChatbot(
                tc.pathParams.longParam("bucketId"),
                tc.pathParams.longParam("campfireId"),
                CreateChatbotBody(
                    serviceName = tc.requestBody.stringParam("service_name"),
                    commandUrl = tc.requestBody.stringParam("command_url"),
                ),
            )
            DispatchResult()
        }

        "UpdateChatbot" -> {
            account.campfires.updateChatbot(
                tc.pathParams.longParam("bucketId"),
                tc.pathParams.longParam("campfireId"),
                tc.pathParams.longParam("chatbotId"),
                UpdateChatbotBody(
                    serviceName = tc.requestBody.stringParam("service_name"),
                    commandUrl = tc.requestBody.stringParam("command_url"),
                ),
            )
            DispatchResult()
        }

        "DeleteChatbot" -> {
            account.campfires.deleteChatbot(
                tc.pathParams.longParam("bucketId"),
                tc.pathParams.longParam("campfireId"),
                tc.pathParams.longParam("chatbotId"),
            )
            DispatchResult()
        }

        "ListClientApprovals" -> {
            val result = account.clientApprovals.list(tc.pathParams.longParam("bucketId"))
            DispatchResult(totalCount = result.meta.totalCount, truncated = result.meta.truncated)
        }

        "ListClientCorrespondences" -> {
            val result = account.clientCorrespondences.list(tc.pathParams.longParam("bucketId"))
            DispatchResult(totalCount = result.meta.totalCount, truncated = result.meta.truncated)
        }

        "ListClientReplies" -> {
            val result = account.clientReplies.list(
                tc.pathParams.longParam("bucketId"),
                tc.pathParams.longParam("recordingId"),
            )
            DispatchResult(totalCount = result.meta.totalCount, truncated = result.meta.truncated)
        }

        "GetClientReply" -> {
            account.clientReplies.get(
                tc.pathParams.longParam("bucketId"),
                tc.pathParams.longParam("recordingId"),
                tc.pathParams.longParam("replyId"),
            )
            DispatchResult()
        }

        "RepositionTodolistGroup" -> {
            val groupId = tc.pathParams.longParam("groupId")
            account.todolistGroups.reposition(
                groupId,
                RepositionTodolistGroupBody(position = tc.requestBody.longParam("position").toInt()),
            )
            DispatchResult()
        }

        else ->
            throw UnsupportedOperationException("Unknown operation: ${tc.operation}")
    }
}

// --- Helpers ---

/**
 * Normalizes a mock response body for SDK compatibility.
 *
 * Conformance test fixtures may wrap arrays in objects (e.g., `{"projects": [...]}`),
 * but the Kotlin SDK's list operations expect a raw JSON array. When the body is
 * a JSON object with a single key whose value is an array, unwrap it.
 *
 * Success bodies only: an error body with one array-valued key is the unwrapped
 * field map (`{"payload_url": ["is invalid"]}`), and unwrapping it would rewrite
 * the fixture on the wire.
 */
private fun normalizeBody(body: JsonElement, status: Int?): JsonElement {
    if ((status ?: 200) < 400 && body is JsonObject && body.size == 1) {
        val value = body.values.first()
        if (value is JsonArray) return value
    }
    return body
}

/**
 * Resolves a per-request assertion index (0-based; negative = from the end)
 * against the number of recorded requests. Null when out of range.
 */
private fun resolveRequestIndex(index: Int, size: Int): Int? {
    val resolved = if (index < 0) size + index else index
    return if (resolved in 0 until size) resolved else null
}

/** Extracts a request's outgoing content as a parsed JSON object, or null. */
private fun parseRequestBody(body: io.ktor.http.content.OutgoingContent): JsonObject? {
    val text = when (body) {
        is TextContent -> body.text
        is OutgoingContent.ByteArrayContent -> body.bytes().decodeToString()
        else -> null
    }
    return text?.takeIf { it.isNotBlank() }
        ?.let { runCatching { Json.parseToJsonElement(it) }.getOrNull() } as? JsonObject
}

private fun JsonObject?.longParam(key: String): Long {
    if (this == null) return 0L
    val element = this[key] ?: return 0L
    return when (element) {
        is JsonPrimitive -> element.long
        else -> 0L
    }
}

private fun JsonObject?.stringParam(key: String): String {
    if (this == null) return ""
    val element = this[key] ?: return ""
    return when (element) {
        is JsonPrimitive -> element.content
        else -> ""
    }
}

private fun JsonElement.asInt(): Int? = when (this) {
    is JsonPrimitive -> intOrNull ?: longOrNull?.toInt()
    else -> null
}

private fun JsonElement.asString(): String? = when (this) {
    is JsonPrimitive -> content
    else -> null
}

/** Navigate a dot-separated path through a JsonElement. */
private fun navigateJsonPath(element: JsonElement, path: String): JsonElement? {
    var current = element
    for (key in path.split(".")) {
        current = (current as? JsonObject)?.get(key) ?: return null
    }
    return current
}

/** Compare two JsonElements for equality (handles large integers). */
private fun compareJsonValues(label: String, expected: JsonElement?, actual: JsonElement): TestResult? {
    if (expected == null) return TestResult(false, "$label: expected value is null in assertion")
    if (expected is JsonPrimitive && actual is JsonPrimitive) {
        // Compare as long to preserve large integer precision
        val expLong = expected.longOrNull
        val actLong = actual.longOrNull
        if (expLong != null && actLong != null) {
            if (expLong != actLong) {
                return TestResult(false, "Expected $label = $expLong, got $actLong")
            }
            return null
        }
    }
    if (expected != actual) {
        return TestResult(false, "Expected $label = $expected, got $actual")
    }
    return null
}
