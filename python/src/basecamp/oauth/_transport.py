"""Bounded HTTP transport shared by the OAuth discovery and device-flow fetches.

Sync httpx has NO total-request timeout: its timeout is per-read and RESETS on
every received chunk, so a peer dripping header or body bytes just under that
interval can hold a request open indefinitely (verified), and closing a sync
client from a watchdog thread does not interrupt a blocked read either. The
core here runs an async client under ``asyncio.wait_for`` on a dedicated
worker thread, so the deadline CANCELS the request (and closes the socket) —
the caller is bounded regardless of chunk cadence AND the work is actually
terminated, no leaked connection.
"""

from __future__ import annotations

import asyncio
import math
import threading
import time
from collections.abc import Callable

import httpx

from basecamp.oauth.errors import OAuthError

#: Extra time (seconds) to let a timed-out request's async cancellation/cleanup
#: unwind before the caller abandons the (daemon) worker and returns a timeout.
#: Kept SMALL so the caller's worst-case block stays ~timeout: cleanup after a
#: wait_for cancellation is just closing a socket, and joining with no grace at
#: all would race a request that completes right at the deadline. The daemon
#: worker never blocks interpreter exit if even this stalls.
_WORKER_JOIN_GRACE = 1.0

#: Upper bound (seconds) on a bounded request timeout. A per-request timeout
#: beyond this is nonsensical, and a huge finite value would overflow the
#: wall-clock wait primitive (asyncio.wait_for / thread join); callers clamp
#: to their operation default above it (see ``_normalize_timeout``).
_MAX_REQUEST_TIMEOUT = 3600.0


