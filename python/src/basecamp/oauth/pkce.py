from __future__ import annotations

import base64
import hashlib
import os
from typing import NamedTuple


class PKCE(NamedTuple):
    """PKCE code verifier / challenge pair."""

    verifier: str
    challenge: str
    method: str


def _base64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def generate_pkce() -> PKCE:
    """Generate a cryptographically secure PKCE verifier and S256 challenge.

    The verifier is 43 characters (32 random bytes, base64url-encoded without
    padding).  The challenge is the base64url-encoded SHA-256 hash of the
    verifier.
    """
    verifier = _base64url(os.urandom(32))
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = _base64url(digest)
    return PKCE(verifier=verifier, challenge=challenge, method="S256")


def generate_state() -> str:
    """Generate a cryptographically secure OAuth state parameter.

    Returns 43 characters (32 random bytes, base64url-encoded without padding).
    """
    return _base64url(os.urandom(32))
