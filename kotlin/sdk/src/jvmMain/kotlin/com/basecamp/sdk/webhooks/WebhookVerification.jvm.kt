package com.basecamp.sdk.webhooks

import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec
import java.security.MessageDigest

actual fun verifyWebhookSignature(payload: ByteArray, signature: String, secret: String): Boolean {
    if (secret.isBlank() || signature.isBlank()) return false
    val expected = computeWebhookSignature(payload, secret)
    // Constant-time comparison to prevent timing attacks
    return MessageDigest.isEqual(expected.toByteArray(), signature.toByteArray())
}

actual fun computeWebhookSignature(payload: ByteArray, secret: String): String {
    val mac = Mac.getInstance("HmacSHA256")
    mac.init(SecretKeySpec(secret.toByteArray(), "HmacSHA256"))
    val hash = mac.doFinal(payload)
    return hash.joinToString("") { "%02x".format(it) }
}
