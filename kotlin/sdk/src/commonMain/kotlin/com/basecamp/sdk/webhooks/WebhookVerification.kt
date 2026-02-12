package com.basecamp.sdk.webhooks

/**
 * Webhook signature verification for Basecamp webhooks.
 *
 * Basecamp signs webhook payloads with HMAC-SHA256 using the webhook's
 * secret key. The signature is sent in the `X-Basecamp-Signature` header
 * as a hex-encoded string.
 *
 * ```kotlin
 * val isValid = verifyWebhookSignature(
 *     payload = requestBody,
 *     signature = headers["X-Basecamp-Signature"],
 *     secret = webhookSecret,
 * )
 * ```
 */
expect fun verifyWebhookSignature(payload: ByteArray, signature: String, secret: String): Boolean

/**
 * Computes the HMAC-SHA256 signature for a webhook payload.
 * Useful for testing webhook handlers.
 */
expect fun computeWebhookSignature(payload: ByteArray, secret: String): String

/**
 * Verifies a webhook signature from string payload.
 */
fun verifyWebhookSignature(payload: String, signature: String, secret: String): Boolean {
    if (secret.isBlank() || signature.isBlank()) return false
    return verifyWebhookSignature(payload.encodeToByteArray(), signature, secret)
}

/**
 * Computes the HMAC-SHA256 signature for a string payload.
 */
fun computeWebhookSignature(payload: String, secret: String): String =
    computeWebhookSignature(payload.encodeToByteArray(), secret)
