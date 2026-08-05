from __future__ import annotations

import time
from dataclasses import dataclass, field


@dataclass(frozen=True)
class OAuthToken:
    """OAuth 2 access token response."""

    access_token: str
    # RFC 6750 authentication scheme name, not a credential.
    token_type: str = "Bearer"  # noqa: S105
    refresh_token: str | None = None
    expires_in: int | None = None
    expires_at: float | None = field(default=None)
    scope: str | None = None
    #: RFC 8707 resource indicator the token is bound to (BC5:
    #: ``urn:bc:account:<id>``). Echo it as the ``resource`` parameter when
    #: refreshing — BC5 multi-account refresh tokens reject a refresh
    #: without it (SPEC §16). Appended last: earlier fields keep their
    #: positional slots.
    resource: str | None = None

    def __post_init__(self) -> None:
        # Calculate expires_at from expires_in when not explicitly provided.
        if self.expires_at is None and self.expires_in is not None:
            object.__setattr__(self, "expires_at", time.time() + self.expires_in)

    def is_expired(self, buffer_seconds: int = 60) -> bool:
        """Check if the token is expired or will expire within *buffer_seconds*."""
        if self.expires_at is None:
            return False
        return time.time() + buffer_seconds >= self.expires_at
