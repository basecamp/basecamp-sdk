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
import re
import time
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

import httpx

from basecamp._security import is_localhost, require_https, truncate
from basecamp.oauth._transport import MAX_REQUEST_TIMEOUT, request_bounded
from basecamp.oauth.config import OAuthConfig
from basecamp.oauth.device_authorization import DeviceAuthorization
from basecamp.oauth.discovery import _normalize_body_cap, _normalize_timeout
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

#: Granularity (seconds) for polling ``should_cancel`` while waiting between polls.
_CANCEL_POLL_INTERVAL = 0.1

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
    read_body: Callable[[int], bool] = lambda _status: True,
    on_headers: Callable[[Any], None] | None = None,
) -> tuple[int, bytes]:
    """SSRF-hardened form POST: suppress redirects, bound the timeout, and read
    the body under a genuine streaming cap that aborts once ``max_body_bytes`` is
    exceeded (never a post-hoc check on an already-buffered body). Delegates to
    :func:`basecamp.oauth._transport.request_bounded`, which also bounds the
    WHOLE round-trip (httpx's per-read timeout alone cannot).

    ``read_body(status)`` decides — from the response status, known once headers
    arrive — whether the body is drained. A caller returns ``False`` for statuses
    whose body it does not use (a non-2xx device-auth, a 3xx token redirect), so a
    slow/never-ending body cannot time out mid-read and be misclassified as a
    retryable transport failure instead of the api_error the status already is.

    Transport failures propagate as :class:`httpx.HTTPError` (incl.
    :class:`httpx.TimeoutException`) so callers classify them; an oversized body
    raises :class:`OAuthError` (``api_error``).
    """
    # Normalize BEFORE the transport core, which requires a finite, positive,
    # in-range timeout (see request_bounded).
    timeout = _normalize_timeout(timeout, _DEVICE_TIMEOUT, maximum=MAX_REQUEST_TIMEOUT)
    return request_bounded(
        "POST",
        url,
        headers=_FORM_HEADERS,
        params=params,
        timeout=timeout,
        max_body_bytes=max_body_bytes,
        read_body=read_body,
        on_headers=on_headers,
        context="Device flow",
    )


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

    # Normalize the cap at the public boundary: an invalid runtime value (None,
    # float("inf"), a negative) would disable the streaming memory bound
    # (``total > inf`` never trips) — same discipline as discovery.
    max_body_bytes = _normalize_body_cap(max_body_bytes, MAX_DEVICE_BODY_BYTES)

    params: dict[str, str] = {"client_id": client_id}
    # Omit scope entirely when unset so the server applies its default (`read`).
    if scope:
        params["scope"] = scope

    try:
        # A non-2xx device-auth response is a hard failure whose body is unused —
        # skip draining it so a slow error body can't time out and look like transport.
        status, body = _post_form_bounded(
            device_authorization_endpoint,
            params,
            timeout,
            max_body_bytes,
            read_body=lambda s: 200 <= s < 300,
        )
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
    except ValueError:
        # from None — json.JSONDecodeError retains the whole document as its
        # .doc attribute; these bodies carry device codes and access tokens.
        raise OAuthError(
            "api_error",
            "Failed to parse device authorization response",
            http_status=status,
        ) from None

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
    #: Raw Retry-After header on an OAuth-error response, consumed by the
    #: loop's 429 too_many_requests handling only.
    retry_after: str | None = None


def _parse_retry_after_seconds(header: str | None) -> int:
    """Validate a Retry-After delta for the 429 poll contract (SPEC §16): ASCII
    digits only, positive. A representable delta beyond
    :data:`MAX_DEVICE_SECONDS` (the shared 32-bit-ms timer bound) CLAMPS to
    the ceiling — the wait rule clips to the remaining code lifetime, honoring
    the throttle. Anything else — missing, an HTTP-date, signed, fractional,
    non-positive, or unrepresentable (the digit bound below) — returns 0 so
    the caller falls back to the current interval.

    NOT ``str.isdigit()``: it accepts non-ASCII digit-shaped characters
    (``"²"``, ``"٣"``) that ``int()`` rejects with ValueError, and an unbounded
    digit string would trip CPython's int-conversion length limit — both would
    escape the loop as a crash instead of a fallback. Leading zeros are
    stripped BEFORE the length guard so a padded in-range delta
    (``"00000000030"`` = 30) is honored; the 10-significant-digit ceiling
    comfortably covers MAX_DEVICE_SECONDS (7 digits) while keeping ``int()``
    total.

    Trimming is ASCII SP/HTAB only (RFC 9110 OWS) — NOT bare ``str.strip()``,
    whose Unicode whitespace (NBSP above all) would trim a malformed value
    into validity.
    """
    if header is None or not re.fullmatch(r"[0-9]+", header.strip(" \t")):
        return 0
    significant = header.strip(" \t").lstrip("0")
    if not significant or len(significant) > 10:
        # All zeros (value 0, non-positive) or too many significant digits
        # (overflows the ceiling regardless) → interval fallback.
        return 0
    value = int(significant)
    # A representable delta beyond the shared device ceiling clamps rather
    # than falling back: the wait rule clamps to the remaining code lifetime
    # anyway, so an over-ceiling throttle waits out the rest of the lifetime
    # instead of resending before the server's throttle. Only unrepresentable
    # strings (the digit bound above) are malformed -> interval fallback.
    return min(value, MAX_DEVICE_SECONDS)


