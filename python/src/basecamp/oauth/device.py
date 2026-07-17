"""RFC 8628 device authorization grant — request, poll, and orchestration.

Three synchronous functions (SPEC.md §16):

  - ``request_device_authorization`` obtains a device/user code pair.
  - ``poll_device_token`` runs the §3.5 polling loop against the token endpoint.
  - ``perform_device_login`` orchestrates both against an already-selected config.

Both HTTP calls are TLS-guarded (HTTPS required off localhost, §9). The polling
clock and sleep are injectable so tests can drive the interval schedule
(slow_down, backoff) and expiry deterministically, with no real delays.
"""

from __future__ import annotations

import json
import math
import threading
import time
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

import httpx

from basecamp._security import is_localhost, require_https
from basecamp.oauth.config import OAuthConfig
from basecamp.oauth.device_authorization import DeviceAuthorization
from basecamp.oauth.discovery import _normalize_timeout
from basecamp.oauth.errors import DeviceFlowError, OAuthError
from basecamp.oauth.token import OAuthToken

#: URN grant type for the device authorization grant (RFC 8628 §3.4).
DEVICE_CODE_GRANT_TYPE = "urn:ietf:params:oauth:grant-type:device_code"

#: Default polling interval when the server omits ``interval`` (RFC 8628 §3.2).
DEFAULT_INTERVAL_SECONDS = 5

#: ``slow_down`` bumps the interval by this many seconds, sustained (§3.5).
SLOW_DOWN_INCREMENT_SECONDS = 5

#: Cap on exponential backoff after connection timeouts.
MAX_BACKOFF_SECONDS = 60

#: Ceiling for ``expires_in``/``interval``: 2147483 s (~24.8 days) is the largest
#: whole-second duration whose millisecond form fits a 32-bit signed timer.
#: Shared across all five SDKs (SPEC.md) — an unbounded value such as 1e100 is
#: a malformed response, not a schedulable deadline.
MAX_DEVICE_SECONDS = 2_147_483

#: Ceiling for an OAuth token's ``expires_in`` (2_147_483_647 s ≈ 68 years):
#: cross-runtime safe and vastly beyond any realistic token lifetime. Unlike
#: :data:`MAX_DEVICE_SECONDS` this bounds ``expires_at`` arithmetic rather than a
#: timer, so a non-finite (``1e400`` parses to ``inf``) or absurd value is a
#: malformed response — never a schedulable deadline. Shared across all five SDKs.
MAX_TOKEN_LIFETIME_SECONDS = 2_147_483_647

_DEVICE_TIMEOUT = 30.0

# Cap on a device-flow response body (1 MiB) — these responses are tiny; a
# larger one is a fault, so abort rather than buffer it. Mirrors discovery.
MAX_DEVICE_BODY_BYTES = 1 * 1024 * 1024

_FORM_HEADERS = {
    "Content-Type": "application/x-www-form-urlencoded",
    "Accept": "application/json",
}


def _post_form_bounded(
    url: str,
    params: dict[str, str],
    timeout: float,
    max_body_bytes: int,
) -> tuple[int, bytes]:
    """SSRF-hardened form POST: suppress redirects, bound the timeout, and read
    the body under a genuine streaming cap that aborts once ``max_body_bytes`` is
    exceeded (never a post-hoc check on an already-buffered body). Mirrors
    :func:`basecamp.oauth.discovery._fetch_discovery_document`.

    Transport failures propagate as :class:`httpx.HTTPError` (incl.
    :class:`httpx.TimeoutException`) so callers classify them; an oversized body
    raises :class:`OAuthError` (``api_error``).
    """
    timeout = _normalize_timeout(timeout, _DEVICE_TIMEOUT)
    client = httpx.Client(timeout=timeout, follow_redirects=False)

    result: list[tuple[int, bytes]] = []
    error: list[Exception] = []

    def _worker() -> None:
        try:
            with client.stream("POST", url, data=params, headers=_FORM_HEADERS) as response:
                chunks: list[bytes] = []
                total = 0
                for chunk in response.iter_bytes():
                    total += len(chunk)
                    if total > max_body_bytes:
                        # An oversized body is api_error, not a timeout — abort the
                        # stream so it is never fully buffered.
                        raise OAuthError("api_error", "Device flow response exceeds size cap")
                    chunks.append(chunk)
                result.append((response.status_code, b"".join(chunks)))
        except Exception as exc:  # re-raised on the caller thread below
            error.append(exc)

    # httpx's timeout is per-read — it resets on every received chunk — so it does
    # NOT bound the WHOLE exchange against a peer that slow-drips header or body
    # bytes just under that interval, and httpx has no total-request timeout
    # (https://www.python-httpx.org/advanced/timeouts/). Closing the client from a
    # watchdog does not interrupt a blocked read either. So run the request on a
    # daemon worker and enforce a HARD wall-clock bound on the caller with
    # join(timeout): a stalled/slow-drip request returns within `timeout` as a read
    # timeout. The abandoned daemon worker never blocks interpreter exit and dies on
    # its own per-read timeout; client.close() below is a best-effort unblock.
    worker = threading.Thread(target=_worker, daemon=True)
    worker.start()
    worker.join(timeout)
    try:
        if worker.is_alive():
            raise httpx.ReadTimeout("Device flow request exceeded the timeout deadline")
        if error:
            raise error[0]
        return result[0]
    finally:
        client.close()


