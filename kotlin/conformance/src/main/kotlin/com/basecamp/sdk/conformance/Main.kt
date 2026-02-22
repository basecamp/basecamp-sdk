package com.basecamp.sdk.conformance

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.*
import com.basecamp.sdk.generated.services.*
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.runBlocking
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
)

private fun runTest(tc: TestCase): TestResult {
    // Track requests
    val requestCounter = AtomicInteger(0)
    val requestTimes = mutableListOf<Long>()
    val requestPaths = mutableListOf<String>()
    val responseIndex = AtomicInteger(0)

    val engine = MockEngine { request ->
        synchronized(requestTimes) {
            requestCounter.incrementAndGet()
            requestTimes.add(System.currentTimeMillis())
            requestPaths.add(request.url.encodedPath)
        }

        val idx = responseIndex.getAndIncrement()
        if (idx >= tc.mockResponses.size) {
            respond(
                content = """{"error": "No more mock responses"}""",
                status = HttpStatusCode.InternalServerError,
                headers = headersOf(HttpHeaders.ContentType, "application/json"),
            )
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

    val client = BasecampClient {
        accessToken("test-token")
        baseUrl = "http://localhost:3000"
        this.engine = engine
    }

    val account = client.forAccount(TEST_ACCOUNT_ID)

    var caughtException: BasecampException? = null
    var httpStatusCode: Int? = null
    var dispatchResult = DispatchResult()

    try {
        runBlocking {
            dispatchResult = dispatchOperation(tc, account)
        }
        // If we got here with mock responses, the last consumed response's status is the one
        // that succeeded. For successful responses, set the status from the mock.
        val lastIdx = responseIndex.get() - 1
        if (lastIdx >= 0 && lastIdx < tc.mockResponses.size) {
            httpStatusCode = tc.mockResponses[lastIdx].status
        }
    } catch (e: BasecampException) {
        caughtException = e
        httpStatusCode = e.httpStatus
    } catch (e: Exception) {
        client.close()
        return TestResult(false, "Unexpected exception: ${e::class.simpleName}: ${e.message}")
    }

    client.close()

    // Run assertions
    val requestCount = requestCounter.get()

    for (assertion in tc.assertions) {
        when (assertion.type) {
            "requestCount" -> {
                val expected = assertion.expected?.asInt()
                    ?: return TestResult(false, "requestCount assertion missing expected value")
                if (requestCount != expected) {
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
                // Verify the SDK correctly parsed and surfaced the response header.
                // For X-Total-Count, the SDK parses it into ListResult.meta.totalCount.
                when (headerName.lowercase()) {
                    "x-total-count" -> {
                        val actual = dispatchResult.totalCount?.toString()
                        if (actual != expected) {
                            return TestResult(false, "SDK meta.totalCount: expected $expected, got $actual")
                        }
                    }
                    else -> {
                        return TestResult(false, "headerValue assertion for unsupported header: $headerName")
                    }
                }
            }

            else -> {
                return TestResult(false, "Unknown assertion type: ${assertion.type}")
            }
        }
    }

    return TestResult(true, "All assertions passed")
}

/**
 * Dispatches the test operation against the SDK and returns observed metadata.
 */
private suspend fun dispatchOperation(tc: TestCase, account: AccountClient): DispatchResult {
    return when (tc.operation) {
        "ListProjects" -> {
            val result = account.projects.list()
            DispatchResult(totalCount = result.meta.totalCount)
        }

        "GetProject" -> {
            val projectId = tc.pathParams.longParam("projectId")
            account.projects.get(projectId)
            DispatchResult()
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
            DispatchResult(totalCount = result.meta.totalCount)
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
            account.timeline.projectTimeline()
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
