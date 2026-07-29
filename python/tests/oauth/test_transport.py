"""Real-socket transport-bounding tests for the shared bounded request core.

respx serves a complete response instantly, so it can exercise neither a
header-then-stall nor a byte-drip; these tests run a real localhost TCP server
(discovery's origin validation exempts http on localhost) and drive the
discovery fetch through :func:`basecamp.oauth._transport.request_bounded`.
"""

from __future__ import annotations

import asyncio
import json
import socket
import threading
import time

import httpx
import pytest

from basecamp.oauth import OAuthError, discover_protected_resource
from basecamp.oauth._transport import _WORKER_JOIN_GRACE


def _serve_on_localhost() -> tuple[socket.socket, int]:
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.bind(("127.0.0.1", 0))
    srv.listen(1)
    return srv, srv.getsockname()[1]


def _settled_thread_count(baseline: int, deadline_s: float = 5.0) -> int:
    # The transport worker is a daemon thread that asyncio.wait_for bounds at
    # ~timeout; give its cancellation/cleanup a moment to unwind before counting.
    deadline = time.monotonic() + deadline_s
    while threading.active_count() > baseline and time.monotonic() < deadline:
        time.sleep(0.05)
    return threading.active_count()


def test_discovery_header_stall_is_bounded_and_leaks_no_worker() -> None:
    # The peer sends complete headers then stalls forever without a body byte.
    # The fetch must surface a retryable network timeout within ~timeout (plus
    # the worker-join grace), and the transport's worker thread must be gone —
    # not parked forever on the dead connection.
    srv, port = _serve_on_localhost()
    stop = threading.Event()

    def stall() -> None:
        conn, _ = srv.accept()
        conn.recv(4096)
        try:
            conn.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 100\r\nContent-Type: application/json\r\n\r\n")
            stop.wait()
        except OSError:
            pass
        finally:
            conn.close()

    baseline = threading.active_count()
    server = threading.Thread(target=stall, daemon=True)
    server.start()
    timeout = 0.5
    try:
        start = time.monotonic()
        with pytest.raises(OAuthError) as exc_info:
            discover_protected_resource(f"http://127.0.0.1:{port}", timeout=timeout)
        elapsed = time.monotonic() - start
        assert exc_info.value.code == "network"
        assert exc_info.value.retryable
        assert "timed out" in str(exc_info.value)
        assert elapsed < timeout + _WORKER_JOIN_GRACE + 1.0, f"fetch not bounded by the timeout: took {elapsed:.2f}s"
    finally:
        stop.set()
        server.join(2)
        srv.close()
    assert _settled_thread_count(baseline) == baseline, "leaked transport worker thread"


def test_unknown_method_fails_fast() -> None:
    # An unknown verb must fail fast, never reach httpx.
    from basecamp.oauth._transport import request_bounded

    with pytest.raises(ValueError, match="must be GET or POST"):
        request_bounded(
            "POTS",
            "https://issuer.example/x",
            headers={},
            params=None,
            timeout=1.0,
            max_body_bytes=1024,
        )


def test_params_with_non_post_fails_fast() -> None:
    # A form body on a GET would emit a GET-with-body; misuse fails fast instead.
    from basecamp.oauth._transport import request_bounded

    with pytest.raises(ValueError, match="only valid with POST"):
        request_bounded(
            "GET",
            "https://issuer.example/x",
            headers={},
            params={"a": "b"},
            timeout=1.0,
            max_body_bytes=1024,
        )


def test_invalid_max_body_bytes_fails_fast() -> None:
    # The cap IS the streaming bound this core exists to provide — a bool,
    # float (inf included), or non-positive value would disable or crash it,
    # so misuse rejects before any connection.
    from basecamp.oauth._transport import request_bounded

    for cap in (None, True, False, 0, -8, 1.5, float("inf")):
        with pytest.raises(ValueError, match="max_body_bytes must be a positive int"):
            request_bounded(
                "GET",
                "https://issuer.example/x",
                headers={},
                params=None,
                timeout=1.0,
                max_body_bytes=cap,  # type: ignore[arg-type]
            )


