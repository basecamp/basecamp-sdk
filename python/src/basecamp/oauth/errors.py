from __future__ import annotations

from typing import Any, Literal

from basecamp.errors import BasecampError, ErrorCode

_OAUTH_TYPE_TO_CODE: dict[str, str] = {
    "validation": ErrorCode.VALIDATION,
    "auth": ErrorCode.AUTH,
    "network": ErrorCode.NETWORK,
    "api_error": ErrorCode.API,
}

# Hard resource-first selection/validation failures. These are RAISED, never
# returned as a soft fallback, so no consumer can convert one into a Launchpad
# request.
DiscoverySelectionReason = Literal[
    "ambiguous_issuers",
    "expected_issuer_unavailable",
    "invalid_issuer_origin",
    "as_fetch_failed",
    "issuer_mismatch",
    "capability_unavailable",
]


class OAuthError(BasecampError):
    """OAuth-specific error with a type classifier.

    Types: "validation", "auth", "network", "api_error"
    """

    def __init__(self, oauth_type: str, message: str, **kwargs: Any):
        code = _OAUTH_TYPE_TO_CODE.get(oauth_type, ErrorCode.API)
        super().__init__(message, code=code, **kwargs)
        self.oauth_type = oauth_type


class DiscoverySelectionError(OAuthError):
    """Hard resource-first selection/validation failure.

    Raised — never returned as a fallback — so no consumer can convert it into a
    Launchpad request. The ``reason`` attribute distinguishes the specific hard
    case (see :data:`DiscoverySelectionReason`).

    ``capability_unavailable`` and ``expected_issuer_unavailable`` are
    consumer/usage-shaped; the remaining reasons are AS-metadata faults surfaced
    as ``api_error``.
    """

    def __init__(self, reason: DiscoverySelectionReason, message: str, **kwargs: Any):
        oauth_type = "validation" if reason == "capability_unavailable" else "api_error"
        super().__init__(oauth_type, message, **kwargs)
        self.reason: DiscoverySelectionReason = reason
