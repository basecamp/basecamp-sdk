from __future__ import annotations

import base64
import hashlib
import re

from basecamp.oauth.pkce import generate_pkce, generate_state


class TestGeneratePkce:
    def test_generate_pkce(self):
        pkce = generate_pkce()

        # Verifier is 43 chars of base64url (32 random bytes encoded)
        assert len(pkce.verifier) == 43
        assert re.fullmatch(r"[A-Za-z0-9_-]+", pkce.verifier)

        # Challenge is SHA-256 of verifier, base64url-encoded without padding
        expected_digest = hashlib.sha256(pkce.verifier.encode("ascii")).digest()
        expected_challenge = base64.urlsafe_b64encode(expected_digest).rstrip(b"=").decode("ascii")
        assert pkce.challenge == expected_challenge

        # Method is S256
        assert pkce.method == "S256"

    def test_pkce_uniqueness(self):
        a = generate_pkce()
        b = generate_pkce()

        assert a.verifier != b.verifier
        assert a.challenge != b.challenge


class TestGenerateState:
    def test_generate_state(self):
        state = generate_state()

        assert len(state) == 43
        assert re.fullmatch(r"[A-Za-z0-9_-]+", state)

    def test_state_uniqueness(self):
        assert generate_state() != generate_state()
