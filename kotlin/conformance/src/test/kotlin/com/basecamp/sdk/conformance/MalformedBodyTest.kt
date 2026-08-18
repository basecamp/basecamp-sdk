package com.basecamp.sdk.conformance

import com.basecamp.sdk.AuthStrategy
import com.basecamp.sdk.BasecampClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.projects
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.SerializationException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertSame

/**
 * Both directions of the runner's malformed-body test (#750, the tail of #730).
 *
 * The runner's `BasecampException` arm decides between "the mock body is wrong,
 * fail loudly so the fixture gets repaired" (#555) and "this is a result the
 * assertions get to judge". Getting that wrong in the first direction reports a
 * fixture bug that does not exist and hides the real error; in the second it
 * lets a case pinning `errorCode: api_error` be satisfied by a decoder
 * rejection.
 *
 * Only the first direction is reachable from `conformance/tests/`: a fixture
 * cannot install an `AuthStrategy`, so the misread this file pins was live in
 * the runner with every committed fixture green.
 */
class MalformedBodyTest {
    private fun clientReturning(body: String): BasecampClient =
        BasecampClient {
            accessToken("conformance-test-token")
            baseUrl = "http://localhost:3000"
            enableRetry = false
            engine = MockEngine {
                respond(
                    content = body,
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        }

    /**
     * The positive direction, produced the only way it can be: by the SDK's own
     * response decoder, since the factory that fills the slot is internal to
     * `:basecamp-sdk`. A body the `Project` model refuses arrives as the SPEC §6
     * statusless `api_error` (#604), and the decoder's exception is what the
     * runner reports to the fixture author.
     */
    @Test
    fun `a decoder refusal carries the decode failure`() {
        val client = clientReturning("""{"id": "not-a-number"}""")

        val error = assertFailsWith<BasecampException.Api> {
            runBlocking { client.forAccount("999").projects.get(1) }
        }

        val failure = malformedBodyFailure(error)
        assertNotNull(failure, "a body the model refuses must be recognized as a malformed body")
        assertSame(
            error.cause, failure,
            "the runner reports the decoder's own account of what was wrong",
        )
        client.close()
    }

    /**
     * The #730 case, one module over. `BasecampHttpClient` propagates an
     * already-classified `BasecampException` from the auth strategy untouched,
     * so a token provider that classifies its own JSON failure as
     * `Api(cause = SerializationException(…))` reaches the runner's
     * `BasecampException` arm. Through the public constructor `decodeFailure`
     * stays null, which is the whole point: the strategy did not decode a
     * Basecamp response body, and no request was ever sent.
     *
     * While the runner read `cause as? SerializationException`, this matched —
     * and the case was failed as "Mock body does not decode into the Kotlin
     * model" naming a fixture that is fine, or silently absorbed as the expected
     * failure of a case that declares `errorRaised`.
     */
    @Test
    fun `an auth strategy failure carrying a serialization cause is not a malformed body`() {
        val thrown = BasecampException.Api(
            message = "the token endpoint returned a body that does not decode",
            cause = SerializationException("Unexpected JSON token at offset 0"),
        )
        val client = BasecampClient {
            auth(AuthStrategy { throw thrown })
            baseUrl = "http://localhost:3000"
            enableRetry = false
            engine = MockEngine { respond(content = "{}", status = HttpStatusCode.OK) }
        }

        val error = assertFailsWith<BasecampException.Api> {
            runBlocking { client.forAccount("999").projects.get(1) }
        }

        assertSame(thrown, error, "the strategy's own exception must reach the runner unchanged")
        assertNull(
            malformedBodyFailure(error),
            "an auth failure is not a malformed response body — no request was sent",
        )
        client.close()
    }

    /**
     * The other statusless `api_error` a runner sees, and the reason
     * statuslessness alone cannot be the test: the pagination same-origin
     * refusal is a deliberate guard, `security.json` asserts on it, and routing
     * it to "repair the fixture body" would both lose the assertion and mislabel
     * the refusal.
     */
    @Test
    fun `a guard refusal with no decode failure is not a malformed body`() {
        val refusal = BasecampException.Api(
            message = "Pagination Link header points to a different origin: https://evil.example.com/",
        )

        assertNull(malformedBodyFailure(refusal))
        assertEquals(null, refusal.decodeFailure)
    }
}
