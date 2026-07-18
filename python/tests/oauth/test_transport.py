"""Real-socket transport-bounding tests for the shared bounded request core.

respx serves a complete response instantly, so it can exercise neither a
header-then-stall nor a byte-drip; these tests run a real localhost TCP server
(discovery's origin validation exempts http on localhost) and drive the
discovery fetch through :func:`basecamp.oauth._transport.request_bounded`.
"""

from __future__ import annotations

import json
import socket
import threading
import time

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