def _validated_clock_sample(value: object, entry: str) -> float:
    """Validate one sample from an injected clock seam.

    Type-gated and overflow-safe like the ``deadline_at`` guard: a NaN sample
    defeats every deadline comparison (an ``authorization_pending`` endpoint
    would be polled indefinitely), and a string or huge-int sample must surface
    as the typed usage error — never a raw ``TypeError``/``OverflowError`` out
    of the deadline arithmetic.
    """
    result = 0.0
    valid = False
    if isinstance(value, (int, float)) and not isinstance(value, bool):
        try:
            result = float(value)
            valid = math.isfinite(result)
        except OverflowError:
            valid = False
    if not valid:
        raise OAuthError("usage", f"{entry} clock must return a finite number of seconds")
    return result


def _wait_cancellable(
    seconds: float,
    should_cancel: Callable[[], bool] | None,
    sleep: Callable[[float], Any],
) -> None:
    """Wait ``seconds``, observing cancellation DURING the wait.

    ``time.sleep`` is not interruptible, so a plain ``sleep(interval)`` would not
    notice a cancellation until the whole interval (possibly a grown ``slow_down``
    interval) elapses. When a ``should_cancel`` probe is supplied, poll it every
    :data:`_CANCEL_POLL_INTERVAL` and raise promptly. With no probe (the common
    case) a single ``sleep`` preserves the exact wait schedule — matching the
    ctx/AbortSignal/coroutine cancellation the Go/TS/Kotlin waits already have.
    """
    if should_cancel is None:
        sleep(seconds)
        return
    remaining = seconds
    while remaining > 0:
        if should_cancel():
            raise DeviceFlowError("cancelled", "Device flow cancelled")
        step = min(remaining, _CANCEL_POLL_INTERVAL)
        sleep(step)
        remaining -= step


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
    deadline_at: float | None = None,
) -> OAuthToken:
    """Poll the token endpoint until approval, denial, or expiry (RFC 8628 §3.4–3.5).

    Waits at least ``interval`` seconds between polls, enforces a monotonic
    ``expires_in`` deadline against the injected ``clock``, sustains ``slow_down``
    (+5s), backs off exponentially on connection timeouts, and cooperatively
    cancels. ``should_cancel`` is a callable polled around each wait (a
    ``threading.Event.is_set`` fits directly).

    Raises :class:`DeviceFlowError` with a reason of ``access_denied``,
    ``expired``, ``transport``, or ``cancelled``; :class:`OAuthError`
    (``usage``) for out-of-range caller durations; and :class:`OAuthError`
    (``api_error``) for a malformed, redirecting, oversized, or otherwise
    unexpected token response.
    """
    if not is_localhost(token_endpoint):
        require_https(token_endpoint, "token endpoint")

    # Caller-input sanity for this exported entry point (usage, not the RFC
    # response validation request_device_authorization applies): a non-finite or
    # oversized expires_in builds a deadline that NEVER passes (clock() + inf) —
    # an unbounded poll loop — and a non-positive/oversized interval is not a
    # schedulable wait. Fractional values are accepted: perform_device_login
    # legitimately passes a fractional remaining lifetime after deducting
    # display-hook time. Mirrors the Go/TS/Ruby/Kotlin caller guards.
    # Range checks run BEFORE math.isfinite: isfinite converts an int to float,
    # so an astronomically large int (10**400) would raise OverflowError out of
    # the guard instead of the usage error. Int/float comparisons never convert,
    # and NaN compares False on both, falling through to the isfinite reject.
    # The polling interval is additionally whole seconds (RFC 8628), matching
    # the response validation and the integer-typed Go/Kotlin APIs — a
    # fractional interval (0.001) would otherwise permit ~1000 polls/second;
    # the integral check runs LAST so an oversized int never reaches float().
    for name, value, whole in (("expires_in", expires_in, False), ("interval", interval, True)):
        if (
            isinstance(value, bool)
            or not isinstance(value, (int, float))
            or value <= 0
            or value > MAX_DEVICE_SECONDS
            or not math.isfinite(value)
            or (whole and float(value) != int(value))
        ):
            noun = "positive whole number" if whole else "positive number"
            raise OAuthError(
                "usage",
                f"poll_device_token: {name} must be a {noun} of seconds no greater than {MAX_DEVICE_SECONDS}",
            )

    # Normalize the cap at the public boundary: an invalid runtime value (None,
    # float("inf"), a negative) would disable the streaming memory bound
    # (``total > inf`` never trips) — same discipline as discovery.
    max_body_bytes = _normalize_body_cap(max_body_bytes, MAX_DEVICE_BODY_BYTES)
    # Normalize the timeout ONCE at the public boundary too: the per-poll
    # remaining-lifetime clamp takes min() against it, and an invalid runtime
    # value (None → TypeError from min, a negative, an oversized 1e100) must
    # resolve to the device budget BEFORE any arithmetic touches it.
    timeout = _normalize_timeout(timeout, _DEVICE_TIMEOUT, maximum=MAX_REQUEST_TIMEOUT)

    # The server-driven interval (initial value + sustained slow_down bumps) is
    # tracked SEPARATELY from the transient timeout backoff: each wait is
    # max(interval, backoff), and any completed round-trip snaps the backoff
    # back to the current server interval, so an inflated backoff never sticks.
    interval_seconds = interval
    backoff_seconds = interval_seconds

    def _sample_clock() -> float:
        # EVERY sample of the injected clock is validated: a NaN sample makes
        # the deadline (or a comparison against it) permanently false, so an
        # authorization_pending endpoint would be polled indefinitely.
        return _validated_clock_sample(clock(), "poll_device_token")

    # One-shot next-wait override from a 429 too_many_requests Retry-After
    # (SPEC §16): consumed by the next wait, never inflating the slow_down
    # interval. 0 = none.
    override_seconds = 0
    # An absolute issuance-anchored deadline (perform_device_login passes
    # issued_at + expires_in) beats re-anchoring: clock time elapsing between
    # the caller's remaining-lifetime computation and this entry — a process
    # suspension above all — must never be handed back to the code. It can
    # only SHORTEN the validated lifetime, never extend it.
    if deadline_at is not None:
        # Type-gated and overflow-safe: bool is not a timestamp, non-numerics
        # would raise TypeError out of isfinite, and a huge int overflows the
        # float conversion — every invalid shape must be the typed usage
        # error, matching the other duration guards.
        valid = isinstance(deadline_at, (int, float)) and not isinstance(deadline_at, bool)
        if valid:
            try:
                valid = math.isfinite(float(deadline_at)) and deadline_at <= _sample_clock() + expires_in
            except OverflowError:
                valid = False
        if not valid:
            raise OAuthError(
                "usage",
                "poll_device_token deadline_at must be a finite monotonic timestamp "
                "no later than expires_in seconds from now",
            )
        deadline = deadline_at
    else:
        deadline = _sample_clock() + expires_in

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
        remaining = deadline - _sample_clock()
        if remaining <= 0:
            raise DeviceFlowError("expired", "Device code expired before authorization completed")
        wait = min(max(interval_seconds, backoff_seconds, override_seconds), remaining)
        override_seconds = 0  # one-shot: consumed by this wait, then gone
        _wait_cancellable(wait, should_cancel, sleep)

        if should_cancel is not None and should_cancel():
            raise DeviceFlowError("cancelled", "Device flow cancelled")

        post_remaining = deadline - _sample_clock()
        if post_remaining <= 0:
            raise DeviceFlowError("expired", "Device code expired before authorization completed")

        try:
            # Bound the request by the REMAINING code lifetime as well as the
            # per-request timeout: near expiry, a stalled token POST must not
            # hold the flow past the monotonic deadline for the full budget.
            result = _post_device_token(token_endpoint, params, min(timeout, post_remaining), max_body_bytes)
        except httpx.TimeoutException:
            # A connection timeout → back off exponentially and keep polling.
            backoff_seconds = min(backoff_seconds * 2, MAX_BACKOFF_SECONDS)
            continue
        except httpx.HTTPError as exc:
            # Cancellation-beats-classification on the error path too: a cancel
            # that flipped while the doomed request was in flight must surface
            # as cancelled, not as the transport fault it happened to raise.
            if should_cancel is not None and should_cancel():
                raise DeviceFlowError("cancelled", "Device flow cancelled") from None
            raise DeviceFlowError("transport", f"Device token poll failed: {exc}") from exc

        # Re-check cancellation the moment the round-trip completes: the sync
        # POST cannot observe the probe while in flight (bounded only by its
        # timeout), so a cancel raised mid-request must surface here — never a
        # token returned after the caller asked to stop. Go/TS/Kotlin get this
        # in-flight via ctx/AbortSignal/coroutine cancellation.
        if should_cancel is not None and should_cancel():
            raise DeviceFlowError("cancelled", "Device flow cancelled")

        # ANY completed HTTP round-trip — a token, authorization_pending,
        # slow_down, or another OAuth error — resets the transient timeout
        # backoff to the current server interval.
        backoff_seconds = interval_seconds

        if result.token is not None:
            return result.token

        error = result.error
        if error == "authorization_pending":
            continue
        if error == "too_many_requests" and result.status == 429:
            # Retryable ONLY as the exact 429 + too_many_requests pair
            # (SPEC §16). The next wait honors a positive integral Retry-After
            # delta via a one-shot max(interval, Retry-After) override — a
            # missing/malformed header falls back to the current interval, and
            # the override decays after one wait.
            override_seconds = _parse_retry_after_seconds(result.retry_after)
            continue
        if error == "slow_down":
            interval_seconds += SLOW_DOWN_INCREMENT_SECONDS
            # Re-sync the backoff to the GROWN interval (the reset above used the
            # pre-increment value) so a later timeout doubles from the new interval,
            # not the stale one.
            backoff_seconds = interval_seconds
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
        # RFC 6750 authentication scheme name, not a credential.
        token_type = "Bearer"  # noqa: S105
    elif not isinstance(token_type, str) or not token_type:
        raise OAuthError("api_error", "Device token response token_type must be a non-empty string", http_status=status)

    refresh_token = data.get("refresh_token")
    if refresh_token is not None and not isinstance(refresh_token, str):
        raise OAuthError("api_error", "Device token response refresh_token must be a string", http_status=status)

    scope = data.get("scope")
    if scope is not None and not isinstance(scope, str):
        raise OAuthError("api_error", "Device token response scope must be a string", http_status=status)

    # resource: absent and JSON null are unset; when present it must be a
    # non-empty string (SPEC §16) — an empty binding is not a binding.
    resource = data.get("resource")
    if resource is not None and (not isinstance(resource, str) or not resource):
        raise OAuthError(
            "api_error",
            "Device token response resource must be a non-empty string when present",
            http_status=status,
        )

    return OAuthToken(
        access_token=access_token,
        token_type=token_type,
        refresh_token=refresh_token,
        expires_in=expires_in,
        scope=scope,
        resource=resource,
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
    # A 3xx token response is an api fault whose body is unused — skip draining it
    # so a slow redirect body can't time out and be retried by the poll loop until
    # expiry. A 4xx body IS read (it carries authorization_pending/slow_down).
    captured_headers: list[Any] = []
    status, body = _post_form_bounded(
        token_endpoint,
        params,
        timeout,
        max_body_bytes,
        read_body=lambda s: s == 200 or 400 <= s < 500,
        on_headers=captured_headers.append,
    )
    retry_after = captured_headers[0].get("Retry-After") if captured_headers else None

    # A redirect is never a token-endpoint outcome. Classify it before parsing
    # so a 3xx body carrying {"error": "authorization_pending"} cannot keep the
    # poll loop alive — redirects are suppressed, not interpreted.
    if 300 <= status < 400:
        raise OAuthError(
            "api_error",
            f"Device token request failed with redirect status {status}",
            http_status=status,
        )

    # Every remaining status outside 200 and 4xx is terminal WITHOUT its body
    # (only a 200 carries the token and only a 4xx the OAuth error code) — the
    # read_body predicate above already skipped the body, so a 201/500 that
    # stalls while streaming can never time out mid-read and be retried as a
    # transient failure until the code expires.
    if not (status == 200 or 400 <= status < 500):
        raise OAuthError(
            "api_error",
            f"Device token request failed with status {status}",
            http_status=status,
        )

    try:
        data = json.loads(body)
    except ValueError:
        # from None — json.JSONDecodeError retains the whole document as its
        # .doc attribute; these bodies carry device codes and access tokens.
        raise OAuthError(
            "api_error",
            "Failed to parse device token response",
            http_status=status,
        ) from None
    if not isinstance(data, dict):
        raise OAuthError("api_error", "Device token response is not a JSON object", http_status=status)

    # Exactly HTTP 200, not any 2xx: RFC 8628/6749 token responses are 200, and
    # SPEC §16 pins the contract. A nonstandard 201/202 carrying an access_token
    # must not prematurely complete polling — it falls through to the OAuth-error
    # path below and terminates as api_error (http_<status>).
    if status == 200:
        return _PollResult(token=_build_token(data, status))

    # Validate ``error`` as a non-empty string: a non-string (e.g. ``{"error": 123}``)
    # must not be treated as an OAuth error code — fall back to ``http_<status>``,
    # matching the other SDKs.
    # Recognize OAuth protocol error codes ONLY on a 4xx (RFC 8628 §3.5 error
    # responses are 400-class): a nonstandard 2xx (201/202) or a 5xx carrying a
    # crafted authorization_pending body must not keep the loop polling — only
    # a 200 can produce a token and only a 4xx a protocol state. Everything
    # else falls back to http_<status>, which the loop terminates as api_error.
    raw_error = data.get("error")
    # ``truncate`` at extraction (SPEC §9's 500-unit cap): the server controls
    # this string and an unrecognized value is interpolated into the api_error
    # message. Real protocol codes are short, so classification is unaffected.
    error = (
        truncate(raw_error) if 400 <= status < 500 and isinstance(raw_error, str) and raw_error else f"http_{status}"
    )
    # A 429 recognizes ONLY too_many_requests (the exact retryable pair): a
    # throttling endpoint whose body parrots authorization_pending/slow_down
    # must not keep the loop polling until code expiry.
    if status == 429 and error != "too_many_requests":
        error = f"http_{status}"
    return _PollResult(error=error, status=status, retry_after=retry_after)


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

    # A non-callable display is a usage error, not a late TypeError: it is
    # the only mechanism surfacing the verification code, so dereferencing it
    # AFTER the request would mint a code nobody can approve. Reject before
    # any network activity (matching Go). Capability
    # is checked FIRST so probing consumers keep the documented unavailable
    # outcome.
    if not callable(display):
        raise OAuthError("usage", "perform_device_login requires a callable display hook")

    # Honor a cancellation raised BEFORE the flow does any work: the sync
    # authorization POST cannot observe the probe in flight, so without this
    # entry check an already-cancelled flow still performs the request and
    # invokes the display hook.
    if should_cancel is not None and should_cancel():
        raise DeviceFlowError("cancelled", "Device flow cancelled")

    try:
        auth = request_device_authorization(
            config.device_authorization_endpoint,
            client_id,
            scope,
            timeout=timeout,
            max_body_bytes=max_body_bytes,
        )
    except (DeviceFlowError, OAuthError):
        # Cancellation-beats-classification on the error path too: a cancel
        # that flipped while the authorization request was in flight wins over
        # whatever fault the doomed request raised.
        if should_cancel is not None and should_cancel():
            raise DeviceFlowError("cancelled", "Device flow cancelled") from None
        raise

    # Anchor the code's lifetime at ISSUANCE — the response's arrival, per
    # SPEC §16 — before the display hook, so a slow display eats into the
    # deadline instead of resetting it. Expiry past this point is arbitrated
    # by the server (expired_token), so receipt-anchoring fails safe.
    issued_at = _validated_clock_sample(clock(), "perform_device_login")

    # Re-check after the round-trip AND after the anchor sample (itself a
    # cancellation-capable callback seam), before surfacing the code: a
    # cancel set in either window must not reach display.
    if should_cancel is not None and should_cancel():
        raise DeviceFlowError("cancelled", "Device flow cancelled")

    display(auth)
    remaining = auth.expires_in - (_validated_clock_sample(clock(), "perform_device_login") - issued_at)
    # Cancellation raised DURING the display hook OR during the clock sample
    # just above (the clock is a cancellation-capable callback seam, exactly
    # like the issuance anchor) wins over expiry: checked after the sample
    # and before the expiry branch, matching the TS orchestrator's ordering.
    if should_cancel is not None and should_cancel():
        raise DeviceFlowError("cancelled", "Device flow cancelled")
    if remaining <= 0:
        raise DeviceFlowError("expired", "Device code expired before authorization completed")

    return poll_device_token(
        config.token_endpoint,
        client_id,
        auth.device_code,
        auth.interval,
        remaining,
        # The EXACT issuance-anchored deadline: a clock advance between the
        # remaining computation above and the poller's entry must not extend
        # the code lifetime.
        deadline_at=issued_at + auth.expires_in,
        clock=clock,
        sleep=sleep,
        should_cancel=should_cancel,
        timeout=timeout,
        max_body_bytes=max_body_bytes,
    )