def request_device_authorization(
    device_authorization_endpoint: str,
    client_id: str,
    scope: str | None = None,
    *,
    timeout: float = _DEVICE_TIMEOUT,
    max_body_bytes: int = MAX_DEVICE_BODY_BYTES,
) -> DeviceAuthorization:
    """Request a device/user code pair (RFC 8628 §3.1–3.2).

    POSTs ``client_id`` (and ``scope`` only when set — an omitted scope lets the
    server apply its default, ``read``) to the TLS-guarded endpoint, then
    validates the response: ``device_code``, ``user_code``, ``verification_uri``
    non-empty; ``expires_in``/``interval`` positive whole seconds no greater
    than :data:`MAX_DEVICE_SECONDS` (interval defaults to 5 when absent).

    Raises :class:`DeviceFlowError` (``transport``) on a network failure and
    :class:`OAuthError` (``api_error``) on a non-2xx status or invalid metadata.
    """
    if not is_localhost(device_authorization_endpoint):
        require_https(device_authorization_endpoint, "device authorization endpoint")
    if not client_id:
        raise OAuthError("validation", "Client ID is required for device authorization")

    params: dict[str, str] = {"client_id": client_id}
    # Omit scope entirely when unset so the server applies its default (`read`).
    if scope:
        params["scope"] = scope

    try:
        status, body = _post_form_bounded(device_authorization_endpoint, params, timeout, max_body_bytes)
    except httpx.HTTPError as exc:
        raise DeviceFlowError("transport", f"Device authorization request failed: {exc}") from exc

    # Check status BEFORE parsing (as discovery does): a non-2xx here is a hard
    # failure with no OAuth error semantics, so a non-JSON error body must surface
    # as "failed with status …", not a misleading parse error. (The token poll is
    # different — it MUST parse non-2xx bodies to read authorization_pending etc.)
    if not 200 <= status < 300:
        raise OAuthError(
            "api_error",
            f"Device authorization failed with status {status}",
            http_status=status,
        )

    try:
        data = json.loads(body)
    except ValueError as exc:
        raise OAuthError(
            "api_error",
            "Failed to parse device authorization response",
            http_status=status,
        ) from exc

    if not isinstance(data, dict):
        raise OAuthError("api_error", "Device authorization response is not a JSON object", http_status=status)

    return _validate_device_authorization(data, status)


