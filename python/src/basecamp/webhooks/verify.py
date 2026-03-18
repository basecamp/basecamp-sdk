from __future__ import annotations

import hashlib
import hmac


def compute_signature(payload: bytes, secret: str) -> str:
    """Compute HMAC-SHA256 hex digest for a webhook payload."""
    return hmac.new(secret.encode(), payload, hashlib.sha256).hexdigest()


def verify_signature(payload: bytes, secret: str, signature: str) -> bool:
    """Verify an HMAC-SHA256 signature for a webhook payload.

    Returns False if secret or signature is empty.
    Uses constant-time comparison to prevent timing attacks.
    """
    if not secret or not signature:
        return False

    expected = compute_signature(payload, secret)
    return hmac.compare_digest(expected, signature)
