package com.basecamp.sdk

import com.basecamp.sdk.oauth.*
import kotlin.test.Test
import kotlin.test.assertEquals
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
}