def test_discovery_non_2xx_with_stalled_body_is_immediate_api_error() -> None:
    # SPEC.md: non-2xx on either discovery hop → api_error, never network —
    # status dominates even when the error body stalls forever, so the fetch
    # classifies at header time instead of timing the body out.
    srv, port = _serve_on_localhost()
    stop = threading.Event()

    def stall() -> None:
        conn, _ = srv.accept()
        conn.recv(4096)
        try:
            conn.sendall(b"HTTP/1.1 500 Internal Server Error\r\nContent-Length: 1000\r\n\r\n")
            stop.wait()
        except OSError:
            pass
        finally:
            conn.close()

    baseline = threading.active_count()
    server = threading.Thread(target=stall, daemon=True)
    server.start()
    timeout = 0.5
    try:
        start = time.monotonic()
        with pytest.raises(OAuthError) as exc_info:
            discover_protected_resource(f"http://127.0.0.1:{port}", timeout=timeout)
        elapsed = time.monotonic() - start
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 500
        assert elapsed < timeout, f"status must classify at header time, not after a body timeout: {elapsed:.2f}s"
    finally:
        stop.set()
        server.join(2)
        srv.close()
    assert _settled_thread_count(baseline) == baseline, "leaked transport worker thread"


def test_skipped_status_survives_cleanup_crossing_the_deadline(monkeypatch) -> None:
    # A skipped non-2xx whose headers arrive just before the total deadline
    # must classify by STATUS even when the awaited response/client cleanup
    # crosses the deadline and is cancelled by wait_for — never soften into a
    # retryable timeout. Deterministic: the patched client aclose stalls past
    # the deadline after the response is already known.
    # Patch __aexit__ (NOT aclose — AsyncClient.__aexit__ closes the
    # transport directly and never routes through aclose).
    real_aexit = httpx.AsyncClient.__aexit__

    async def slow_aexit(self, *args):
        await asyncio.sleep(1.5)  # crosses the 0.5s deadline
        return await real_aexit(self, *args)

    monkeypatch.setattr(httpx.AsyncClient, "__aexit__", slow_aexit)

    srv, port = _serve_on_localhost()

    def serve() -> None:
        conn, _ = srv.accept()
        conn.recv(4096)
        try:
            conn.sendall(b"HTTP/1.1 500 Internal Server Error\r\nContent-Length: 0\r\n\r\n")
        except OSError:
            pass
        finally:
            conn.close()

    server = threading.Thread(target=serve, daemon=True)
    server.start()
    try:
        with pytest.raises(OAuthError) as exc_info:
            discover_protected_resource(f"http://127.0.0.1:{port}", timeout=0.5)
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 500
    finally:
        server.join(3)
        srv.close()


def test_size_cap_error_survives_cleanup_crossing_the_deadline(monkeypatch) -> None:
    # An oversized body trips the documented api_error size cap; when async
    # cleanup then crosses the deadline and wait_for cancels, the terminal
    # fault must survive — never soften into a retryable timeout.
    real_aexit = httpx.AsyncClient.__aexit__

    async def slow_aexit(self, *args):
        await asyncio.sleep(1.5)  # crosses the 0.5s deadline
        return await real_aexit(self, *args)

    monkeypatch.setattr(httpx.AsyncClient, "__aexit__", slow_aexit)

    srv, port = _serve_on_localhost()
    big = b"x" * (2 * 1024 * 1024)

    def serve() -> None:
        conn, _ = srv.accept()
        conn.recv(4096)
        try:
            conn.sendall(
                b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                + str(len(big)).encode()
                + b"\r\n\r\n"
                + big
            )
        except OSError:
            pass
        finally:
            conn.close()

    server = threading.Thread(target=serve, daemon=True)
    server.start()
    try:
        with pytest.raises(OAuthError) as exc_info:
            discover_protected_resource(f"http://127.0.0.1:{port}", timeout=0.5)
        assert exc_info.value.code == "api_error"
        assert "size cap" in str(exc_info.value)
    finally:
        server.join(3)
        srv.close()


