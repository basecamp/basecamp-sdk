package basecamp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// VerifyWebhookSignature checks that the given payload matches the HMAC-SHA256 signature.
// Returns false if secret or signature is empty.
func VerifyWebhookSignature(payload []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}
	expected := ComputeWebhookSignature(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ComputeWebhookSignature computes the HMAC-SHA256 signature for a webhook payload.
func ComputeWebhookSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
