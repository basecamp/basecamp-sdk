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
import threading
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

    if params is not None and method != "POST":
        # A form body on a non-POST would emit e.g. a GET-with-body — commonly
        # rejected server-side and hard to debug; fail fast on the misuse.
        raise ValueError("request_bounded: params (a form body) is only valid with POST")

    async def _do() -> tuple[int, bytes]:
        async with (
            httpx.AsyncClient(timeout=timeout, follow_redirects=False) as client,
            client.stream(method, url, data=params, headers=headers) as response,
        ):
            if not read_body(response.status_code):
                return response.status_code, b""
            chunks: list[bytes] = []
            total = 0
            async for chunk in response.aiter_bytes():
                total += len(chunk)
                if total > max_body_bytes:
                    # An oversized body is api_error, not a timeout — abort the
                    # stream so it is never fully buffered.
                    raise OAuthError("api_error", f"{context} response exceeds size cap")
                chunks.append(chunk)
            return response.status_code, b"".join(chunks)

    # httpx's timeout is per-read (it resets on every received chunk) and httpx has
    # NO total-request timeout, so a peer slow-dripping header or body bytes just
    # under that interval could otherwise hold the request open indefinitely
    # (verified); closing a sync client from a watchdog does not interrupt a blocked
    # read either. asyncio.wait_for CANCELS the request (and closes the socket) at
    # the deadline — the caller is bounded AND the work is actually terminated, no
    # leaked worker.
    #
    # Run it in a DEDICATED thread with its own event loop rather than calling
    # asyncio.run() here: this sync helper may be invoked from code that already has
    # a running loop (Jupyter/FastAPI/async CLI), where asyncio.run() raises
    # RuntimeError before any request is made. wait_for bounds the thread's work at
    # ~timeout, so the bounded join below normally returns almost immediately; the
    # is_alive backstop after it covers only a pathological async-cleanup hang.
    result: list[tuple[int, bytes]] = []
    error: list[Exception] = []

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
            raise httpx.ReadTimeout(f"{context} request exceeded the timeout deadline") from exc
        raise exc
    return result[0]
