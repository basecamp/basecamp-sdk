"""OAuth 2.0 discovery for the Basecamp SDK.

Two composable operations plus an orchestrator (SPEC.md §16):

  - ``discover(base_url)``                    — RFC 8414 AS metadata + issuer binding
  - ``discover_protected_resource(origin)``   — RFC 9728 resource metadata
  - ``discover_from_resource(origin, ...)``   — resource-first selection + fallback

All fetches are SSRF-hardened: HTTPS-only origins (localhost exempt), the origin
is parsed/validated with the transport URL parser before any socket opens,
redirects are suppressed, timeouts are bounded, and bodies are read under a
genuine streaming cap that aborts before the whole oversized body is buffered.
"""

from __future__ import annotations

import json
import time
from typing import Any

import httpx

from basecamp._security import require_origin_root, truncate
from basecamp.errors import BasecampError
from basecamp.oauth.config import (
    DiscoveryResult,
    FallbackReason,
    OAuthConfig,
    ProtectedResourceMetadata,
)
from basecamp.oauth.errors import DiscoverySelectionError, OAuthError

LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com"

_DISCOVERY_TIMEOUT = 10.0

# Cap on a discovery response body (1 MiB) — discovery documents are tiny.
MAX_DISCOVERY_BODY_BYTES = 1 * 1024 * 1024


def _is_str_list(value: Any) -> bool:
    """True iff ``value`` is a list whose every element is a ``str``."""
    return isinstance(value, list) and all(isinstance(v, str) for v in value)


class _IssuerBindingError(OAuthError):
    """Structured marker: AS metadata failed the RFC 8414 issuer code-point bind.

    Kept module-private (and deliberately NOT in ``errors.py``, which device flow
    shares) so :func:`discover_from_resource` classifies an issuer mismatch by
    ``isinstance`` — never by substring-matching an exception message, which is
    brittle and locale/wording-sensitive.
    """

    def __init__(self, message: str, **kwargs: Any) -> None:
        super().__init__("api_error", message, **kwargs)


def _fetch_discovery_document(url: str, timeout: float, max_body_bytes: int) -> Any:
    """SSRF-hardened GET of a discovery document.

    The origin must already be validated (via :func:`require_origin_root`); this
    suppresses redirects, bounds the timeout, reads the body under a genuine
    streaming cap that aborts once ``max_body_bytes`` is exceeded, and maps any
    non-2xx status to ``api_error`` (not ``network``).
    """
    try:
        with httpx.stream(
            "GET",
            url,
            headers={"Accept": "application/json"},
            timeout=timeout,
            follow_redirects=False,
        ) as response:
            # httpx's timeout is per-operation and RESETS after each received
            # chunk, so a peer dripping one byte at a time never trips the read
            # timeout and can hold the caller arbitrarily long. Bound the WHOLE
            # response with a wall-clock deadline so a slow-drip stream is aborted
            # as a retryable network timeout regardless of chunk cadence.
            deadline = time.monotonic() + timeout
            chunks: list[bytes] = []
            total = 0
            for chunk in response.iter_bytes():
                if time.monotonic() > deadline:
                    raise OAuthError("network", "OAuth discovery timed out", retryable=True)
                total += len(chunk)
                if total > max_body_bytes:
                    # Abort the stream — leaving the ``with`` closes the
                    # connection, so the oversized body is never fully buffered.
                    raise OAuthError("api_error", "OAuth discovery response exceeds size cap")
                chunks.append(chunk)
            status = response.status_code
            body = b"".join(chunks)
    except httpx.TimeoutException as exc:
        raise OAuthError("network", "OAuth discovery timed out", retryable=True) from exc
    except httpx.HTTPError as exc:
        raise OAuthError("network", f"OAuth discovery failed: {exc}", retryable=True) from exc

    # A suppressed redirect (3xx) surfaces here as a non-2xx api_error rather
    # than a followed request to an attacker-influenced Location.
    if not 200 <= status < 300:
        raise OAuthError(
            "api_error",
            f"OAuth discovery failed with status {status}: {truncate(body.decode(errors='replace'))}",
            http_status=status,
        )

    try:
        return json.loads(body)
    except ValueError as exc:
        raise OAuthError("api_error", f"Failed to parse OAuth discovery response: {exc}") from exc


def discover(
    base_url: str,
    *,
    timeout: float = _DISCOVERY_TIMEOUT,
    max_body_bytes: int = MAX_DISCOVERY_BODY_BYTES,
) -> OAuthConfig:
    """Discover OAuth 2.0 Authorization Server Metadata (RFC 8414).

    GETs ``{base_url}/.well-known/oauth-authorization-server`` and binds it: the
    returned ``issuer`` must equal the requested origin by code-point (no
    normalization beyond origin-root parsing). ``token_endpoint`` is required;
    ``authorization_endpoint`` is optional (device-only servers omit it).

    Raises :class:`~basecamp.errors.UsageError` on a malformed origin and
    :class:`OAuthError` (``api_error``) on invalid metadata.
    """
    issuer_origin = require_origin_root(base_url, "OAuth discovery base URL")
    url = f"{issuer_origin}/.well-known/oauth-authorization-server"

    data = _fetch_discovery_document(url, timeout, max_body_bytes)
    if not isinstance(data, dict):
        raise OAuthError("api_error", "OAuth discovery response is not a JSON object")

    return _parse_and_bind_as_metadata(data, issuer_origin)