def _validate_device_authorization(data: dict[str, Any], status: int) -> DeviceAuthorization:
    # Every validation error carries the (2xx) status so a malformed success body
    # is diagnosable as such — uniform with the token-poll raises and the other SDKs.
    # Validate TYPES, not just presence: a non-string (list/number/null) is
    # malformed, not merely absent. A bare truthiness check let those through.
    for field in ("device_code", "user_code", "verification_uri"):
        value = data.get(field)
        if not isinstance(value, str) or not value:
            raise OAuthError(
                "api_error", "Invalid device authorization response: missing required fields", http_status=status
            )

    complete = data.get("verification_uri_complete")
    if complete is not None and not isinstance(complete, str):
        raise OAuthError(
            "api_error",
            "Invalid device authorization response: verification_uri_complete must be a string",
            http_status=status,
        )

    expires_in = _positive_int_seconds(data.get("expires_in"))
    if expires_in is None:
        raise OAuthError(
            "api_error",
            "Invalid device authorization response: expires_in must be a "
            f"positive integer no greater than {MAX_DEVICE_SECONDS}",
            http_status=status,
        )

    interval = DEFAULT_INTERVAL_SECONDS
    if data.get("interval") is not None:
        parsed_interval = _positive_int_seconds(data["interval"])
        if parsed_interval is None:
            raise OAuthError(
                "api_error",
                "Invalid device authorization response: interval must be a "
                f"positive integer no greater than {MAX_DEVICE_SECONDS}",
                http_status=status,
            )
        interval = parsed_interval

    return DeviceAuthorization(
        device_code=data["device_code"],
        user_code=data["user_code"],
        verification_uri=data["verification_uri"],
        verification_uri_complete=data.get("verification_uri_complete"),
        expires_in=expires_in,
        interval=interval,
    )


def _positive_int_seconds(value: Any) -> int | None:
    """Coerce an RFC 8628 duration field (``expires_in``/``interval``) to a
    positive whole-second ``int``, or ``None`` when it is not a valid one.

    Accepts an ``int`` or an integer-valued ``float`` (e.g. ``5.0``). Rejects
    fractional values (``5.9`` truncates to an interval that violates the
    validated contract; ``0.5`` would yield a ``0``-second expiry), non-positive
    values, values beyond :data:`MAX_DEVICE_SECONDS` (1e100 is not a schedulable
    deadline), and ``bool`` (which is an ``int`` subclass but never a duration).
    """
    if isinstance(value, bool):
        return None
    if isinstance(value, int):
        return value if 0 < value <= MAX_DEVICE_SECONDS else None
    if isinstance(value, float) and value.is_integer() and 0 < value <= MAX_DEVICE_SECONDS:
        return int(value)
    return None


@dataclass
class _PollResult:
    """One token-endpoint round-trip: a token on success, else an error code."""

    token: OAuthToken | None = None
    error: str | None = None
    status: int = 0


def poll_device_token(
    token_endpoint: str,
    client_id: str,
    device_code: str,
    interval: int,
    expires_in: float,
    *,
    clock: Callable[[], float] = time.monotonic,
    sleep: Callable[[float], Any] = time.sleep,
    should_cancel: Callable[[], bool] | None = None,
    timeout: float = _DEVICE_TIMEOUT,
    max_body_bytes: int = MAX_DEVICE_BODY_BYTES,
) -> OAuthToken:
    """Poll the token endpoint until approval, denial, or expiry (RFC 8628 §3.4–3.5).

    Waits at least ``interval`` seconds between polls, enforces a monotonic
    ``expires_in`` deadline against the injected ``clock``, sustains ``slow_down``
    (+5s), backs off exponentially on connection timeouts, and cooperatively
    cancels. ``should_cancel`` is a callable polled around each wait (a
    ``threading.Event.is_set`` fits directly).

    Raises :class:`DeviceFlowError` with a reason of ``access_denied``,
    ``expired``, ``transport``, or ``cancelled``.
    """
    if not is_localhost(token_endpoint):
        require_https(token_endpoint, "token endpoint")

    # The server-driven interval (initial value + sustained slow_down bumps) is
    # tracked SEPARATELY from the transient timeout backoff: each wait is
    # max(interval, backoff), and any completed round-trip snaps the backoff
    # back to the current server interval, so an inflated backoff never sticks.
    interval_seconds = interval if interval > 0 else DEFAULT_INTERVAL_SECONDS
    backoff_seconds = interval_seconds
    deadline = clock() + expires_in

    params = {
        "grant_type": DEVICE_CODE_GRANT_TYPE,
        "device_code": device_code,
        "client_id": client_id,
    }

    while True:
        if should_cancel is not None and should_cancel():
            raise DeviceFlowError("cancelled", "Device flow cancelled")

        # Bound the wait by the time left to the deadline so a grown backoff
        # interval can never sleep past expiry — a stalled request or a long
        # backoff must not blow through the monotonic deadline.
        remaining = deadline - clock()
        if remaining <= 0:
            raise DeviceFlowError("expired", "Device code expired before authorization completed")
        sleep(min(max(interval_seconds, backoff_seconds), remaining))

        if should_cancel is not None and should_cancel():
            raise DeviceFlowError("cancelled", "Device flow cancelled")

        if clock() >= deadline:
            raise DeviceFlowError("expired", "Device code expired before authorization completed")

        try:
            result = _post_device_token(token_endpoint, params, timeout, max_body_bytes)
        except httpx.TimeoutException:
            # A connection timeout → back off exponentially and keep polling.
            backoff_seconds = min(backoff_seconds * 2, MAX_BACKOFF_SECONDS)
            continue
        except httpx.HTTPError as exc:
            raise DeviceFlowError("transport", f"Device token poll failed: {exc}") from exc

        # ANY completed HTTP round-trip — a token, authorization_pending,
        # slow_down, or another OAuth error — resets the transient timeout
        # backoff to the current server interval.
        backoff_seconds = interval_seconds

        if result.token is not None:
            return result.token

        error = result.error
        if error == "authorization_pending":
            continue
        if error == "slow_down":
            interval_seconds += SLOW_DOWN_INCREMENT_SECONDS
            continue
        if error == "access_denied":
            raise DeviceFlowError("access_denied", "The authorization request was denied")
        if error == "expired_token":
            raise DeviceFlowError("expired", "Device code expired before authorization completed")
        raise OAuthError(
            "api_error",
            f"Device token request failed: {error}",
            http_status=result.status,
        )


