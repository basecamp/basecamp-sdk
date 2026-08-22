package com.basecamp.sdk

import io.ktor.client.engine.mock.*
import io.ktor.http.*
import io.ktor.utils.io.*
import kotlinx.coroutines.test.runTest
import kotlin.test.*

class DownloadTest {

    private fun mockClient(
        handler: MockRequestHandler,
        hooks: BasecampHooks = NoopHooks,
        enableRetry: Boolean = false,
        maxRetries: Int? = null,
    ): BasecampClient {
        val mockEngine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            engine = mockEngine
            this.enableRetry = enableRetry
            this.hooks = hooks
            if (maxRetries != null) {
                this.maxRetries = maxRetries
            }
        }
    }

    // -- filenameFromURL tests --

    @Test
    fun filenameFromURL_simple() {
        assertEquals("report.pdf", filenameFromURL("https://example.com/files/report.pdf"))
    }

    @Test
    fun filenameFromURL_encoded() {
        assertEquals("my report.pdf", filenameFromURL("https://example.com/files/my%20report.pdf"))
    }

    @Test
    fun filenameFromURL_trailingSlash() {
        assertEquals("download", filenameFromURL("https://example.com/files/"))
    }

    @Test
    fun filenameFromURL_noPath() {
        assertEquals("download", filenameFromURL("https://example.com"))
    }

    @Test
    fun filenameFromURL_empty() {
        assertEquals("download", filenameFromURL(""))
    }

    @Test
    fun filenameFromURL_deepPath() {
        assertEquals("notes.txt", filenameFromURL("https://example.com/a/b/c/notes.txt"))
    }

    @Test
    fun filenameFromURL_withQuery() {
        assertEquals("image.png", filenameFromURL("https://example.com/image.png?size=large"))
    }

    @Test
    fun filenameFromURL_rootPath() {
        assertEquals("download", filenameFromURL("https://example.com/"))
    }

    // -- Validation tests --

    @Test
    fun downloadURL_emptyThrowsUsage() = runTest {
        val client = mockClient({ respondOk("") })
        val account = client.forAccount("12345")
        val e = assertFailsWith<BasecampException.Usage> { account.downloadURL("") }
        assertContains(e.message!!, "required")
        client.close()
    }

    @Test
    fun downloadURL_relativeThrowsUsage() = runTest {
        val client = mockClient({ respondOk("") })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.Usage> { account.downloadURL("/just/a/path") }
        client.close()
    }

    // -- URL rewriting tests --

    @Test
    fun downloadURL_rewritesOrigin() = runTest {
        val client = mockClient({ request ->
            assertEquals("localhost", request.url.host)
            assertEquals(3000, request.url.port)
            assertEquals("/12345/attachments/abc/download/report.pdf", request.url.encodedPath)
            respond(
                content = ByteReadChannel("file-content"),
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf("application/pdf"),
                    HttpHeaders.ContentLength to listOf("12"),
                )
            )
        })
        val account = client.forAccount("12345")
        val result = account.downloadURL("https://other-host.example.com/12345/attachments/abc/download/report.pdf")
        assertEquals("file-content", result.body.decodeToString())
        assertEquals("application/pdf", result.contentType)
        client.close()
    }

    @Test
    fun downloadURL_preservesQueryParams() = runTest {
        val client = mockClient({ request ->
            assertEquals("abc", request.url.parameters["token"])
            assertEquals("2", request.url.parameters["v"])
            respond(
                content = ByteReadChannel("data"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType to listOf("application/octet-stream"))
            )
        })
        val account = client.forAccount("12345")
        val result = account.downloadURL("https://any-host.com/12345/download?token=abc&v=2")
        assertEquals("data", result.body.decodeToString())
        client.close()
    }

    // -- Redirect flow tests --

    @Test
    fun downloadURL_redirectFlow() = runTest {
        var requestCount = 0
        val client = mockClient({ request ->
            requestCount++
            if (requestCount == 1) {
                // Hop 1: API redirect
                respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file?sig=xyz"))
                )
            } else {
                // Hop 2: Signed download
                respond(
                    content = ByteReadChannel("pdf-content"),
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf("application/pdf"),
                        HttpHeaders.ContentLength to listOf("11"),
                    )
                )
            }
        })
        val account = client.forAccount("12345")
        val result = account.downloadURL("http://localhost:3000/12345/attachments/abc/download/report.pdf")
        assertEquals("pdf-content", result.body.decodeToString())
        assertEquals("application/pdf", result.contentType)
        assertEquals(11L, result.contentLength)
        assertEquals("report.pdf", result.filename)
        client.close()
    }

    @Test
    fun downloadURL_directDownload() = runTest {
        val client = mockClient({ _ ->
            respond(
                content = ByteReadChannel("direct-content"),
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf("text/plain"),
                    HttpHeaders.ContentLength to listOf("14"),
                )
            )
        })
        val account = client.forAccount("12345")
        val result = account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        assertEquals("direct-content", result.body.decodeToString())
        assertEquals("text/plain", result.contentType)
        assertEquals(14L, result.contentLength)
        assertEquals("file.txt", result.filename)
        client.close()
    }

    @Test
    fun downloadURL_redirectNoLocation() = runTest {
        val client = mockClient({ _ ->
            respond(
                content = ByteReadChannel(""),
                status = HttpStatusCode.Found,
                headers = headersOf()
            )
        })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.Api> {
            account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        }
        client.close()
    }

    // -- Error tests --

    @Test
    fun downloadURL_api404() = runTest {
        val client = mockClient({ _ ->
            respond(
                content = ByteReadChannel("""{"error":"Not found"}"""),
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType to listOf("application/json"))
            )
        })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.NotFound> {
            account.downloadURL("http://localhost:3000/12345/attachments/missing/download/file.txt")
        }
        client.close()
    }

    @Test
    fun downloadURL_fieldKeyed422MatchesServiceLayerParsing() = runTest {
        val client = mockClient({ _ ->
            respond(
                content = ByteReadChannel(
                    """{"error": "Validation failed", "errors": {"color": ["is not a valid color"]}}"""
                ),
                status = HttpStatusCode.UnprocessableEntity,
                headers = headersOf(HttpHeaders.ContentType to listOf("application/json"))
            )
        })
        val account = client.forAccount("12345")
        val e = assertFailsWith<BasecampException.Validation> {
            account.downloadURL("http://localhost:3000/12345/attachments/1/download/file.txt")
        }
        assertEquals("Validation failed (color: is not a valid color)", e.message)
        assertEquals(mapOf("color" to listOf("is not a valid color")), e.fieldErrors)
        client.close()
    }

    @Test
    fun downloadURL_messageKeyFallback() = runTest {
        val client = mockClient({ _ ->
            respond(
                content = ByteReadChannel("""{"message": "Try again later"}"""),
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType to listOf("application/json"))
            )
        })
        val account = client.forAccount("12345")
        val e = assertFailsWith<BasecampException.NotFound> {
            account.downloadURL("http://localhost:3000/12345/attachments/missing/download/file.txt")
        }
        assertEquals("Try again later", e.message)
        client.close()
    }

    @Test
    fun downloadURL_api403() = runTest {
        val client = mockClient({ _ ->
            respond(
                content = ByteReadChannel("""{"error":"Forbidden"}"""),
                status = HttpStatusCode.Forbidden,
                headers = headersOf(HttpHeaders.ContentType to listOf("application/json"))
            )
        })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.Forbidden> {
            account.downloadURL("http://localhost:3000/12345/attachments/secret/download/file.txt")
        }
        client.close()
    }

    @Test
    fun downloadURL_api500() = runTest {
        val client = mockClient({ _ ->
            respond(
                content = ByteReadChannel("""{"error":"Server error"}"""),
                status = HttpStatusCode.InternalServerError,
                headers = headersOf(HttpHeaders.ContentType to listOf("application/json"))
            )
        })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.Api> {
            account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        }
        client.close()
    }

    @Test
    fun downloadURL_s3Error() = runTest {
        var requestCount = 0
        val client = mockClient({ _ ->
            requestCount++
            if (requestCount == 1) {
                respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file"))
                )
            } else {
                respond(
                    content = ByteReadChannel("AccessDenied"),
                    status = HttpStatusCode.Forbidden,
                )
            }
        })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.Api> {
            account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        }
        client.close()
    }

    // SPEC §14 "Hop-2 Redirect Policy": the signed URL is the one destination
    // the API host named. A redirect from it surfaces with its status, and the
    // Location it names is never dialled (#805). Kotlin has always run hop 2 on
    // hop 1's followRedirects = false client; this pins the explicit refusal.
    @Test
    fun downloadURL_hop2RedirectIsRefusedNotFollowed() = runTest {
        var requestCount = 0
        var thirdHits = 0
        val client = mockClient({ request ->
            requestCount++
            when (request.url.encodedPath) {
                "/elsewhere/file" -> {
                    thirdHits++
                    respond(content = ByteReadChannel("SECRET"), status = HttpStatusCode.OK)
                }
                "/signed/file" -> respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/elsewhere/file"))
                )
                else -> respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file"))
                )
            }
        })
        val account = client.forAccount("12345")

        val e = assertFailsWith<BasecampException.Api> {
            account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        }
        assertEquals(302, e.httpStatus)
        assertTrue(e.message!!.contains("not followed"), "expected the message to name the refusal, got ${e.message}")
        assertEquals(2, requestCount, "the redirect target must never be dialled")
        assertEquals(0, thirdHits)
        client.close()
    }

    // -- Auth header tests --

    @Test
    fun downloadURL_authOnApiNotOnS3() = runTest {
        var requestCount = 0
        val client = mockClient({ request ->
            requestCount++
            if (requestCount == 1) {
                // API leg should have auth
                assertNotNull(request.headers[HttpHeaders.Authorization])
                respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file"))
                )
            } else {
                // S3 leg should NOT have auth
                assertNull(request.headers[HttpHeaders.Authorization])
                respond(
                    content = ByteReadChannel("data"),
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf("application/octet-stream"))
                )
            }
        })
        val account = client.forAccount("12345")
        account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        client.close()
    }

    // -- Hook tests --

    @Test
    fun downloadURL_operationHooks() = runTest {
        val opsStarted = mutableListOf<OperationInfo>()
        val opsEnded = mutableListOf<Pair<OperationInfo, OperationResult>>()

        val hooks = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) { opsStarted.add(info) }
            override fun onOperationEnd(info: OperationInfo, result: OperationResult) { opsEnded.add(info to result) }
        }

        val client = mockClient(
            handler = { _ ->
                respond(
                    content = ByteReadChannel("data"),
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf("text/plain"))
                )
            },
            hooks = hooks,
        )
        val account = client.forAccount("12345")
        account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")

        assertEquals(1, opsStarted.size)
        assertEquals("Account", opsStarted[0].service)
        assertEquals("DownloadURL", opsStarted[0].operation)

        assertEquals(1, opsEnded.size)
        assertNull(opsEnded[0].second.error)
        client.close()
    }

    @Test
    fun downloadURL_requestHooksApiOnly() = runTest {
        val reqStarted = mutableListOf<RequestInfo>()
        val reqEnded = mutableListOf<Pair<RequestInfo, RequestResult>>()

        val hooks = object : BasecampHooks {
            override fun onRequestStart(info: RequestInfo) { reqStarted.add(info) }
            override fun onRequestEnd(info: RequestInfo, result: RequestResult) { reqEnded.add(info to result) }
        }

        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                if (requestCount == 1) {
                    respond(
                        content = ByteReadChannel(""),
                        status = HttpStatusCode.Found,
                        headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file"))
                    )
                } else {
                    respond(
                        content = ByteReadChannel("data"),
                        status = HttpStatusCode.OK,
                        headers = headersOf(HttpHeaders.ContentType to listOf("text/plain"))
                    )
                }
            },
            hooks = hooks,
        )
        val account = client.forAccount("12345")
        account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")

        // Request hooks fire for hop 1 only
        assertEquals(1, reqStarted.size)
        assertEquals(1, reqEnded.size)
        assertEquals("GET", reqStarted[0].method)
        client.close()
    }

    // -- Network failure tests --

    @Test
    fun downloadURL_hop1NetworkFailure() = runTest {
        val reqEnded = mutableListOf<Pair<RequestInfo, RequestResult>>()

        val hooks = object : BasecampHooks {
            override fun onRequestEnd(info: RequestInfo, result: RequestResult) { reqEnded.add(info to result) }
        }

        val client = mockClient(
            handler = { _ -> throw java.io.IOException("Connection refused") },
            hooks = hooks,
        )
        val account = client.forAccount("12345")

        val e = assertFailsWith<BasecampException.Network> {
            account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        }
        assertEquals("network", e.code)

        // on_request_end fires with statusCode=0
        assertEquals(1, reqEnded.size)
        assertEquals(0, reqEnded[0].second.statusCode)
        client.close()
    }

    @Test
    fun downloadURL_hop2NetworkFailure() = runTest {
        var requestCount = 0
        val client = mockClient({ _ ->
            requestCount++
            if (requestCount == 1) {
                respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file"))
                )
            } else {
                throw java.io.IOException("Connection reset")
            }
        })
        val account = client.forAccount("12345")

        val e = assertFailsWith<BasecampException.Network> {
            account.downloadURL("http://localhost:3000/12345/attachments/abc/download/file.txt")
        }
        assertEquals("network", e.code)
        client.close()
    }

    // -- Hop-1 retry policy (SPEC §14) --

    private val hop1URL = "http://localhost:3000/12345/attachments/abc/download/file.txt"

    // The COMPLETE declared retry set, pinned status by status (the shared
    // conformance fixtures cover 429/503): hop 1 retries {429, 502, 503, 504}
    // under the public maxRetries total-attempt cap coerced to at least one.
    @Test
    fun downloadURL_retriesDeclaredStatusesThenFollowsRedirect() = runTest {
        for (status in listOf(429, 502, 503, 504)) {
            var hop1Attempts = 0
            var hop2Requests = 0
            val client = mockClient(
                handler = { request ->
                    if (request.url.encodedPath.startsWith("/signed/")) {
                        hop2Requests++
                        respond(
                            content = ByteReadChannel("data"),
                            status = HttpStatusCode.OK,
                            headers = headersOf(HttpHeaders.ContentType to listOf("application/octet-stream")),
                        )
                    } else {
                        hop1Attempts++
                        if (hop1Attempts == 1) {
                            respond(
                                content = ByteReadChannel("{}"),
                                status = HttpStatusCode.fromValue(status),
                                headers = headersOf(HttpHeaders.ContentType to listOf("application/json")),
                            )
                        } else {
                            respond(
                                content = ByteReadChannel(""),
                                status = HttpStatusCode.Found,
                                headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file")),
                            )
                        }
                    }
                },
                enableRetry = true,
            )
            val account = client.forAccount("12345")

            val result = account.downloadURL(hop1URL)

            assertEquals("data", result.body.decodeToString(), "status $status")
            assertEquals(2, hop1Attempts, "status $status")
            assertEquals(1, hop2Requests, "status $status")
            client.close()
        }
    }

    @Test
    fun downloadURL_neverRetries500() = runTest {
        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                respond(
                    content = ByteReadChannel("""{"error":"Server error"}"""),
                    status = HttpStatusCode.InternalServerError,
                    headers = headersOf(HttpHeaders.ContentType to listOf("application/json")),
                )
            },
            enableRetry = true,
        )
        val account = client.forAccount("12345")

        assertFailsWith<BasecampException.Api> { account.downloadURL(hop1URL) }

        // 500 is deliberately outside the declared set {429, 502, 503, 504}.
        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun downloadURL_retriesNetworkErrorThenSucceeds() = runTest {
        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                if (requestCount == 1) {
                    throw java.io.IOException("Connection refused")
                }
                respond(
                    content = ByteReadChannel("content"),
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf("text/plain")),
                )
            },
            enableRetry = true,
        )
        val account = client.forAccount("12345")

        val result = account.downloadURL(hop1URL)

        assertEquals("content", result.body.decodeToString())
        assertEquals(2, requestCount)
        client.close()
    }

    @Test
    fun downloadURL_exhaustsCapThenSurfacesError() = runTest {
        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                respond(
                    content = ByteReadChannel("{}"),
                    status = HttpStatusCode.ServiceUnavailable,
                    headers = headersOf(HttpHeaders.ContentType to listOf("application/json")),
                )
            },
            enableRetry = true,
            maxRetries = 3,
        )
        val account = client.forAccount("12345")

        assertFailsWith<BasecampException.Api> { account.downloadURL(hop1URL) }

        assertEquals(3, requestCount)
        client.close()
    }

    @Test
    fun downloadURL_zeroMaxRetriesStillSendsOneAttempt() = runTest {
        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                respond(
                    content = ByteReadChannel("{}"),
                    status = HttpStatusCode.ServiceUnavailable,
                    headers = headersOf(HttpHeaders.ContentType to listOf("application/json")),
                )
            },
            enableRetry = true,
            maxRetries = 0,
        )
        val account = client.forAccount("12345")

        // The download attempt budget is the public cap coerced to at least
        // one: an accepted maxRetries = 0 still sends exactly one attempt.
        assertFailsWith<BasecampException.Api> { account.downloadURL(hop1URL) }

        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun downloadURL_enableRetryFalseSendsExactlyOneAttempt() = runTest {
        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                respond(
                    content = ByteReadChannel("{}"),
                    status = HttpStatusCode.ServiceUnavailable,
                    headers = headersOf(HttpHeaders.ContentType to listOf("application/json")),
                )
            },
            enableRetry = false,
        )
        val account = client.forAccount("12345")

        assertFailsWith<BasecampException.Api> { account.downloadURL(hop1URL) }

        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun downloadURL_authOnEveryHop1AttemptNeverOnHop2() = runTest {
        val hop1AuthHeaders = mutableListOf<String?>()
        var hop2AuthHeader: String? = "unset"
        var hop1Attempts = 0
        val client = mockClient(
            handler = { request ->
                if (request.url.encodedPath.startsWith("/signed/")) {
                    hop2AuthHeader = request.headers[HttpHeaders.Authorization]
                    respond(
                        content = ByteReadChannel("data"),
                        status = HttpStatusCode.OK,
                        headers = headersOf(HttpHeaders.ContentType to listOf("application/octet-stream")),
                    )
                } else {
                    hop1Attempts++
                    hop1AuthHeaders.add(request.headers[HttpHeaders.Authorization])
                    if (hop1Attempts == 1) {
                        respond(
                            content = ByteReadChannel("{}"),
                            status = HttpStatusCode.ServiceUnavailable,
                            headers = headersOf(HttpHeaders.ContentType to listOf("application/json")),
                        )
                    } else {
                        respond(
                            content = ByteReadChannel(""),
                            status = HttpStatusCode.Found,
                            headers = headersOf(HttpHeaders.Location to listOf("http://localhost:3000/signed/file")),
                        )
                    }
                }
            },
            enableRetry = true,
        )
        val account = client.forAccount("12345")

        account.downloadURL(hop1URL)

        assertEquals(listOf<String?>("Bearer test-token", "Bearer test-token"), hop1AuthHeaders)
        assertNull(hop2AuthHeader)
        client.close()
    }

    @Test
    fun downloadURL_balancedHooksAcrossRetries() = runTest {
        val starts = mutableListOf<Int>()
        val ends = mutableListOf<Int>()
        val retries = mutableListOf<Int>()
        val hooks = object : BasecampHooks {
            override fun onRequestStart(info: RequestInfo) {
                starts.add(info.attempt)
            }

            override fun onRequestEnd(info: RequestInfo, result: RequestResult) {
                ends.add(info.attempt)
            }

            override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
                retries.add(attempt)
            }
        }
        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                if (requestCount < 3) {
                    respond(
                        content = ByteReadChannel("{}"),
                        status = HttpStatusCode.ServiceUnavailable,
                        headers = headersOf(HttpHeaders.ContentType to listOf("application/json")),
                    )
                } else {
                    respond(
                        content = ByteReadChannel("content"),
                        status = HttpStatusCode.OK,
                        headers = headersOf(HttpHeaders.ContentType to listOf("text/plain")),
                    )
                }
            },
            hooks = hooks,
            enableRetry = true,
        )
        val account = client.forAccount("12345")

        account.downloadURL(hop1URL)

        assertEquals(listOf(1, 2, 3), starts)
        assertEquals(listOf(1, 2, 3), ends)
        // onRetry receives the UPCOMING attempt (SPEC §7 attempt semantics).
        assertEquals(listOf(2, 3), retries)
        client.close()
    }

    @Test
    fun downloadURL_honorsRetryAfterOn429() = runTest {
        var requestCount = 0
        val requestTimestamps = mutableListOf<Long>()
        val client = mockClient(
            handler = { _ ->
                requestCount++
                requestTimestamps.add(testScheduler.currentTime)
                if (requestCount == 1) {
                    respond(
                        content = ByteReadChannel("{}"),
                        status = HttpStatusCode.TooManyRequests,
                        headers = headersOf(
                            HttpHeaders.ContentType to listOf("application/json"),
                            "Retry-After" to listOf("7"),
                        ),
                    )
                } else {
                    respond(
                        content = ByteReadChannel("content"),
                        status = HttpStatusCode.OK,
                        headers = headersOf(HttpHeaders.ContentType to listOf("text/plain")),
                    )
                }
            },
            enableRetry = true,
        )
        val account = client.forAccount("12345")

        account.downloadURL(hop1URL)

        assertEquals(2, requestCount)
        val elapsed = requestTimestamps[1] - requestTimestamps[0]
        assertTrue(elapsed >= 7000, "Expected delay >= 7000ms from Retry-After: 7, got $elapsed")
        client.close()
    }

    @Test
    fun downloadURL_cancellationIsTerminal() = runTest {
        var requestCount = 0
        val client = mockClient(
            handler = { _ ->
                requestCount++
                throw kotlin.coroutines.cancellation.CancellationException("cancelled")
            },
            enableRetry = true,
        )
        val account = client.forAccount("12345")

        // Cancellation is not a transport failure: it must propagate raw with
        // NO further attempts, even though a network error would have been
        // retried under the same budget.
        assertFailsWith<kotlin.coroutines.cancellation.CancellationException> {
            account.downloadURL(hop1URL)
        }

        assertEquals(1, requestCount)
        client.close()
    }
}
