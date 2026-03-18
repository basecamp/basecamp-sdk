from __future__ import annotations

import hashlib
import hmac

from basecamp.webhooks.verify import compute_signature, verify_signature


class TestComputeSignature:
    def test_compute_signature(self):
        payload = b'{"id":1,"kind":"todo_created"}'
        secret = "webhook-secret-key"

        result = compute_signature(payload, secret)

        expected = hmac.new(secret.encode(), payload, hashlib.sha256).hexdigest()
        assert result == expected
        assert len(result) == 64  # SHA-256 hex digest length


class TestVerifySignature:
    def test_verify_valid_signature(self):
        payload = b'{"id":42,"kind":"message_created"}'
        secret = "test-secret"
        signature = compute_signature(payload, secret)

        assert verify_signature(payload, secret, signature) is True

    def test_verify_invalid_signature(self):
        payload = b'{"id":42,"kind":"message_created"}'
        secret = "test-secret"

        assert verify_signature(payload, secret, "bad-signature") is False

    def test_verify_empty_inputs(self):
        payload = b'{"id":1}'

        assert verify_signature(payload, "", "some-sig") is False
        assert verify_signature(payload, "secret", "") is False
        assert verify_signature(payload, "", "") is False
