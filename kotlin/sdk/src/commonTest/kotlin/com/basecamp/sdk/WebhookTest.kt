package com.basecamp.sdk

import com.basecamp.sdk.webhooks.computeWebhookSignature
import com.basecamp.sdk.webhooks.verifyWebhookSignature
import kotlin.test.Test
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class WebhookTest {

    private val secret = "test-webhook-secret"
    private val payload = """{"id": 123, "kind": "todo_created"}"""

    @Test
    fun verifyValidSignature() {
        val signature = computeWebhookSignature(payload, secret)
        assertTrue(verifyWebhookSignature(payload, signature, secret))
    }

    @Test
    fun rejectInvalidSignature() {
        assertFalse(verifyWebhookSignature(payload, "invalid-signature", secret))
    }

    @Test
    fun rejectEmptySecret() {
        val signature = computeWebhookSignature(payload, secret)
        assertFalse(verifyWebhookSignature(payload, signature, ""))
    }

    @Test
    fun rejectEmptySignature() {
        assertFalse(verifyWebhookSignature(payload, "", secret))
    }

    @Test
    fun rejectWrongSecret() {
        val signature = computeWebhookSignature(payload, secret)
        assertFalse(verifyWebhookSignature(payload, signature, "wrong-secret"))
    }

    @Test
    fun rejectTamperedPayload() {
        val signature = computeWebhookSignature(payload, secret)
        val tampered = payload.replace("123", "456")
        assertFalse(verifyWebhookSignature(tampered, signature, secret))
    }

    @Test
    fun signatureIsDeterministic() {
        val sig1 = computeWebhookSignature(payload, secret)
        val sig2 = computeWebhookSignature(payload, secret)
        assertTrue(sig1 == sig2)
    }

    @Test
    fun signatureIsHexEncoded() {
        val signature = computeWebhookSignature(payload, secret)
        assertTrue(signature.length == 64) // SHA-256 = 32 bytes = 64 hex chars
        assertTrue(signature.all { it in '0'..'9' || it in 'a'..'f' })
    }

    @Test
    fun byteArrayOverloadWorks() {
        val bytes = payload.encodeToByteArray()
        val signature = computeWebhookSignature(bytes, secret)
        assertTrue(verifyWebhookSignature(bytes, signature, secret))
    }
}
