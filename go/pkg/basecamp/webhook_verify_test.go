package basecamp

import (
	"testing"
)

func TestComputeWebhookSignature_Deterministic(t *testing.T) {
	payload := []byte(`{"id":1,"kind":"todo_created"}`)
	secret := "test-secret"

	sig1 := ComputeWebhookSignature(payload, secret)
	sig2 := ComputeWebhookSignature(payload, secret)

	if sig1 != sig2 {
		t.Errorf("expected deterministic signatures, got %q and %q", sig1, sig2)
	}
	if sig1 == "" {
		t.Error("expected non-empty signature")
	}
}

func TestVerifyWebhookSignature_RoundTrip(t *testing.T) {
	payload := []byte(`{"id":1,"kind":"todo_created","details":{}}`)
	secret := "my-webhook-secret"

	sig := ComputeWebhookSignature(payload, secret)
	if !VerifyWebhookSignature(payload, sig, secret) {
		t.Error("expected signature to verify")
	}
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	payload := []byte(`{"id":1,"kind":"todo_created"}`)
	sig := ComputeWebhookSignature(payload, "correct-secret")

	if VerifyWebhookSignature(payload, sig, "wrong-secret") {
		t.Error("expected verification to fail with wrong secret")
	}
}

func TestVerifyWebhookSignature_TamperedPayload(t *testing.T) {
	payload := []byte(`{"id":1,"kind":"todo_created"}`)
	secret := "test-secret"
	sig := ComputeWebhookSignature(payload, secret)

	tampered := []byte(`{"id":1,"kind":"todo_deleted"}`)
	if VerifyWebhookSignature(tampered, sig, secret) {
		t.Error("expected verification to fail with tampered payload")
	}
}

func TestVerifyWebhookSignature_EmptySecret(t *testing.T) {
	payload := []byte(`{"id":1}`)
	sig := ComputeWebhookSignature(payload, "some-secret")

	if VerifyWebhookSignature(payload, sig, "") {
		t.Error("expected false with empty secret")
	}
}

func TestVerifyWebhookSignature_EmptySignature(t *testing.T) {
	payload := []byte(`{"id":1}`)

	if VerifyWebhookSignature(payload, "", "some-secret") {
		t.Error("expected false with empty signature")
	}
}

func TestVerifyWebhookSignature_BothEmpty(t *testing.T) {
	payload := []byte(`{"id":1}`)

	if VerifyWebhookSignature(payload, "", "") {
		t.Error("expected false with both empty")
	}
}

func TestComputeWebhookSignature_DifferentPayloads(t *testing.T) {
	secret := "test-secret"
	sig1 := ComputeWebhookSignature([]byte(`{"id":1}`), secret)
	sig2 := ComputeWebhookSignature([]byte(`{"id":2}`), secret)

	if sig1 == sig2 {
		t.Error("expected different signatures for different payloads")
	}
}

func TestComputeWebhookSignature_DifferentSecrets(t *testing.T) {
	payload := []byte(`{"id":1}`)
	sig1 := ComputeWebhookSignature(payload, "secret-a")
	sig2 := ComputeWebhookSignature(payload, "secret-b")

	if sig1 == sig2 {
		t.Error("expected different signatures for different secrets")
	}
}
