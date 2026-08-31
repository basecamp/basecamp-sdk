package com.basecamp.sdk

import com.basecamp.sdk.oauth.*
import io.ktor.client.*
import io.ktor.client.engine.mock.*
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import java.security.MessageDigest
import java.util.Base64
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

class OAuthTest {

    @Test
    fun pkceVerifierLength() {
        val pkce = generatePkce()
        assertTrue(pkce.verifier.length >= 43, "PKCE verifier should be at least 43 chars")
    }

    @Test
    fun pkceChallengeLength() {
        val pkce = generatePkce()
        assertTrue(pkce.challenge.isNotBlank(), "PKCE challenge should not be blank")
    }

    @Test
    fun pkceVerifierIsBase64Url() {
        val pkce = generatePkce()
        val base64UrlChars = ('A'..'Z') + ('a'..'z') + ('0'..'9') + listOf('-', '_')
        assertTrue(pkce.verifier.all { it in base64UrlChars }, "PKCE verifier should be base64url")
    }

    @Test
    fun pkceIsUnique() {
        val pkce1 = generatePkce()
        val pkce2 = generatePkce()
        assertNotEquals(pkce1.verifier, pkce2.verifier, "PKCE verifiers should be unique")
    }

    @Test
    fun pkceChallengeIsDeterministicForSameVerifier() {
        // SHA-256 of same input should produce same output
        val pkce = generatePkce()
        assertTrue(pkce.challenge.isNotBlank())
    }

    @Test
    fun generateStateLength() {
        val state = generateState()
        assertTrue(state.length >= 22, "State token should be at least 22 chars")
    }

    @Test
    fun generateStateIsBase64Url() {
        val state = generateState()
        val base64UrlChars = ('A'..'Z') + ('a'..'z') + ('0'..'9') + listOf('-', '_')
        assertTrue(state.all { it in base64UrlChars }, "State should be base64url")
    }

    @Test
    fun generateStateIsUnique() {
        val s1 = generateState()
        val s2 = generateState()
        assertNotEquals(s1, s2, "State tokens should be unique")
    }

    @Test
    fun oauthConfigSerialization() {
        val config = OAuthConfig(
            issuer = "https://launchpad.37signals.com",
            authorizationEndpoint = "https://launchpad.37signals.com/authorization/new",
            tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
        )
        assertEquals("https://launchpad.37signals.com", config.issuer)
        assertEquals("https://launchpad.37signals.com/authorization/new", config.authorizationEndpoint)
    }

    @Test
    fun tokenExpirationCheck() {
        val token = OAuthToken(
            accessToken = "test",
            refreshToken = null,
            tokenType = "Bearer",
            expiresIn = 3600,
            expiresAt = System.currentTimeMillis() + 3600_000,
            scope = null,
        )
        assertTrue(!isTokenExpired(token), "Token should not be expired")
    }

    @Test
    fun tokenExpirationCheckExpired() {
        val token = OAuthToken(
            accessToken = "test",
            refreshToken = null,
            tokenType = "Bearer",
            expiresIn = 0,
            expiresAt = System.currentTimeMillis() - 1000,
            scope = null,
        )
        assertTrue(isTokenExpired(token), "Token should be expired")
    }

    @Test
    fun tokenExpirationWithBuffer() {
        val token = OAuthToken(
            accessToken = "test",
            refreshToken = null,
            tokenType = "Bearer",
            expiresIn = 30,
            expiresAt = System.currentTimeMillis() + 30_000,
            scope = null,
        )
        // With default 60-second buffer, a token expiring in 30 seconds is "expired"
        assertTrue(isTokenExpired(token), "Token within buffer window should be expired")
    }

    @Test
    fun tokenExpirationNoExpiry() {
        val token = OAuthToken(
            accessToken = "test",
            refreshToken = null,
            tokenType = "Bearer",
            expiresIn = null,
            expiresAt = null,
            scope = null,
        )
        assertTrue(!isTokenExpired(token), "Token with no expiry should not be expired")
    }

    @Test
    fun launchpadBaseUrl() {
        assertEquals("https://launchpad.37signals.com", LAUNCHPAD_BASE_URL)
    }

    // =========================================================================
    // exchangeCode with mock HTTP
    // =========================================================================

