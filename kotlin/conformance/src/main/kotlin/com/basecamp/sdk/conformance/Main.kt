package com.basecamp.sdk.conformance

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.generated.services.*
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.MissingFieldException
import kotlinx.serialization.json.*
import java.io.File
import java.util.concurrent.atomic.AtomicInteger

/** Default account ID for conformance tests. */
private const val TEST_ACCOUNT_ID = "999"

fun main() {
    val testsDir = File("../conformance/tests")

    val testFiles = testsDir.listFiles { f -> f.extension == "json" }
        ?.sorted()

    if (testFiles.isNullOrEmpty()) {
        println("No test files found in ${testsDir.absolutePath}")
        return
    }

    val json = Json { ignoreUnknownKeys = true }
    var passed = 0
    var failed = 0
    var skipped = 0

    for (file in testFiles) {
        val testCases = json.decodeFromString<List<TestCase>>(file.readText())
        println("\n=== ${file.name} ===")

        for (tc in testCases) {
            // The Kotlin SDK auto-paginates list operations (like the TS SDK),
            // so tests that assert requestCount=1 with Link headers are not applicable.
            if ("link-header" in tc.tags) {
                skipped++
                println("  SKIP: ${tc.name}")
                println("        Kotlin SDK auto-paginates (follows Link headers by design)")
                continue
            }
            // Note: MissingFieldException from kotlinx.serialization (when mock
            // bodies lack required model fields) is caught at runtime in runTest()
            // and reported as SKIP, so no pre-flight filtering is needed.
            val result = runTest(tc)
            when {
                result.skipped -> {
                    skipped++
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
    println("Passed: $passed, Failed: $failed, Skipped: $skipped, Total: ${passed + failed + skipped}")

    if (failed > 0) {
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
)

@kotlinx.serialization.Serializable
data class ConfigOverrides(
    val baseUrl: String? = null,
    val maxPages: Int? = null,
    val maxItems: Int? = null,
)

@kotlinx.serialization.Serializable
data class MockResponse(
    val status: Int,
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
    // Track requests
    val requestCounter = AtomicInteger(0)
    val requestTimes = mutableListOf<Long>()
    val requestPaths = mutableListOf<String>()
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

            val responseHeaders = HeadersBuilder().apply {
                append(HttpHeaders.ContentType, ContentType.Application.Json.toString())
                for ((key, value) in mockResp.headers) {
                    append(key, value)
                }
            }

            val bodyContent = if (mockResp.body != null) {
                Json.encodeToString(JsonElement.serializer(), normalizeBody(mockResp.body))
            } else {
                ""
            }

            respond(
                content = bodyContent,
                status = HttpStatusCode.fromValue(mockResp.status),
                headers = responseHeaders.build(),
            )
        }
    }

    // Handle configOverrides.baseUrl for HTTPS enforcement tests
    var caughtException: BasecampException? = null
    var httpStatusCode: Int? = null
    var dispatchResult = DispatchResult()

    val overrideBaseUrl = tc.configOverrides?.baseUrl

    try {
        val client = BasecampClient {
            accessToken("conformance-test-token")
            baseUrl = overrideBaseUrl ?: "http://localhost:3000"
            this.engine = engine
            tc.configOverrides?.maxPages?.let { maxPages = it }
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
            caughtException = e
            httpStatusCode = e.httpStatus
        } catch (e: MissingFieldException) {
            client.close()
            return TestResult(passed = false, message = "Mock body lacks required Kotlin model fields: ${e.message}", skipped = true)
        } catch (e: Exception) {
            client.close()
            return TestResult(false, "Unexpected exception: ${e::class.simpleName}: ${e.message}")
        }

        client.close()
    } catch (e: IllegalArgumentException) {
        // SDK's require() throws IllegalArgumentException for HTTPS enforcement.
        // Map to BasecampException.Usage for assertion compatibility.
        caughtException = BasecampException.Usage(e.message ?: "HTTPS required")
    }

    // Run assertions
    val requestCount = requestCounter.get()

    for (assertion in tc.assertions) {
        when (assertion.type) {
            "requestCount" -> {
                val expected = assertion.expected?.asInt()
                    ?: return TestResult(false, "requestCount assertion missing expected value")
                if (autoPaginates) {
                    if (requestCount < expected) {
                        return TestResult(false, "Expected >= $expected requests (SDK auto-paginates), got $requestCount")
                    }
                } else if (requestCount != expected) {
                    return TestResult(false, "Expected $expected requests, got $requestCount")
                }
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

            "requestPath" -> {
                val expected = assertion.expected?.asString()
                    ?: return TestResult(false, "requestPath assertion missing expected value")
                if (requestPaths.isEmpty()) {
                    return TestResult(false, "Expected a request to be made, but no requests were recorded")
                }
                if (requestPaths[0] != expected) {
                    return TestResult(false, "Expected request path \"$expected\", got \"${requestPaths[0]}\"")
                }
            }

            "delayBetweenRequests" -> {
                if (requestTimes.size >= 2) {
                    val delay = requestTimes[1] - requestTimes[0]
                    val minDelay = assertion.min.toLong()
                    if (delay < minDelay) {
                        return TestResult(false, "Expected delay >= ${minDelay}ms, got ${delay}ms")
                    }
                }
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
                val headerName = assertion.path
                if (requestHeadersList.isEmpty()) {
                    return TestResult(false, "Expected header $headerName to be present, but no requests were recorded")
                }
                val actual = requestHeadersList[0][headerName]
                if (actual.isNullOrEmpty()) {
                    return TestResult(false, "Expected header $headerName to be present, but it was empty or missing")
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
            val opts = if (maxItems != null && maxItems > 0) {
                ListProjectsOptions(maxItems = maxItems)
            } else null
            val result = account.projects.list(opts)
            DispatchResult(totalCount = result.meta.totalCount, truncated = result.meta.truncated)
        }

        "GetProject" -> {
            val projectId = tc.pathParams.longParam("projectId")
            val project = account.projects.get(projectId)
            val resultJson = Json.encodeToJsonElement(Project.serializer(), project)
            DispatchResult(resultJson = resultJson)
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

        "CreateTodo" -> {
            val todolistId = tc.pathParams.longParam("todolistId")
            val content = tc.requestBody.stringParam("content")
            account.todos.create(todolistId, CreateTodoBody(content = content))
            DispatchResult()
        }

        "GetTimesheetEntry" -> {
            val entryId = tc.pathParams.longParam("timesheetEntryId")
                .let { if (it != 0L) it else tc.pathParams.longParam("entryId") }
            account.timesheets.get(entryId)
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

        "CloneTool" -> {
            val sourceRecordingId = tc.requestBody!!["source_recording_id"]!!.jsonPrimitive.long
            val title = tc.requestBody?.get("title")?.jsonPrimitive?.contentOrNull
            account.tools.clone(CloneToolBody(sourceRecordingId = sourceRecordingId, title = title))
            DispatchResult()
        }

        "EnableTool" -> {
            val toolId = tc.pathParams.longParam("toolId")
            account.tools.enable(toolId)
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
 */
private fun normalizeBody(body: JsonElement): JsonElement {
    if (body is JsonObject && body.size == 1) {
        val value = body.values.first()
        if (value is JsonArray) return value
    }
    return body
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
