package com.basecamp.sdk

import com.basecamp.sdk.generated.people
import com.basecamp.sdk.generated.projects
import com.basecamp.sdk.generated.recordings
import com.basecamp.sdk.generated.reports
import com.basecamp.sdk.services.BaseService
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.SerializationException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The response decoder is isolated from everything else the request primitives
 * do (#604).
 *
 * Each primitive in [BaseService] runs encode → URL build → auth → transport →
 * status check → decode inside one `try` whose terminal `catch` maps nothing.
 * That made a malformed 2xx body indistinguishable, to a caller catching
 * [BasecampException], from the auth strategy throwing or the socket dropping:
 * all three arrived raw. Only the decode call is wrapped now, so:
 *
 * - a body that does not decode is a SPEC §6 statusless, non-retryable
 *   `api_error` carrying the decoder's own exception as `cause`, and
 * - an auth-phase throw and a *request-body* encoding failure — which happens
 *   inside the same `try`, and throws the very same [SerializationException]
 *   type — still surface raw.
 *
 * The negative tests are what makes the isolation checkable. Wrapping the block
 * rather than the expression, or widening the mapped type past
 * [SerializationException], reintroduces the original conflation in a new shape
 * and is what they fail on.
 */
class DecodeIsolationTest {

    private val jsonHeaders = headersOf(HttpHeaders.ContentType, "application/json")

    private fun mockClient(handler: MockRequestHandler): BasecampClient =
        testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            engine = MockEngine(handler)
            enableRetry = false
        }

    // -- Positive: a malformed 2xx body is a statusless api_error --

    /**
     * `request`: the single-object primitive. `{}` decodes as a JSON object and
     * then fails on `Project`'s required members, so this is a decode failure
     * and nothing else — the transport returned 200.
     */
    @Test
    fun malformedBodyOnASingleGetIsAStatuslessApiError() = runTest {
        val client = mockClient { respond("{}", HttpStatusCode.OK, jsonHeaders) }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").projects.get(1L)
        }

        assertNull(error.httpStatus, "the transport succeeded, so no status describes this")
        assertFalse(error.retryable, "re-requesting cannot repair a malformed body")
        assertIs<SerializationException>(
            error.cause,
            "the decoder's own exception must survive as the cause, got ${error.cause}",
        )
        assertTrue(
            error.message!!.contains("GetProject returned a body that does not decode"),
            "expected the operation to be named in the message, got: ${error.message}",
        )
        client.close()
    }

    /** `requestPaginated`: first page. */
    @Test
    fun malformedFirstPageIsAStatuslessApiError() = runTest {
        val client = mockClient { respond("[{}]", HttpStatusCode.OK, jsonHeaders) }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").projects.list()
        }

        assertNull(error.httpStatus)
        assertFalse(error.retryable)
        assertIs<SerializationException>(error.cause)
        assertTrue(
            error.message!!.contains("ListProjects returned a body that does not decode"),
            "got: ${error.message}",
        )
        client.close()
    }

    /**
     * `requestPaginated`: the follow loop. A second page decoded outside the
     * wrap would leak raw, so the loop site needs its own isolation — the first
     * page here is well-formed.
     */
    @Test
    fun malformedSecondPageIsAStatuslessApiError() = runTest {
        var page = 0
        val client = mockClient {
            page += 1
            if (page == 1) {
                respond(
                    "[]",
                    HttpStatusCode.OK,
                    headersOf(
                        HttpHeaders.ContentType to listOf("application/json"),
                        HttpHeaders.Link to listOf(
                            "<http://localhost:3000/12345/projects.json?page=2>; rel=\"next\""
                        ),
                    ),
                )
            } else {
                respond("[{}]", HttpStatusCode.OK, jsonHeaders)
            }
        }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").projects.list()
        }

        assertEquals(2, page, "the second page must actually have been fetched")
        assertNull(error.httpStatus)
        assertFalse(error.retryable)
        assertIs<SerializationException>(error.cause)
        client.close()
    }

    /**
     * A *numeric* refusal.
     *
     * `Person.id` decodes through `FlexibleLongSerializer`, which reaches
     * `JsonPrimitive.long` for an unquoted number — and that is
     * `content.toLong()`, so a fractional or out-of-range literal raises
     * [NumberFormatException] rather than [SerializationException]. Nearly every
     * response carries a Person, so this is an ordinary operation's ordinary
     * failure, not an exotic one, and before #604 it escaped the SDK entirely.
     *
     * The serializer translates it rather than the helper catching it, so that
     * exactly one exception type crosses the mapping boundary: the composites
     * and the conformance runner both read `cause` to tell a decoder rejection
     * from a real API failure, and a second cause type is a second thing each
     * of them would have to learn. The numeric original is still underneath.
     */
    @Test
    fun aNumericRefusalIsAStatuslessApiError() = runTest {
        for (badId in listOf("1.5", "99999999999999999999")) {
            val client = mockClient {
                respond(
                    """{"id": $badId, "name": "Ann"}""",
                    HttpStatusCode.OK,
                    jsonHeaders,
                )
            }

            val error = assertFailsWith<BasecampException.Api>(
                "an id of $badId must be refused as a malformed body",
            ) {
                client.forAccount("12345").people.get(1L)
            }

            assertNull(error.httpStatus)
            assertFalse(error.retryable)
            val cause = error.cause
            assertIs<SerializationException>(
                cause,
                "every decode failure must reach `cause` as one type, got $cause",
            )
            assertIs<NumberFormatException>(
                cause.cause,
                "the numeric original must survive beneath it, got ${cause.cause}",
            )
            assertTrue(
                cause.message!!.contains(badId),
                "the refused value must be named: ${cause.message}",
            )
            client.close()
        }
    }

    /**
     * A value-returning operation that unexpectedly receives 204.
     *
     * The primitive used to short-circuit ANY 204 to `Unit as T`. Against the
     * un-fixed primitive this test does not merely fail — it reports "completed
     * successfully", because the unchecked cast raises nothing at the boundary:
     * the caller gets Unit wearing `Project`'s type, and the ClassCastException
     * arrives later at whatever site first uses it, or never. The empty body now
     * reaches the decoder, where it is what it looks like: a body that does not
     * decode.
     */
    @Test
    fun a204ForAValueReturningOperationIsAStatuslessApiError() = runTest {
        val client = mockClient { respond("", HttpStatusCode.NoContent, jsonHeaders) }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").projects.get(1L)
        }

        assertNull(error.httpStatus, "204 is a success; the body is what failed")
        assertFalse(error.retryable)
        assertIs<SerializationException>(error.cause)
        client.close()
    }

    /** The other half of that: a void operation's 204 still decodes to Unit. */
    @Test
    fun a204ForAVoidOperationStillSucceeds() = runTest {
        val client = mockClient { respond("", HttpStatusCode.NoContent, jsonHeaders) }

        client.forAccount("12345").recordings.trash(42L)

        client.close()
    }

    /**
     * `requestPaginatedWrapped`: a body that is not JSON at all.
     *
     * Deliberately not a valid-JSON-wrong-shape body. The generated wrapper
     * accessor for this operation reaches `["events"]!!.jsonArray`, whose
     * `NullPointerException`/`IllegalArgumentException` this helper does not map
     * — a generated-code defect tracked with the rest of the wrapper-decode gap
     * (#728), not an exception type to widen the catch for.
     */
    @Test
    fun malformedWrappedBodyIsAStatuslessApiError() = runTest {
        val client = mockClient { respond("not json at all", HttpStatusCode.OK, jsonHeaders) }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").reports.personProgress(7L)
        }

        assertNull(error.httpStatus)
        assertFalse(error.retryable)
        assertIs<SerializationException>(error.cause)
        assertTrue(
            error.message!!.contains("GetPersonProgress returned a body that does not decode"),
            "got: ${error.message}",
        )
        client.close()
    }

    // -- Negative: everything else in the block keeps its own identity --

    /**
     * An auth-phase throw is a credential-provider fault, not a malformed
     * response. It surfaces raw (the strategy's own exception), which is only
     * checkable *because* the decoder no longer shares its error shape.
     */
    @Test
    fun anAuthStrategyFailureIsNotAnApiError() = runTest {
        var requests = 0
        val client = testBasecampClient {
            auth(AuthStrategy { throw IllegalStateException("token vault unreachable") })
            baseUrl = "http://localhost:3000"
            engine = MockEngine {
                requests += 1
                respond("{}", HttpStatusCode.OK, jsonHeaders)
            }
            enableRetry = false
        }

        val error: Throwable? = runCatching {
            client.forAccount("12345").projects.get(1L)
        }.exceptionOrNull()

        assertNotNull(error, "the auth strategy's throw must reach the caller")
        assertFalse(
            error is BasecampException,
            "the auth strategy's own exception must not be relabelled as an SDK error, got $error",
        )
        assertIs<IllegalStateException>(error)
        assertEquals("token vault unreachable", error.message)
        assertEquals(0, requests, "auth failed, so no request was ever sent")
        client.close()
    }

    /**
     * The wrap-the-block mistake, pinned. Generated services serialize the
     * *request* body inside the `fn` lambda — `json.encodeToString(...)`, see
     * any generated mutation — which runs inside the same `try` as the decode
     * and throws the same [SerializationException] type. Wrapping the block
     * would report a request-encoding fault as a malformed *response*.
     */
    @Test
    fun aRequestBodyEncodingFailureIsNotAnApiError() = runTest {
        var requests = 0
        val client = mockClient {
            requests += 1
            respond("{}", HttpStatusCode.OK, jsonHeaders)
        }
        val service = EncodeFailingService(client.forAccount("12345"))

        val error: Throwable? = runCatching { service.call() }.exceptionOrNull()

        assertNotNull(error, "the encode failure must reach the caller")
        assertFalse(
            error is BasecampException,
            "an encode fault must not be reported as a malformed response, got $error",
        )
        assertIs<SerializationException>(error)
        assertEquals("request body could not be encoded", error.message)
        assertEquals(0, requests, "encoding failed, so no request was ever sent")
        client.close()
    }

    /**
     * A transport failure keeps its own classification (`network`, retryable),
     * which a decoder mapping that reached past its expression would flatten
     * into a statusless `api_error`.
     */
    @Test
    fun aTransportFailureIsNotAnApiError() = runTest {
        val client = mockClient { throw IllegalStateException("connection reset") }

        val error = assertFailsWith<BasecampException.Network> {
            client.forAccount("12345").projects.get(1L)
        }

        assertTrue(error.retryable, "a network fault is retryable; a malformed body is not")
        client.close()
    }

    /**
     * Stands in for the request-body encoding step the generated services run
     * inside `fn`. Nothing else can make `json.encodeToString` fail from
     * outside the SDK, and the point under test is *where* the mapping reaches,
     * not which value provoked it.
     */
    private class EncodeFailingService(client: AccountClient) : BaseService(client) {
        suspend fun call(): String = request(
            OperationInfo("Test", "EncodeFailure", "test", isMutation = true),
            { throw SerializationException("request body could not be encoded") },
        ) { body -> body }
    }
}
