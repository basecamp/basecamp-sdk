"""Bounded HTTP transport shared by the OAuth discovery and device-flow fetches.

Sync httpx has NO total-request timeout: its timeout is per-read and RESETS on
every received chunk, so a peer dripping header or body bytes just under that
interval can hold a request open indefinitely (verified), and closing a sync
client from a watchdog thread does not interrupt a blocked read either. The
core here runs an async client under ``asyncio.wait_for`` on a dedicated
worker thread, so the deadline CANCELS the request (and closes the socket) —
the caller is bounded regardless of chunk cadence AND the work is actually
terminated. (One documented residual below: a getaddrinfo blocked in the OS
resolver runs on an executor thread cancellation cannot interrupt, so a
slow-DNS attempt's daemon worker outlives the deadline until the resolver's
own OS-bounded timeout.)

Timing contract: a bounded request returns within ``timeout`` plus a fixed
cleanup grace of at most 1 second (``_WORKER_JOIN_GRACE``), spent only when
the request already missed its deadline and its cancellation is unwinding.
Callers folding these timeouts into their own budgets should budget
``timeout + 1``.
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

#: Cap on concurrently OUTSTANDING transport workers. A worker abandoned on a
#: stuck resolver (getaddrinfo is uninterruptible; asyncio.run then blocks in
#: shutdown_default_executor) parks itself AND an executor thread — and
#: concurrent.futures joins executor threads at interpreter shutdown, so
#: unbounded accumulation against a black-holed resolver could pile up
#: threads and delay process exit. Each worker releases its own slot when it
#: actually finishes; an abandoned worker keeps its slot held until the
#: resolver unsticks, so at most this many can ever be outstanding.
_WORKER_SLOTS = threading.BoundedSemaphore(8)

#: Upper bound (seconds) on a bounded request timeout. A per-request timeout
#: beyond this is nonsensical, and a huge finite value would overflow the
#: wall-clock wait primitive (asyncio.wait_for / thread join); callers clamp
#: to their operation default above it (see ``_normalize_timeout``).
MAX_REQUEST_TIMEOUT = 3600.0


def request_bounded(
    method: str,
    url: str,
    *,
    headers: dict[str, str],
    params: dict[str, str] | None = None,
    timeout: float,
    max_body_bytes: int,
    read_body: Callable[[int], bool] = lambda _status: True,
    on_headers: Callable[[httpx.Headers], None] | None = None,
    context: str = "OAuth",
) -> tuple[int, bytes]:
    """SSRF-hardened request: suppress redirects, bound the WHOLE round-trip by
    ``timeout``, and read the body under a genuine streaming cap that aborts once
    ``max_body_bytes`` is exceeded (never a post-hoc check on an already-buffered
    body). ``params``, when given, is sent as a form body (POST).

    ``timeout`` must arrive ALREADY normalized — finite, positive, and no greater
    than :data:`MAX_REQUEST_TIMEOUT` (callers run it through
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
    raises :class:`OAuthError` (``api_error``) carrying the response's status.
    ``context`` labels both messages.
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
        or timeout > MAX_REQUEST_TIMEOUT
        or not math.isfinite(timeout)
    ):
        raise ValueError(
            "request_bounded: timeout must be a finite positive number of seconds "
            f"no greater than {MAX_REQUEST_TIMEOUT}"
        )

    # The body cap gets the same fail-fast discipline: a bool, float (inf
    # included), or negative value would disable or crash the streaming bound
    # this core exists to provide. Same predicate as ``_normalize_body_cap``
    # so the transport can never reject a cap the public entry points accept —
    # zero is a legitimate strict cap (any non-empty body trips it).
    if isinstance(max_body_bytes, bool) or not isinstance(max_body_bytes, int) or max_body_bytes < 0:
        raise ValueError("request_bounded: max_body_bytes must be a non-negative int")

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

    async def _do() -> tuple[int, bytes]:
        try:
            return await _do_inner()
        except httpx.HTTPError as exc:
            # Backstop stamp for faults that escape before the interior
            # try (client construction above all); the interior stamp — taken
            # before any context manager unwinds — wins when present.
            if not wire_fault and not isinstance(exc, httpx.TimeoutException):
                wire_fault.append((exc, time.monotonic()))
            raise

    async def _do_inner() -> tuple[int, bytes]:
        async with httpx.AsyncClient(timeout=timeout, follow_redirects=False) as client:
            try:
                return await _request(client)
            except httpx.HTTPError as exc:
                # Stamp non-timeout wire faults at the exception SITE, before
                # the client's cleanup unwinds: __aexit__ can cross the
                # deadline (misdating an in-deadline fault as post-deadline)
                # or be cancelled by wait_for (replacing the fault with
                # TimeoutError entirely) — either way the terminal transport
                # failure must survive, mirroring completed-outcome
                # preservation.
                if not wire_fault and not isinstance(exc, httpx.TimeoutException):
                    wire_fault.append((exc, time.monotonic()))
                raise

    async def _request(client: httpx.AsyncClient) -> tuple[int, bytes]:

        async with client.stream(method, url, data=params, headers=request_headers) as response:
            try:
                return await _read(response)
            except httpx.HTTPError as exc:
                # Stamp INSIDE the response context: response.aclose() runs
                # before any outer handler sees a body-phase fault, and a
                # cleanup that crosses the deadline (or is cancelled by
                # wait_for) would otherwise erase an in-deadline wire fault.
                if not wire_fault and not isinstance(exc, httpx.TimeoutException):
                    wire_fault.append((exc, time.monotonic()))
                raise

    async def _read(response: httpx.Response) -> tuple[int, bytes]:
        # Response headers are available once the stream opens; the poll
        # loop uses this to read Retry-After without widening the return.
        if on_headers is not None:
            on_headers(response.headers)
        if not read_body(response.status_code):
            # Deadline first, symmetric with the body paths: headers
            # becoming runnable past the total bound mean the status was
            # NOT known before the deadline — classify as the timeout it
            # is rather than racing ahead of wait_for's already-due
            # callback into a status fault.
            if time.monotonic() > deadline_ts:
                raise TimeoutError(f"{context} headers arrived past the total deadline")
            # Record the KNOWN, in-deadline outcome before the context
            # managers unwind: the awaited response/client cleanup can
            # cross the total deadline and be cancelled by wait_for, and a
            # skipped non-2xx must classify by status (api_error), never
            # soften into a retryable timeout because only its CLEANUP was
            # late.
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
                # The status is known here — the cap only trips while reading
                # a body read_body admitted by status — so carry it, as the
                # buffered pre-transport path did (callers diagnose an
                # oversized 200 differently from an oversized 502).
                exc = OAuthError(
                    "api_error",
                    f"{context} response exceeds size cap",
                    http_status=response.status_code,
                )
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
    error: list[tuple[Exception, float]] = []
    wire_fault: list[tuple[Exception, float]] = []
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
            _run_bounded()
        finally:
            # The worker itself releases the slot: an abandoned (still-stuck)
            # worker keeps its slot held, which is exactly what bounds the
            # pile-up.
            _WORKER_SLOTS.release()

    def _run_bounded() -> None:
        try:
            # Budget from the CALLER'S deadline, not a fresh full timeout: a
            # late-scheduled worker (thread/CPU pressure) must not start a
            # whole new window after the advertised bound.
            remaining = deadline_ts - time.monotonic()
            if remaining <= 0:
                # Past the deadline before the request even started: refuse to
                # open a connection at all (a device-token POST after code
                # expiry above all).
                raise TimeoutError(f"{context} deadline expired before the request started")
            result.append(asyncio.run(asyncio.wait_for(_do(), remaining)))
        except Exception as exc:  # captured and re-raised on the caller thread
            # Stamp the capture instant: the caller must classify a wire
            # fault by WHEN it happened (in or past the deadline), and its
            # own clock read after the join would misdate an in-deadline
            # fault whose join crossed the bound.
            error.append((exc, time.monotonic()))

    # Acquire a worker slot inside the remaining budget: if every slot is
    # held by workers stuck on uninterruptible resolver calls, fail as the
    # timeout this request would have become anyway — without adding another
    # stuck thread to the pile.
    if not _WORKER_SLOTS.acquire(timeout=max(0.001, deadline_ts - time.monotonic())):
        raise httpx.ConnectTimeout(f"{context} transport worker slots exhausted")
    worker = threading.Thread(target=_runner, daemon=True)
    try:
        worker.start()
    except BaseException:
        _WORKER_SLOTS.release()
        raise
    # asyncio.wait_for cancels the request at `timeout`, so the worker normally
    # finishes well within it. Join with a small grace for the cancellation/cleanup
    # to unwind; if even that stalls (an uninterruptible resolver call above
    # all), return a timeout rather than block the caller. The abandoned
    # worker holds its _WORKER_SLOTS slot until it unsticks, bounding how many
    # can ever pile up (executor threads are joined at interpreter shutdown,
    # so the pile-up — not this single worker — is what could delay exit).
    # Join with the REMAINING deadline budget (not the full timeout): a
    # late-started worker already consumed part of the window, and the caller's
    # total bound must hold regardless of scheduling pressure.
    worker.join(max(0.001, deadline_ts - time.monotonic()) + _WORKER_JOIN_GRACE)
    if worker.is_alive():
        raise httpx.ReadTimeout(f"{context} request exceeded the timeout deadline")
    if error:
        exc, captured_at = error[0]
        # Recorded outcomes dominate WHATEVER the exception is — a wait_for
        # cancellation of late cleanup, or an httpx error raised BY the
        # response/client __aexit__ while closing an unconsumed stream. A
        # known non-2xx skip or completed body must not soften into a
        # retryable network error, and a documented terminal fault (the size
        # cap) must not become a transport failure, because only CLEANUP
        # misbehaved after the fact.
        if error_outcome:
            raise error_outcome[0]
        if outcome:
            return outcome[0]
        # On Python >= 3.11 (this package's floor) asyncio.TimeoutError IS the
        # builtin TimeoutError, so this catches wait_for's deadline expiry.
        if isinstance(exc, TimeoutError):
            if wire_fault and wire_fault[0][1] <= deadline_ts:
                # A wire fault observed IN deadline whose cleanup was
                # cancelled by wait_for: the terminal transport failure
                # dominates the cancellation that replaced it.
                raise wire_fault[0][0]
            raise httpx.ReadTimeout(f"{context} request exceeded the timeout deadline") from exc
        if isinstance(exc, httpx.HTTPError) and not isinstance(exc, httpx.TimeoutException):
            # Date the fault by its exception-SITE stamp when available — the
            # runner-level stamp lands after cleanup, which can cross the
            # deadline and misdate an in-deadline fault.
            stamped_at = wire_fault[0][1] if wire_fault else captured_at
            if stamped_at > deadline_ts:
                # A wire fault that became runnable past the total deadline is
                # the timeout it raced (wait_for's already-due cancellation),
                # not a distinct transport fault: a late connection reset must
                # feed the poll loop's transient backoff, never terminate.
                raise httpx.ReadTimeout(f"{context} failed past the total deadline") from exc
        raise exc
    if not result:
        # The worker died without a result AND without recording an exception
        # (a BaseException such as KeyboardInterrupt escaping the except
        # Exception net). Fail closed inside the documented contract rather
        # than leak an IndexError.
        raise httpx.TransportError(f"{context} request worker exited without a result")
    return result[0]