    @Test
    fun exchangeParseFailureNeverEchoesTheBody() = runTest {
        // A syntactically-broken token body can still carry credential
        // material, and kotlinx embeds input excerpts in its exception
        // messages — the mapped fault must not echo any of it.
        val secret = "sk-live-SUPERSECRET"
        val engine = MockEngine {
            respond(
                content = "{\"access_token\": \"$secret' oops",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        val e = assertFailsWith<BasecampException.Api> {
            exchangeCode(
                tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                code = "test-code",
                redirectUri = "https://myapp.com/callback",
                clientId = "test-client",
                clientSecret = "test-secret",
                client = httpClient,
            )
        }
        assertFalse(e.message!!.contains(secret), "parse fault must not echo the body")
        assertNull(e.cause, "decoder cause must be dropped — its message embeds the body")
    }

    @Test
    fun exchangeEmptyTokenTypeIsApiFault() = runTest {
        // SPEC §16: token_type defaults to Bearer only when absent/JSON-null;
        // a present-but-empty value is a malformed response — matching the
        // device-flow parser.
        val engine = MockEngine {
            respond(
                content = "{\"access_token\": \"a\", \"token_type\": \"\"}",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val httpClient = HttpClient(engine)
        val e = assertFailsWith<BasecampException.Api> {
            exchangeCode(
                tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                code = "c",
                redirectUri = "https://myapp.com/callback",
                clientId = "id",
                clientSecret = "s",
                client = httpClient,
            )
        }
        assertTrue(e.message!!.contains("token_type"))
    }

    @Test
    fun exchangeNullTokenTypeDefaultsToBearer() = runTest {
        val engine = MockEngine {
            respond(
                content = "{\"access_token\": \"a\", \"token_type\": null}",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val httpClient = HttpClient(engine)
        val token = exchangeCode(
            tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
            code = "c",
            redirectUri = "https://myapp.com/callback",
            clientId = "id",
            clientSecret = "s",
            client = httpClient,
        )
        assertEquals("Bearer", token.tokenType)
    }

    @Test
    fun exchangeCodeSuccess() = runTest {
        val engine = MockEngine { request ->
            assertEquals(HttpMethod.Post, request.method)
            val body = request.body.toByteArray().decodeToString()
            assertTrue(body.contains("grant_type=authorization_code"))
            assertTrue(body.contains("code=test-code"))
            assertTrue(body.contains("client_id=test-client"))

            respond(
                content = """{
                    "access_token": "access-123",
                    "refresh_token": "refresh-456",
                    "token_type": "Bearer",
                    "expires_in": 3600
                }""",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        val token = exchangeCode(
            tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
            code = "test-code",
            redirectUri = "https://myapp.com/callback",
            clientId = "test-client",
            clientSecret = "test-secret",
            client = httpClient,
        )

        assertEquals("access-123", token.accessToken)
        assertEquals("refresh-456", token.refreshToken)
        assertEquals("Bearer", token.tokenType)
        assertEquals(3600L, token.expiresIn)
        assertTrue(token.expiresAt!! > System.currentTimeMillis())

        httpClient.close()
    }

    @Test
    fun exchangeCodeErrorResponse() = runTest {
        val engine = MockEngine { _ ->
            respond(
                content = """{
                    "error": "invalid_grant",
                    "error_description": "Authorization code has expired"
                }""",
                status = HttpStatusCode.BadRequest,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        try {
            exchangeCode(
                tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                code = "expired-code",
                redirectUri = "https://myapp.com/callback",
                clientId = "test-client",
                clientSecret = "test-secret",
                client = httpClient,
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Auth) {
            assertTrue(e.message!!.contains("Authorization code has expired"))
        }

        httpClient.close()
    }

    // =========================================================================
    // refreshToken with mock HTTP
    // =========================================================================

    @Test
    fun refreshTokenSuccess() = runTest {
        val engine = MockEngine { request ->
            val body = request.body.toByteArray().decodeToString()
            assertTrue(body.contains("grant_type=refresh_token"))
            assertTrue(body.contains("refresh_token=refresh-456"))

            respond(
                content = """{
                    "access_token": "new-access-789",
                    "refresh_token": "new-refresh-012",
                    "token_type": "Bearer",
                    "expires_in": 7200
                }""",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        val token = refreshToken(
            tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
            refreshToken = "refresh-456",
            clientId = "test-client",
            clientSecret = "test-secret",
            client = httpClient,
        )

        assertEquals("new-access-789", token.accessToken)
        assertEquals("new-refresh-012", token.refreshToken)
        assertEquals(7200L, token.expiresIn)

        httpClient.close()
    }

    @Test
    fun refreshTokenErrorResponse() = runTest {
        val engine = MockEngine { _ ->
            respond(
                content = """{
                    "error": "invalid_grant",
                    "error_description": "Refresh token is invalid"
                }""",
                status = HttpStatusCode.Unauthorized,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        try {
            refreshToken(
                tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                refreshToken = "invalid-token",
                clientId = "test-client",
                clientSecret = "test-secret",
                client = httpClient,
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Auth) {
            assertTrue(e.message!!.contains("Refresh token is invalid"))
        }

        httpClient.close()
    }

    @Test
    fun refreshTokenSendsResourceWhenSetAndOmitsWhenUnset() = runTest {
        var lastBody = ""
        val engine = MockEngine { request ->
            lastBody = request.body.toByteArray().decodeToString()
            respond(
                content = """{"access_token": "new-access"}""",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        refreshToken(
            tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
            refreshToken = "refresh-456",
            clientId = "basecamp-cli",
            client = httpClient,
            resource = "urn:bc:account:42",
        )
        assertTrue(lastBody.contains("resource=urn%3Abc%3Aaccount%3A42"))

        refreshToken(
            tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
            refreshToken = "refresh-456",
            clientId = "basecamp-cli",
            client = httpClient,
        )
        assertTrue(!lastBody.contains("resource="))

        httpClient.close()
    }

    @Test
    fun refreshTokenCapturesResourceAndTreatsNullAsAbsent() = runTest {
        suspend fun tokenFor(resourceJson: String): OAuthToken {
            val engine = MockEngine { _ ->
                respond(
                    content = """{"access_token": "a", "resource": $resourceJson}""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
            val httpClient = HttpClient(engine)
            try {
                return refreshToken(
                    tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                    refreshToken = "refresh-456",
                    clientId = "basecamp-cli",
                    client = httpClient,
                )
            } finally {
                httpClient.close()
            }
        }

        assertEquals("urn:bc:account:42", tokenFor("\"urn:bc:account:42\"").resource)
        assertEquals(null, tokenFor("null").resource)
    }

    @Test
    fun refreshTokenRejectsNonStringResourceAsApiError() = runTest {
        // Mirrors conformance/oauth-token 05-resource-non-string-rejected: a
        // wrong-typed resource fails deserialization, surfaced as api_error.
        val engine = MockEngine { _ ->
            respond(
                content = """{"access_token": "a", "resource": 7}""",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        try {
            refreshToken(
                tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                refreshToken = "refresh-456",
                clientId = "basecamp-cli",
                client = httpClient,
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Api) {
            assertEquals("api_error", e.code)
        } finally {
            httpClient.close()
        }
    }

    @Test
    fun refreshTokenRejectsEmptyResourceAsApiError() = runTest {
        val engine = MockEngine { _ ->
            respond(
                content = """{"access_token": "a", "resource": ""}""",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        try {
            refreshToken(
                tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                refreshToken = "refresh-456",
                clientId = "basecamp-cli",
                client = httpClient,
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Api) {
            assertTrue(e.message!!.contains("resource"))
        } finally {
            httpClient.close()
        }
    }

    // =========================================================================
    // PKCE challenge is correct SHA-256 of verifier
    // =========================================================================

    @Test
    fun pkceChallengeIsCorrectSha256OfVerifier() {
        val pkce = generatePkce()

        // Compute expected challenge: base64url(sha256(verifier))
        val digest = MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(pkce.verifier.toByteArray(Charsets.US_ASCII))
        val expected = Base64.getUrlEncoder().withoutPadding().encodeToString(hash)

        assertEquals(expected, pkce.challenge, "PKCE challenge should be base64url(SHA-256(verifier))")
    }

    @Test
    fun exchangeCodeWithLegacyFormat() = runTest {
        val engine = MockEngine { request ->
            val body = request.body.toByteArray().decodeToString()
            assertTrue(body.contains("type=web_server"), "Legacy format should use type=web_server")
            assertTrue(!body.contains("grant_type"), "Legacy format should not use grant_type")

            respond(
                content = """{
                    "access_token": "legacy-access",
                    "token_type": "Bearer",
                    "expires_in": 1209600
                }""",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val httpClient = HttpClient(engine)
        val token = exchangeCode(
            tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
            code = "test-code",
            redirectUri = "https://myapp.com/callback",
            clientId = "test-client",
            clientSecret = "test-secret",
            useLegacyFormat = true,
            client = httpClient,
        )

        assertEquals("legacy-access", token.accessToken)

        httpClient.close()
    }

    // =========================================================================
    // Token-endpoint transport policy (SPEC §16): redirects refused, timeouts
    // mapped. Every test here injects its client, so the engine re-wrap path
    // (hardening applies to caller-supplied clients too) is what is exercised.
    // =========================================================================

    private val redirectStatuses = listOf(
        HttpStatusCode.MovedPermanently, // 301
        HttpStatusCode.Found, // 302
        HttpStatusCode.SeeOther, // 303
        HttpStatusCode.TemporaryRedirect, // 307
        HttpStatusCode.PermanentRedirect, // 308
    )

    @Test
    fun exchangeRefusesEveryRedirectStatus() = runTest {
        for (redirect in redirectStatuses) {
            val engine = MockEngine {
                respond(
                    content = """{"access_token": "planted-by-attacker"}""",
                    status = redirect,
                    headers = headersOf(HttpHeaders.Location, "https://attacker.example/token"),
                )
            }
            val httpClient = HttpClient(engine)
            try {
                val e = assertFailsWith<BasecampException.Api> {
                    exchangeCode(
                        tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                        code = "c",
                        redirectUri = "https://myapp.com/callback",
                        clientId = "id",
                        clientSecret = "s",
                        client = httpClient,
                    )
                }
                assertEquals(redirect.value, e.httpStatus, "the real status must survive classification")
                assertTrue(e.message!!.contains("not followed"), "message contract, got: ${e.message}")
                // Exactly one request: the Location target is never dialled.
                assertEquals(1, engine.requestHistory.size)
            } finally {
                httpClient.close()
            }
        }
    }

    @Test
    fun refreshRefusesEveryRedirectStatus() = runTest {
        for (redirect in redirectStatuses) {
            val engine = MockEngine {
                respond(
                    content = """{"access_token": "planted-by-attacker"}""",
                    status = redirect,
                    headers = headersOf(HttpHeaders.Location, "https://attacker.example/token"),
                )
            }
            val httpClient = HttpClient(engine)
            try {
                val e = assertFailsWith<BasecampException.Api> {
                    refreshToken(
                        tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                        refreshToken = "refresh-456",
                        clientId = "basecamp-cli",
                        client = httpClient,
                    )
                }
                assertEquals(redirect.value, e.httpStatus, "the real status must survive classification")
                assertTrue(e.message!!.contains("not followed"), "message contract, got: ${e.message}")
                assertEquals(1, engine.requestHistory.size)
            } finally {
                httpClient.close()
            }
        }
    }

    @Test
    fun exchange304StaysOnTheGenericBranch() = runTest {
        // 304 is a cache validator, not a Location redirect: it takes the
        // generic non-success classification, never the refused-redirect one.
        val engine = MockEngine {
            respond(content = "", status = HttpStatusCode.NotModified)
        }
        val httpClient = HttpClient(engine)
        try {
            val e = assertFailsWith<BasecampException.Auth> {
                exchangeCode(
                    tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                    code = "c",
                    redirectUri = "https://myapp.com/callback",
                    clientId = "id",
                    clientSecret = "s",
                    client = httpClient,
                )
            }
            assertTrue(e.message!!.contains("HTTP 304"))
            assertFalse(e.message!!.contains("not followed"))
        } finally {
            httpClient.close()
        }
    }

    @Test
    fun exchangeMapsRequestTimeoutToRetryableNetwork() = runTest {
        // HttpRequestTimeoutException subclasses CancellationException, so an
        // unmapped one would masquerade as a cooperative cancellation. Raised
        // from the handler because the wrapper's HttpTimeout timer runs on a
        // real dispatcher runTest's virtual clock cannot advance.
        val engine = MockEngine {
            throw HttpRequestTimeoutException(
                "https://launchpad.37signals.com/authorization/token",
                30_000L,
            )
        }
        val httpClient = HttpClient(engine)
        try {
            val e = assertFailsWith<BasecampException.Network> {
                refreshToken(
                    tokenEndpoint = "https://launchpad.37signals.com/authorization/token",
                    refreshToken = "refresh-456",
                    clientId = "basecamp-cli",
                    client = httpClient,
                )
            }
            assertTrue(e.message!!.contains("timed out"))
            assertTrue(e.retryable)
        } finally {
            httpClient.close()
        }
    }
}
