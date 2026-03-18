from __future__ import annotations

import time
from dataclasses import dataclass, field


@dataclass(frozen=True)
class OAuthToken:
    """OAuth 2 access token response."""

    access_token: str
    token_type: str = "Bearer"
    refresh_token: str | None = None
    expires_in: int | None = None
    expires_at: float | None = field(default=None)
    scope: str | None = None

    def __post_init__(self) -> None:
        # Calculate expires_at from expires_in when not explicitly provided.
        if self.expires_at is None and self.expires_in is not None:
            object.__setattr__(self, "expires_at", time.time() + self.expires_in)

    def is_expired(self, buffer_seconds: int = 60) -> bool:
        """Check if the token is expired or will expire within *buffer_seconds*."""
        if self.expires_at is None:
            return False
        return time.time() + buffer_seconds >= self.expires_at