def test_compressed_bodies_are_never_inflated_by_the_transport() -> None:
    # Transparent decompression would let a compression bomb balloon past the
    # byte cap BEFORE the per-chunk check ran (httpx inflates in
    # aiter_bytes). The transport requests identity and reads RAW wire bytes:
    # a server compressing anyway hands over compressed bytes bounded by the
    # cap, and classification (a JSON parse failure) happens on the small
    # payload — memory never exceeds the advertised bound.
    import gzip as gzip_mod

    compressed = gzip_mod.compress(b"x" * 10_000_000)  # ~10 MB decoded
    assert len(compressed) < 20_000, "bomb premise: tiny on the wire"

    srv, port = _serve_on_localhost()

    def serve() -> None:
        conn, _ = srv.accept()
        conn.recv(4096)
        try:
            conn.sendall(
                b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n"
                b"Content-Encoding: gzip\r\nContent-Length: " + str(len(compressed)).encode() + b"\r\n\r\n" + compressed
            )
        except OSError:
            pass
        finally:
            conn.close()

    server = threading.Thread(target=serve, daemon=True)
    server.start()
    try:
        with pytest.raises(OAuthError) as exc_info:
            discover_protected_resource(f"http://127.0.0.1:{port}", timeout=2)
        # The raw bytes flowed through under the cap and failed JSON parsing —
        # never a decoded blow-up into the size cap.
        assert exc_info.value.code == "api_error"
        assert "size cap" not in str(exc_info.value)
    finally:
        server.join(2)
        srv.close()


def test_discovery_slow_drip_is_bounded_by_the_total_timeout() -> None:
    # httpx's read timeout resets on every received chunk, so a peer dripping a
    # VALID discovery document byte-by-byte (each read under the timeout) would
    # otherwise hold the fetch open far past it — httpx has no total timeout.
    # asyncio.wait_for must cancel the whole round-trip at ~timeout regardless
    # of chunk cadence, surfacing a retryable network timeout.
    srv, port = _serve_on_localhost()
    origin = f"http://127.0.0.1:{port}"
    body = json.dumps({"resource": origin}).encode()
    payload = (
        f"HTTP/1.1 200 OK\r\nContent-Length: {len(body)}\r\nContent-Type: application/json\r\n\r\n"
    ).encode() + body

    def drip() -> None:
        conn, _ = srv.accept()
        conn.recv(4096)
        try:
            # ~100+ bytes dripped at 0.2s each ≈ 20s+ total; the 0.5s timeout must win.
            for byte in payload:
                conn.sendall(bytes([byte]))
                time.sleep(0.2)
        except OSError:
            pass
        finally:
            conn.close()

    baseline = threading.active_count()
    server = threading.Thread(target=drip, daemon=True)
    server.start()
    timeout = 0.5
    try:
        start = time.monotonic()
        with pytest.raises(OAuthError) as exc_info:
            discover_protected_resource(origin, timeout=timeout)
        elapsed = time.monotonic() - start
        assert exc_info.value.code == "network"
        assert exc_info.value.retryable
        assert "timed out" in str(exc_info.value)
        assert elapsed < timeout + _WORKER_JOIN_GRACE + 1.0, f"fetch not bounded by the timeout: took {elapsed:.2f}s"
    finally:
        srv.close()
        server.join(5)
    assert _settled_thread_count(baseline) == baseline, "leaked transport worker thread"


@pytest.mark.parametrize(
    "timeout",
    [None, "5", True, 0, -1, float("inf"), float("nan"), 3601.0, 10**400],
)
def test_invalid_timeout_fails_fast(timeout) -> None:
    # request_bounded's whole purpose is a TOTAL request bound; an unnormalized
    # timeout (inf/nan/non-positive/oversized/huge-int) would disable or
    # overflow asyncio.wait_for and the thread join. Callers normalize, but the
    # contract is enforced here too — a violation is a programming error.
    from basecamp.oauth._transport import request_bounded

    with pytest.raises(ValueError, match="timeout must be a finite positive"):
        request_bounded(
            "GET",
            "https://issuer.example/x",
            headers={},
            timeout=timeout,
            max_body_bytes=1024,
        )
