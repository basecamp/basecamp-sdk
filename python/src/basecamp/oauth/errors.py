from __future__ import annotations

from typing import Any, Literal

from basecamp.errors import BasecampError, ErrorCode

_OAUTH_TYPE_TO_CODE: dict[str, str] = {
    "validation": ErrorCode.VALIDATION,
    "auth": ErrorCode.AUTH,
    "network": ErrorCode.NETWORK,
    "api_error": ErrorCode.API,
    "usage": ErrorCode.USAGE,
}

# Why a device authorization grant (RFC 8628) terminated. The parent error
# category is DERIVED from the reason (SPEC.md §16), so callers can branch on
# either ``.reason`` (precise) or ``.code``/``.exit_code`` (coarse).
DeviceFlowReason = Literal[
    "access_denied",
    "expired",
    "transport",
    "unavailable",
    "cancelled",
]

# Maps a device-flow reason to its parent OAuth type (and thus BasecampError
# category): access_denied/expired → auth, transport → network (retryable),
# unavailable → validation, cancelled → usage.
_DEVICE_REASON_TO_TYPE: dict[str, str] = {
    "access_denied": "auth",
    "expired": "auth",
    "transport": "network",
    "unavailable": "validation",
    "cancelled": "usage",
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

    Types: "validation", "auth", "network", "api_error", "usage"
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

    Only ``capability_unavailable`` is consumer/usage-shaped (``validation``).
    Every other reason — including ``expected_issuer_unavailable`` — is an
    AS-metadata fault surfaced as ``api_error``, matching the other four SDKs (an
    issuer the resource does not advertise is a metadata fault, not caller usage).
    """

    def __init__(self, reason: DiscoverySelectionReason, message: str, **kwargs: Any):
        oauth_type = "validation" if reason == "capability_unavailable" else "api_error"
        super().__init__(oauth_type, message, **kwargs)
        self.reason: DiscoverySelectionReason = reason


class DeviceFlowError(OAuthError):
    """Terminal RFC 8628 device authorization grant error.

    Carries a :data:`DeviceFlowReason`; the parent OAuth type — and thus the
    BasecampError ``code``/``exit_code`` — is DERIVED from that reason. Only a
    ``transport`` failure is retryable.
    """

    def __init__(self, reason: DeviceFlowReason, message: str, **kwargs: Any):
        # Default an unrecognized reason to api_error rather than raising a raw
        # KeyError — mirrors OAuthError's defensive `.get()` on unknown types.
        oauth_type = _DEVICE_REASON_TO_TYPE.get(reason, "api_error")
        # The reason is authoritative for retryability (only ``transport`` is
        # retryable). Overwrite — never setdefault — so a caller's ``retryable``
        # kwarg cannot flip the invariant (transport → non-retryable, or a
        # terminal denial → retryable).
        kwargs["retryable"] = reason == "transport"
        super().__init__(oauth_type, message, **kwargs)
        self.reason: DeviceFlowReason = reason