def _parse_and_bind_as_metadata(data: dict[str, Any], expected_issuer_origin: str) -> OAuthConfig:
    """Validate AS metadata and bind ``issuer`` to ``expected_issuer_origin`` by
    code-point. Universal validation only: ``issuer`` + ``token_endpoint``
    present and non-empty, and any present ``*_endpoint`` field non-empty.
    Per-grant endpoint checks are the consumer's responsibility.
    """
    issuer = data.get("issuer")
    if not isinstance(issuer, str) or not issuer:
        raise OAuthError("api_error", "Invalid OAuth discovery response: missing required fields: issuer")
    # RFC 8414 §3.3/§4: issuer identical by code-point. No normalization.
    if issuer != expected_issuer_origin:
        raise _IssuerBindingError(
            f'OAuth issuer mismatch: metadata issuer "{issuer}" does not equal "{expected_issuer_origin}"',
        )
    if not isinstance(data.get("token_endpoint"), str) or not data.get("token_endpoint"):
        raise OAuthError("api_error", "Invalid OAuth discovery response: missing required fields: token_endpoint")
    # Every present ``*_endpoint`` field must be a non-empty string — reject "",
    # lists, numbers, null (a non-string endpoint is malformed, not merely empty).
    for key, value in data.items():
        if key.endswith("_endpoint") and (not isinstance(value, str) or value == ""):
            raise OAuthError("api_error", f"Invalid OAuth discovery response: invalid {key}")
    # List-valued metadata fields, when present, must be arrays of strings —
    # never a bare string (substring-matching one would falsely enable a grant
    # or scope) nor a list carrying non-string members.
    for list_field in (
        "grant_types_supported",
        "scopes_supported",
        "code_challenge_methods_supported",
    ):
        value = data.get(list_field)
        if value is not None and not _is_str_list(value):
            raise OAuthError(
                "api_error",
                f"Invalid OAuth discovery response: {list_field} must be an array of strings",
            )

    return OAuthConfig(
        issuer=issuer,
        token_endpoint=data["token_endpoint"],
        authorization_endpoint=data.get("authorization_endpoint"),
        device_authorization_endpoint=data.get("device_authorization_endpoint"),
        registration_endpoint=data.get("registration_endpoint"),
        scopes_supported=data.get("scopes_supported"),
        grant_types_supported=data.get("grant_types_supported"),
        code_challenge_methods_supported=data.get("code_challenge_methods_supported"),
    )


def discover_protected_resource(
    resource_origin: str,
    *,
    timeout: float = _DISCOVERY_TIMEOUT,
    max_body_bytes: int = MAX_DISCOVERY_BODY_BYTES,
) -> ProtectedResourceMetadata:
    """Discover RFC 9728 protected-resource metadata.

    GETs ``{resource_origin}/.well-known/oauth-protected-resource``. ``resource``
    is required and must equal the requested origin by code-point.
    ``authorization_servers`` is OPTIONAL and preserved distinctly as absent
    (``None``) vs present-but-empty (``[]``).

    Raises :class:`~basecamp.errors.UsageError` on a malformed caller origin and
    :class:`OAuthError` (``api_error``) on invalid metadata.
    """
    origin = require_origin_root(resource_origin, "resource origin")
    url = f"{origin}/.well-known/oauth-protected-resource"

    data = _fetch_discovery_document(url, timeout, max_body_bytes)
    if not isinstance(data, dict):
        raise OAuthError("api_error", "Resource metadata response is not a JSON object")

    resource = data.get("resource")
    if not isinstance(resource, str) or not resource:
        raise OAuthError("api_error", "Invalid resource metadata: missing required field: resource")
    # Bind the resource identifier to the requested origin, code-point exact.
    if resource != origin:
        raise OAuthError(
            "api_error",
            f'Resource identifier mismatch: metadata resource "{resource}" does not equal "{origin}"',
        )

    # Preserve absent (None) vs present-empty ([]). When present it must be a list
    # of strings — a bare string previously slipped through and was iterated
    # char-by-char during selection; reject it as malformed so the orchestrator
    # soft-falls-back. A present-but-null value normalizes to [].
    authorization_servers: list[str] | None
    if "authorization_servers" not in data:
        authorization_servers = None
    else:
        servers = data.get("authorization_servers")
        if servers is None:
            authorization_servers = []
        elif _is_str_list(servers):
            authorization_servers = servers
        else:
            raise OAuthError(
                "api_error",
                "Invalid resource metadata: authorization_servers must be an array of strings",
            )

    return ProtectedResourceMetadata(resource=resource, authorization_servers=authorization_servers)


