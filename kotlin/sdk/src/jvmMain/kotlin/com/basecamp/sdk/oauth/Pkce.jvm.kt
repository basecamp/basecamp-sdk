package com.basecamp.sdk.oauth

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

actual fun generatePkce(): Pkce {
    val random = SecureRandom()
    val bytes = ByteArray(32)
    random.nextBytes(bytes)
    val verifier = base64UrlEncode(bytes)

    val digest = MessageDigest.getInstance("SHA-256")
    val challengeBytes = digest.digest(verifier.toByteArray(Charsets.US_ASCII))
    val challenge = base64UrlEncode(challengeBytes)

    return Pkce(verifier = verifier, challenge = challenge)
}

actual fun generateState(): String {
    val random = SecureRandom()
    val bytes = ByteArray(16)
    random.nextBytes(bytes)
    return base64UrlEncode(bytes)
}

private fun base64UrlEncode(bytes: ByteArray): String =
    Base64.getUrlEncoder().withoutPadding().encodeToString(bytes)
