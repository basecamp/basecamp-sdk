from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class OAuthConfig:
    """OAuth 2 server configuration from discovery endpoint."""

    issuer: str
    authorization_endpoint: str
    token_endpoint: str
    registration_endpoint: str | None = None
    scopes_supported: list[str] | None = None