def request_bounded(
    method: str,
    url: str,
    *,
    headers: dict[str, str],
    params: dict[str, str] | None = None,
    timeout: float,
    max_body_bytes: int,
    read_body: Callable[[int], bool] = lambda _status: True,
    context: str = "OAuth",
) -> tuple[int, bytes]:
    """SSRF-hardened request: suppress redirects, bound the WHOLE round-trip by
    ``timeout``, and read the body under a genuine streaming cap that aborts once
    ``max_body_bytes`` is exceeded (never a post-hoc check on an already-buffered
    body). ``params``, when given, is sent as a form body (POST).

    ``timeout`` must arrive ALREADY normalized — finite, positive, and no greater
    than :data:`_MAX_REQUEST_TIMEOUT` (callers run it through
    ``discovery._normalize_timeout``, which this module cannot import without a
    cycle). An unnormalized value would disable the ``wait_for`` deadline
    (``inf`` never fires) or overflow the wait primitive.

    ``read_body(status)`` decides — from the response status, known once headers
    arrive — whether the body is drained. A caller returns ``False`` for statuses
    whose body it does not use, so a slow/never-ending body cannot time out
    mid-read and be misclassified as a retryable transport failure instead of
    the api_error the status already is.

    Transport failures propagate as :class:`httpx.HTTPError` (incl.
    :class:`httpx.TimeoutException`) so callers classify them; an oversized body
    raises :class:`OAuthError` (``api_error``). ``context`` labels both messages.
    """

    # Fail fast on unknown verbs (a typo like "POTS" would otherwise go to
    # httpx as-is), matching the Ruby primitive's :get/:post contract.
    if method.upper() not in ("GET", "POST"):
        raise ValueError("request_bounded: method must be GET or POST")
    if params is not None and method.upper() != "POST":
        # A form body on a non-POST would emit e.g. a GET-with-body — commonly
        # rejected server-side and hard to debug; fail fast on the misuse.
        raise ValueError("request_bounded: params (a form body) is only valid with POST")

    # Defensive enforcement of the caller-normalization contract: an inf/nan/
    # non-positive/oversized timeout would disable or overflow the wait_for
    # deadline and the thread join — the very bound this module exists to
    # provide. Callers run through _normalize_timeout; a violation here is a
    # programming error, so fail fast. Range checks run BEFORE math.isfinite:
    # isfinite converts an int to float, so an astronomically large int
    # (10**400) would raise OverflowError out of the guard instead of this
    # ValueError; int/float comparisons never convert, and NaN compares False
    # on both, falling through to the isfinite reject.
    if (
        isinstance(timeout, bool)
        or not isinstance(timeout, (int, float))
        or timeout <= 0
        or timeout > _MAX_REQUEST_TIMEOUT
        or not math.isfinite(timeout)
    ):
        raise ValueError(
            "request_bounded: timeout must be a finite positive number of seconds "
            f"no greater than {_MAX_REQUEST_TIMEOUT}"
        )

    async def _do() -> tuple[int, bytes]:
        # Identity encoding + aiter_raw(): httpx transparently inflates
        # gzip/deflate in aiter_bytes(), so the per-chunk cap would measure
        # DECODED bytes — a small compressed body could balloon far past
        # max_body_bytes in memory (compression bomb). Request identity and
        # read the RAW wire bytes; a server compressing anyway hands over
        # compressed bytes bounded by the cap, which then fail classification
        # upstream instead of exhausting memory.
        # Case-insensitive replacement: a caller passing "accept-encoding"
        # would otherwise coexist with our key and emit duplicate headers,
        # undermining the identity-only compression-bomb bound (the Ruby
        # transport filters the same way).
        request_headers = {k: v for k, v in headers.items() if k.lower() != "accept-encoding"}
        request_headers["Accept-Encoding"] = "identity"
        async with (
            httpx.AsyncClient(timeout=timeout, follow_redirects=False) as client,
            client.stream(method, url, data=params, headers=request_headers) as response,
        ):
            if not read_body(response.status_code):
                # Record the KNOWN outcome before the context managers unwind:
                # the awaited response/client cleanup can cross the total
                # deadline and be cancelled by wait_for, and a skipped non-2xx
                # must classify by status (api_error), never soften into a
                # retryable timeout because only its cleanup was late.
                outcome.append((response.status_code, b""))
                return response.status_code, b""
            chunks: list[bytes] = []
            total = 0
            async for chunk in response.aiter_raw():
                total += len(chunk)
                if total > max_body_bytes:
                    # Deadline first, symmetric with body completion: a chunk
                    # becoming runnable past the bound must classify as the
                    # timeout it is, not race ahead of wait_for into a
                    # terminal api_error.
                    if time.monotonic() > deadline_ts:
                        raise TimeoutError(f"{context} body exceeded the total deadline")
                    # An oversized body is api_error, not a timeout — abort the
                    # stream so it is never fully buffered. Record the terminal
                    # error BEFORE the context managers unwind: their awaited
                    # cleanup can cross the deadline and be cancelled by
                    # wait_for, which would otherwise soften this documented
                    # size-cap error into a retryable timeout.
                    exc = OAuthError("api_error", f"{context} response exceeds size cap")
                    error_outcome.append(exc)
                    raise exc
                chunks.append(chunk)
            # Same completed-outcome recording as the skip path — but ONLY
            # when the body finished inside the deadline: the task can run
            # ahead of wait_for's already-due timeout callback, and a body
            # completing past the advertised bound must not be accepted.
            # (Skipped statuses stay status-first regardless — their
            # classification is header-time knowledge.)
            if time.monotonic() > deadline_ts:
                # The body finished past the advertised bound but this
                # coroutine ran ahead of wait_for's already-due timeout
                # callback — raising (not returning) keeps the late body out
                # of `result` too, so the caller classifies it as the timeout
                # it is.
                raise TimeoutError(f"{context} body completed past the total deadline")
            joined = b"".join(chunks)
            outcome.append((response.status_code, joined))
            return response.status_code, joined

    # httpx's timeout is per-read (it resets on every received chunk) and httpx has
    # NO total-request timeout, so a peer slow-dripping header or body bytes just
    # under that interval could otherwise hold the request open indefinitely
    # (verified); closing a sync client from a watchdog does not interrupt a blocked
    # read either. asyncio.wait_for CANCELS the request (and closes the socket) at
    # the deadline — the caller is bounded AND the socket work is actually
    # terminated. Known residual: a getaddrinfo blocked in the OS resolver runs on
    # an executor thread that cancellation cannot interrupt, so a slow-DNS attempt
    # leaves a daemon worker alive until the RESOLVER's own timeout (seconds to
    # ~30s, OS-bounded) — the caller still returns at the deadline, the stragglers
    # are bounded in lifetime by the resolver and in count by the attempt rate,
    # and they never block interpreter exit. Sync Python offers no stronger
    # DNS bound short of process isolation.
    #
    # Run it in a DEDICATED thread with its own event loop rather than calling
    # asyncio.run() here: this sync helper may be invoked from code that already has
    # a running loop (Jupyter/FastAPI/async CLI), where asyncio.run() raises
    # RuntimeError before any request is made. wait_for bounds the thread's work at
    # ~timeout, so the bounded join below normally returns almost immediately; the
    # is_alive backstop after it covers only a pathological async-cleanup hang.
    result: list[tuple[int, bytes]] = []
    error: list[Exception] = []
    # Completed outcomes recorded INSIDE _do before async cleanup: when
    # wait_for cancels mid-unwind, the response is already known and must win
    # over the cancellation (status-first classification for skipped statuses;
    # a fully read body likewise survives a late cleanup — but only when it
    # completed inside the monotonic deadline below).
    deadline_ts = time.monotonic() + timeout
    outcome: list[tuple[int, bytes]] = []
    # Terminal errors recorded before async cleanup, same rationale as
    # `outcome`: a documented api fault must survive a cleanup-crossing
    # cancellation.
    error_outcome: list[Exception] = []

    def _runner() -> None:
        try:
            result.append(asyncio.run(asyncio.wait_for(_do(), timeout)))
        except Exception as exc:  # captured and re-raised on the caller thread
            error.append(exc)

    worker = threading.Thread(target=_runner, daemon=True)
    worker.start()
    # asyncio.wait_for cancels the request at `timeout`, so the worker normally
    # finishes well within it. Join with a small grace for the cancellation/cleanup
    # to unwind; if even that stalls (a pathological async cleanup hang), return a
    # timeout rather than block the caller — the daemon worker never blocks
    # interpreter exit. This is bounded AND non-leaking in every non-pathological case.
    worker.join(timeout + _WORKER_JOIN_GRACE)
    if worker.is_alive():
        raise httpx.ReadTimeout(f"{context} request exceeded the timeout deadline")
    if error:
        exc = error[0]
        # On Python >= 3.11 (this package's floor) asyncio.TimeoutError IS the
        # builtin TimeoutError, so this catches wait_for's deadline expiry.
        if isinstance(exc, TimeoutError):
            if error_outcome:
                # A documented terminal fault (the size cap) fired before the
                # deadline — only its cleanup crossed it. The fault dominates.
                raise error_outcome[0]
            if outcome:
                # The response COMPLETED before the deadline — only the async
                # cleanup crossed it and was cancelled. The known outcome
                # dominates the deadline race.
                return outcome[0]
            raise httpx.ReadTimeout(f"{context} request exceeded the timeout deadline") from exc
        raise exc
    if not result:
        # The worker died without a result AND without recording an exception
        # (a BaseException such as KeyboardInterrupt escaping the except
        # Exception net). Fail closed inside the documented contract rather
        # than leak an IndexError.
        raise httpx.TransportError(f"{context} request worker exited without a result")
    return result[0]
