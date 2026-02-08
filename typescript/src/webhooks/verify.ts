import { createHmac, timingSafeEqual } from "node:crypto";

/**
 * Verify an HMAC-SHA256 webhook signature.
 * Returns false if secret or signature is empty.
 */
export function verifyWebhookSignature(
  payload: string | Buffer,
  signature: string,
  secret: string,
): boolean {
  if (!secret || !signature) return false;

  const expected = signWebhookPayload(payload, secret);
  try {
    return timingSafeEqual(Buffer.from(expected), Buffer.from(signature));
  } catch {
    // Lengths differ
    return false;
  }
}

/**
 * Compute the HMAC-SHA256 signature for a webhook payload.
 * Useful for testing webhook handlers.
 */
export function signWebhookPayload(
  payload: string | Buffer,
  secret: string,
): string {
  return createHmac("sha256", secret).update(payload).digest("hex");
}