def _is_launchpad_issuer(issuer: str) -> bool:
    """True when an issuer string is a valid origin root equal to Launchpad's.

    Both sides run through :func:`require_origin_root`, so an advertised
    look-alike that is not a clean origin root — e.g.
    ``https://launchpad.37signals.com/path`` (path), userinfo, or a query — is
    not treated as Launchpad. It stays a non-Launchpad candidate and later fails
    hard (``ambiguous_issuers`` / ``invalid_issuer_origin``) rather than being
    silently excluded. A trailing-slash-only origin root still matches because
    ``require_origin_root`` normalizes it away.
    """
    try:
        return require_origin_root(issuer, "issuer") == require_origin_root(LAUNCHPAD_BASE_URL, "issuer")
    except BasecampError:
        return False


def discover_from_resource(
    resource_origin: str,
    *,
    expected_issuer: str | None = None,
    timeout: float = _DISCOVERY_TIMEOUT,
    max_body_bytes: int = MAX_DISCOVERY_BODY_BYTES,
) -> DiscoveryResult:
    """Resource-first discovery orchestrator (SPEC.md §16).

    Composes RFC 9728 + RFC 8414 and applies the stage-sensitive fallback state
    machine. Returns a :class:`DiscoveryResult` that is either ``selected`` (with
    ``config`` and ``issuer``) or a soft ``fallback`` whose ``reason`` is one of
    :class:`FallbackReason` ONLY. Every hard failure raises
    :class:`DiscoverySelectionError`; callers MUST NOT convert a raise into a
    Launchpad request.
    """
    # Origin-root validation of the *caller's* input is a usage error — let it
    # propagate as-is (never a soft fallback).
    origin = require_origin_root(resource_origin, "resource origin")

    # --- Hop 1: resource metadata. Failure here is soft (before selection). ---
    try:
        resource = discover_protected_resource(origin, timeout=timeout, max_body_bytes=max_body_bytes)
    except OAuthError:
        return DiscoveryResult(kind="fallback", reason=FallbackReason.RESOURCE_DISCOVERY_FAILED)

    advertised = resource.authorization_servers or []

    # --- Selection ---
    if expected_issuer is not None:
        selected = next((s for s in advertised if s == expected_issuer), None)
        if selected is None:
            raise DiscoverySelectionError(
                "expected_issuer_unavailable",
                f'Expected issuer "{expected_issuer}" is not advertised by the resource',
            )
    else:
        # Dedupe by code-point (order-preserving): the same non-Launchpad issuer
        # advertised more than once is ONE candidate, not an ambiguity.
        non_launchpad = list(dict.fromkeys(s for s in advertised if not _is_launchpad_issuer(s)))
        if len(non_launchpad) >= 2:
            raise DiscoverySelectionError(
                "ambiguous_issuers",
                "Multiple non-Launchpad issuers advertised; pass expected_issuer to disambiguate: "
                + ", ".join(non_launchpad),
            )
        if not non_launchpad:
            # Valid resource metadata omits BC5 — soft fallback (before selection).
            return DiscoveryResult(kind="fallback", reason=FallbackReason.NO_AS_ADVERTISED)
        selected = non_launchpad[0]

    # --- BC5 is now committed: every subsequent failure is fatal (no Launchpad). ---
    try:
        issuer_origin = require_origin_root(selected, "advertised issuer")
    except BasecampError as exc:
        # A bad *advertised* issuer origin is a hard classification failure, not
        # the usage error a bad caller origin would be.
        raise DiscoverySelectionError(
            "invalid_issuer_origin",
            f'Advertised issuer "{selected}" is not a valid origin root',
        ) from exc

    try:
        config = discover(issuer_origin, timeout=timeout, max_body_bytes=max_body_bytes)
    except _IssuerBindingError as exc:
        # A structured marker — not a message substring — distinguishes an
        # issuer-binding mismatch from a generic fetch failure.
        raise DiscoverySelectionError("issuer_mismatch", str(exc)) from exc
    except OAuthError as exc:
        raise DiscoverySelectionError(
            "as_fetch_failed",
            f'AS metadata fetch failed for committed issuer "{issuer_origin}": {exc}',
        ) from exc

    return DiscoveryResult(kind="selected", config=config, issuer=config.issuer)


def discover_launchpad(
    *,
    timeout: float = _DISCOVERY_TIMEOUT,
    max_body_bytes: int = MAX_DISCOVERY_BODY_BYTES,
) -> OAuthConfig:
    """Convenience wrapper: discover configuration from Launchpad."""
    return discover(LAUNCHPAD_BASE_URL, timeout=timeout, max_body_bytes=max_body_bytes)