def _valid_token_expires_in(value: Any) -> bool:
    """A token ``expires_in`` is valid when it is a finite, positive, WHOLE number
    of seconds no greater than :data:`MAX_TOKEN_LIFETIME_SECONDS`.

    An integer-valued float (``3600.0``) is accepted; a fractional value (``1.5``)
    is rejected — matching the device-duration rule; every SDK validates the
    decoded numeric value explicitly to reject a fractional token lifetime.
    ``bool`` is an ``int`` subclass but never a lifetime.
    """
    if isinstance(value, bool):
        return False
    if isinstance(value, int):
        return 0 < value <= MAX_TOKEN_LIFETIME_SECONDS
    if isinstance(value, float):
        return math.isfinite(value) and value.is_integer() and 0 < value <= MAX_TOKEN_LIFETIME_SECONDS
    return False


def _build_token(data: dict[str, Any], status: int) -> OAuthToken:
    """Construct an :class:`OAuthToken` from a validated token response.

    Every optional field is type-checked BEFORE construction: ``OAuthToken``
    computes ``expires_at`` arithmetic from ``expires_in``, so a malformed value
    (a string, a bool, a non-finite ``inf`` from ``1e400``, a fractional value, a
    value past the :data:`MAX_TOKEN_LIFETIME_SECONDS` ceiling) must surface as
    ``api_error``, never a ``TypeError`` or an ``inf`` deadline.
    ``token_type``/``refresh_token``/``scope`` must be strings when present.
    Absent/null ``expires_in`` stays allowed — the token then carries no expiry.
    """
    access_token = data.get("access_token")
    if not isinstance(access_token, str) or not access_token:
        raise OAuthError("api_error", "Device token response missing access_token", http_status=status)

    expires_in = data.get("expires_in")
    if expires_in is not None:
        if not _valid_token_expires_in(expires_in):
            raise OAuthError(
                "api_error",
                "Device token response expires_in must be a finite positive whole number "
                f"no greater than {MAX_TOKEN_LIFETIME_SECONDS} seconds",
                http_status=status,
            )
        # Coerce an integer-valued float (3600.0) to int: OAuthToken declares
        # ``expires_in: int | None`` and computes expiry arithmetic from it.
        expires_in = int(expires_in)

    # JSON null is treated as absent (the Go/Kotlin decoders cannot distinguish
    # them) → Bearer default; only an explicit non-string or empty "" is
    # malformed. Uniform across all five SDKs.
    token_type = data.get("token_type")
    if token_type is None:
        token_type = "Bearer"
    elif not isinstance(token_type, str) or not token_type:
        raise OAuthError("api_error", "Device token response token_type must be a non-empty string", http_status=status)

    refresh_token = data.get("refresh_token")
    if refresh_token is not None and not isinstance(refresh_token, str):
        raise OAuthError("api_error", "Device token response refresh_token must be a string", http_status=status)

    scope = data.get("scope")
    if scope is not None and not isinstance(scope, str):
        raise OAuthError("api_error", "Device token response scope must be a string", http_status=status)

    return OAuthToken(
        access_token=access_token,
        token_type=token_type,
        refresh_token=refresh_token,
        expires_in=expires_in,
        scope=scope,
    )


def _post_device_token(
    token_endpoint: str,
    params: dict[str, str],
    timeout: float,
    max_body_bytes: int,
) -> _PollResult:
    """One token-endpoint POST. Transport errors propagate to the caller.

    The body is read under a bounded streaming cap with redirects suppressed
    (see :func:`_post_form_bounded`), so an oversized or redirecting response
    aborts instead of buffering.
    """
    status, body = _post_form_bounded(token_endpoint, params, timeout, max_body_bytes)

    # A redirect is never a token-endpoint outcome. Classify it before parsing
    # so a 3xx body carrying {"error": "authorization_pending"} cannot keep the
    # poll loop alive — redirects are suppressed, not interpreted.
    if 300 <= status < 400:
        raise OAuthError(
            "api_error",
            f"Device token request failed with redirect status {status}",
            http_status=status,
        )

    try:
        data = json.loads(body)
    except ValueError as exc:
        raise OAuthError(
            "api_error",
            "Failed to parse device token response",
            http_status=status,
        ) from exc
    if not isinstance(data, dict):
        raise OAuthError("api_error", "Device token response is not a JSON object", http_status=status)

    if 200 <= status < 300:
        return _PollResult(token=_build_token(data, status))

    error = data.get("error") or f"http_{status}"
    return _PollResult(error=error, status=status)


def perform_device_login(
    config: OAuthConfig,
    client_id: str,
    scope: str | None = None,
    *,
    display: Callable[[DeviceAuthorization], Any],
    clock: Callable[[], float] = time.monotonic,
    sleep: Callable[[float], Any] = time.sleep,
    should_cancel: Callable[[], bool] | None = None,
    timeout: float = _DEVICE_TIMEOUT,
    max_body_bytes: int = MAX_DEVICE_BODY_BYTES,
) -> OAuthToken:
    """Run the full device authorization grant against an ALREADY-SELECTED config.

    Capability guard: requires BOTH ``config.device_authorization_endpoint`` AND
    ``device_code`` in ``config.grant_types_supported`` — otherwise raises
    :class:`DeviceFlowError` (``unavailable``) before any request. Then requests
    a device code, surfaces it through ``display``, and polls for the token.
    """
    # Require a real list before the membership test: a malformed config
    # carrying the URN as a plain str would substring-match `in` and pass the
    # guard. A non-list grant_types_supported fails the capability check.
    grant_types = config.grant_types_supported
    supports_device_grant = isinstance(grant_types, list) and DEVICE_CODE_GRANT_TYPE in grant_types
    if not config.device_authorization_endpoint or not supports_device_grant:
        raise DeviceFlowError(
            "unavailable",
            "The selected authorization server does not support the device authorization grant",
        )

    auth = request_device_authorization(
        config.device_authorization_endpoint,
        client_id,
        scope,
        timeout=timeout,
        max_body_bytes=max_body_bytes,
    )

    # The code's lifetime starts at issuance, not after display: a slow display
    # hook must eat into the deadline, never reset it. Measure the elapsed time
    # across the hook against the monotonic clock and check expiry before polling.
    issued_at = clock()
    display(auth)
    remaining = auth.expires_in - (clock() - issued_at)
    if remaining <= 0:
        raise DeviceFlowError("expired", "Device code expired before authorization completed")

    return poll_device_token(
        config.token_endpoint,
        client_id,
        auth.device_code,
        auth.interval,
        remaining,
        clock=clock,
        sleep=sleep,
        should_cancel=should_cancel,
        timeout=timeout,
        max_body_bytes=max_body_bytes,
    )
