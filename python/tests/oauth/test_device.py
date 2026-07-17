"""RFC 8628 device authorization grant tests (SPEC.md §16).

Timing is deterministic: the ``sleep`` seam records the interval schedule and
returns immediately, and the monotonic ``clock`` is injected. No real delays.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp.oauth import (
    DEVICE_CODE_GRANT_TYPE,
    DeviceFlowError,
    OAuthConfig,
    OAuthError,
    perform_device_login,
    poll_device_token,
    request_device_authorization,
)

ORIGIN = "https://issuer.device-test.example"
DEVICE_ENDPOINT = f"{ORIGIN}/oauth/device"
TOKEN_ENDPOINT = f"{ORIGIN}/oauth/token"

DEVICE_AUTH_RESPONSE = {
    "device_code": "dev-code-123",
    "user_code": "WDJB-MJHT",
    "verification_uri": f"{ORIGIN}/device",
    "verification_uri_complete": f"{ORIGIN}/device?user_code=WDJB-MJHT",
    "expires_in": 900,
    "interval": 5,
}

TOKEN_RESPONSE = {
    "access_token": "device_access_token",  # gitleaks:allow (test fixture)
    "refresh_token": "device_refresh_token",  # gitleaks:allow (test fixture)
    "token_type": "Bearer",
    "expires_in": 3600,
}

CONFIG = OAuthConfig(
    issuer=ORIGIN,
    authorization_endpoint=None,
    token_endpoint=TOKEN_ENDPOINT,
    device_authorization_endpoint=DEVICE_ENDPOINT,
    grant_types_supported=[DEVICE_CODE_GRANT_TYPE, "refresh_token"],
)


class RecordingSleep:
    """A sleep seam that records requested waits and returns immediately."""

    def __init__(self) -> None:
        self.waits: list[float] = []

    def __call__(self, seconds: float) -> None:
        self.waits.append(seconds)


def _queue_token_responses(responses: list[httpx.Response]) -> respx.Route:
    """Serve a fixed sequence of token-endpoint responses, one per poll."""
    return respx.post(TOKEN_ENDPOINT).mock(side_effect=responses)


class TestRequestDeviceAuthorization:
    @respx.mock
    def test_omits_scope_when_unset_and_validates(self):
        route = respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(200, json=DEVICE_AUTH_RESPONSE))

        auth = request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")

        body = route.calls[0].request.content.decode()
        assert "client_id=basecamp-cli" in body
        assert "scope" not in body  # omitted → server default (read)
        assert auth.device_code == "dev-code-123"
        assert auth.user_code == "WDJB-MJHT"
        assert auth.verification_uri_complete == f"{ORIGIN}/device?user_code=WDJB-MJHT"
        assert auth.interval == 5

    @respx.mock
    def test_sends_scope_when_set(self):
        route = respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(200, json=DEVICE_AUTH_RESPONSE))

        request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli", scope="read write")

        assert "scope=read+write" in route.calls[0].request.content.decode()

    @respx.mock
    def test_defaults_interval_to_5_when_omitted(self):
        payload = {k: v for k, v in DEVICE_AUTH_RESPONSE.items() if k != "interval"}
        respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(200, json=payload))

        auth = request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")

        assert auth.interval == 5

    @respx.mock
    def test_treats_null_interval_as_absent(self):
        # `"interval": null` is the cross-SDK contract for absent (Go/Kotlin
        # decoders cannot distinguish null from a missing key) → default 5.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "interval": None})
        )

        auth = request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")

        assert auth.interval == 5

    @respx.mock
    def test_rejects_non_positive_expires_in(self):
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "expires_in": 0})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_non_positive_interval(self):
        respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "interval": 0}))

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_fractional_expires_in(self):
        # A fractional expiry (e.g. 0.5s) truncates to a 0-second deadline and
        # must be rejected outright rather than silently floored.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "expires_in": 5.9})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_fractional_interval(self):
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "interval": 2.5})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_oversized_expires_in(self):
        # 1e100 is integer-valued, so whole-second checking alone would admit
        # it; the shared cross-SDK ceiling (2147483 s) makes it api_error.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "expires_in": 1e100})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_oversized_interval(self):
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "interval": 1e100})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_just_past_max_duration(self):
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "expires_in": 2_147_484})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_accepts_max_duration(self):
        # The 2147483 s ceiling itself is valid — the bound is inclusive.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(
                200, json={**DEVICE_AUTH_RESPONSE, "expires_in": 2_147_483, "interval": 2_147_483}
            )
        )

        auth = request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")

        assert auth.expires_in == 2_147_483
        assert auth.interval == 2_147_483

    @respx.mock
    def test_rejects_bool_expires_in(self):
        # ``bool`` is an ``int`` subclass but is never a valid duration.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "expires_in": True})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_accepts_integer_valued_float_durations(self):
        # ``5.0``/``900.0`` are integer-valued floats — accepted and coerced.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "expires_in": 900.0, "interval": 10.0})
        )

        auth = request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")

        assert auth.expires_in == 900
        assert isinstance(auth.expires_in, int)
        assert auth.interval == 10
        assert isinstance(auth.interval, int)

    @respx.mock
    def test_rejects_missing_field(self):
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={"user_code": "X", "verification_uri": ORIGIN, "expires_in": 900})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_transport_failure_raises_device_flow_error(self):
        respx.post(DEVICE_ENDPOINT).mock(side_effect=httpx.ConnectError("boom"))

        with pytest.raises(DeviceFlowError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.reason == "transport"

    def test_slow_drip_request_is_bounded_by_the_wall_clock_timeout(self):
        # httpx's read timeout resets on every received chunk, so a peer dripping a
        # VALID response byte-by-byte (each read under the timeout) would otherwise
        # hold the request open far past it — httpx has no total timeout. The
        # daemon-worker + join(timeout) bound must return WITHIN the timeout as a
        # retryable transport failure, not run for the whole drip. A real socket is
        # used because respx serves a complete response instantly (no drip).
        import socket
        import threading
        import time as _time

        srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        srv.bind(("127.0.0.1", 0))
        srv.listen(1)
        port = srv.getsockname()[1]
        # ~60 bytes dripped at 0.2s each ≈ 12s total; the 0.5s timeout must win.
        valid = b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nContent-Type: application/json\r\n\r\n{}"

        def drip() -> None:
            conn, _ = srv.accept()
            conn.recv(4096)
            try:
                for byte in valid:
                    conn.sendall(bytes([byte]))
                    _time.sleep(0.2)
            except OSError:
                pass

        threading.Thread(target=drip, daemon=True).start()
        try:
            start = _time.monotonic()
            with pytest.raises(DeviceFlowError) as exc_info:
                request_device_authorization(f"http://127.0.0.1:{port}/device", "basecamp-cli", timeout=0.5)
            elapsed = _time.monotonic() - start
            assert exc_info.value.reason == "transport"
            assert exc_info.value.retryable
            assert elapsed < 4, f"request not bounded by the timeout: took {elapsed:.2f}s"
        finally:
            srv.close()

    @respx.mock
    def test_rejects_non_string_field_types(self):
        # A non-string field is malformed, not merely absent — a truthiness
        # check would let 12345 / a list slip through.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "device_code": 12345})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_non_string_verification_uri_complete(self):
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "verification_uri_complete": ["nope"]})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_non_2xx_with_non_json_body_reports_status_not_parse_error(self):
        # Status is checked BEFORE parsing (as discovery does): a non-2xx with a
        # non-JSON body must surface as "failed with status", not a parse error.
        respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(503, content=b"<html>Service Unavailable</html>"))

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 503
        assert "status" in str(exc_info.value).lower()

    @respx.mock
    def test_rejects_non_object_body_with_http_status(self):
        # A valid-JSON-but-non-object body (list/number/null) is malformed. It must
        # fail as api_error AND carry the HTTP status for debugging parity with the
        # other error raises — not drop it.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, content="[]", headers={"Content-Type": "application/json"})
        )

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 200

    @respx.mock
    def test_oversized_body_aborts(self):
        # A body past the streaming cap must abort with api_error, never buffer.
        payload = {**DEVICE_AUTH_RESPONSE, "pad": "x" * (256 * 1024)}
        respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(200, json=payload))

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli", max_body_bytes=8 * 1024)
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_does_not_follow_redirect(self):
        redirect_target = "https://evil.device-test.example/steal"
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(302, headers={"Location": redirect_target}, json={"error": "moved"})
        )
        target = respx.get(redirect_target).mock(return_value=httpx.Response(200, json=DEVICE_AUTH_RESPONSE))

        with pytest.raises(OAuthError) as exc_info:
            request_device_authorization(DEVICE_ENDPOINT, "basecamp-cli")
        assert exc_info.value.code == "api_error"
        assert not target.called  # redirect suppressed — attacker origin never dialed


class TestPollDeviceToken:
    @respx.mock
    def test_pending_then_slow_down_then_token_sustains_interval(self):
        _queue_token_responses(
            [
                httpx.Response(400, json={"error": "authorization_pending"}),
                httpx.Response(400, json={"error": "slow_down"}),
                httpx.Response(400, json={"error": "authorization_pending"}),
                httpx.Response(200, json=TOKEN_RESPONSE),
            ]
        )
        sleep = RecordingSleep()

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=sleep
        )

        assert token.access_token == "device_access_token"  # gitleaks:allow
        # 5s (pending), 5s (before slow_down), then +5 sustained → 10s, 10s.
        assert sleep.waits == [5, 5, 10, 10]

    @respx.mock
    def test_doubles_interval_after_connection_timeout_then_recovers(self):
        _queue_token_responses(
            [
                httpx.TimeoutException("timed out"),
                httpx.Response(200, json=TOKEN_RESPONSE),
            ]
        )
        sleep = RecordingSleep()

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=sleep
        )

        assert token.access_token == "device_access_token"  # gitleaks:allow
        # First wait 5s, timeout → backoff doubles to 10s for the next wait.
        assert sleep.waits[0] == 5
        assert sleep.waits[1] == 10

    @respx.mock
    def test_backoff_tracks_grown_interval_after_slow_down(self):
        # slow_down grows the interval 5→10; a following timeout must double from
        # the GROWN interval (10→20), not the stale pre-slow_down 5 (which gave 10).
        _queue_token_responses(
            [
                httpx.Response(400, json={"error": "slow_down"}),
                httpx.TimeoutException("timed out"),
                httpx.Response(200, json=TOKEN_RESPONSE),
            ]
        )
        sleep = RecordingSleep()

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=sleep
        )

        assert token.access_token == "device_access_token"  # gitleaks:allow
        assert sleep.waits == [5, 10, 20]

    @respx.mock
    def test_backoff_resets_to_server_interval_after_completed_round_trip(self):
        # Timeout backoff is transient: it doubles per timeout, but ANY completed
        # round-trip (here authorization_pending) snaps the wait back to the
        # server interval — intermittent timeouts must not inflate later polls.
        _queue_token_responses(
            [
                httpx.TimeoutException("timed out"),
                httpx.TimeoutException("timed out"),
                httpx.Response(400, json={"error": "authorization_pending"}),
                httpx.Response(400, json={"error": "authorization_pending"}),
                httpx.Response(200, json=TOKEN_RESPONSE),
            ]
        )
        sleep = RecordingSleep()

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=sleep
        )

        assert token.access_token == "device_access_token"  # gitleaks:allow
        # 5s, timeout → 10s, timeout → 20s, pending → back to 5s, pending → 5s.
        assert sleep.waits == [5, 10, 20, 5, 5]

    @respx.mock
    def test_rejects_string_expires_in_in_token_response(self):
        # A 2xx with a valid access_token but expires_in "3600" (a string) must
        # surface as api_error, never escape as a TypeError from expires_at math.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "expires_in": "3600"})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 200

    @respx.mock
    def test_accepts_token_without_expires_in(self):
        # An absent expires_in is allowed — the token simply carries no expiry.
        payload = {k: v for k, v in TOKEN_RESPONSE.items() if k != "expires_in"}
        _queue_token_responses([httpx.Response(200, json=payload)])

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
        )

        assert token.expires_in is None
        assert token.expires_at is None
        assert token.is_expired() is False

    @respx.mock
    def test_rejects_infinite_expires_in_in_token_response(self):
        # A server sending expires_in: 1e400 (parses to inf via json.loads) is
        # numeric and positive, but it would make expires_at inf so the token
        # would never expire. Sent as a raw body since httpx's json= encoder
        # rejects inf. Must surface api_error.
        raw = json.dumps({k: v for k, v in TOKEN_RESPONSE.items() if k != "expires_in"})
        raw = raw[:-1] + ', "expires_in": 1e400}'
        _queue_token_responses([httpx.Response(200, content=raw, headers={"Content-Type": "application/json"})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 200

    @respx.mock
    def test_rejects_oversized_expires_in_in_token_response(self):
        # One past the 2_147_483_647 s ceiling is a malformed lifetime, not a
        # schedulable deadline.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "expires_in": 2_147_483_648})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_accepts_max_token_lifetime(self):
        # The ceiling itself is accepted — the token carries the full lifetime.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "expires_in": 2_147_483_647})])

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
        )

        assert token.expires_in == 2_147_483_647

    @respx.mock
    def test_rejects_bool_expires_in_in_token_response(self):
        # True is an int subclass but never a lifetime.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "expires_in": True})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    @pytest.mark.parametrize("value", [0, -1])
    def test_rejects_non_positive_expires_in_in_token_response(self, value):
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "expires_in": value})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_explicit_empty_token_type(self):
        # An explicit "" token_type is malformed token metadata → api_error,
        # uniform across all five SDKs.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "token_type": ""})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_null_token_type_defaults_to_bearer(self):
        # JSON null is treated as absent (the Go/Kotlin decoders cannot
        # distinguish them) → Bearer default, uniform across all five SDKs.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "token_type": None})])

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
        )

        assert token.token_type == "Bearer"

    @respx.mock
    @pytest.mark.parametrize("field", ["refresh_token", "token_type", "scope"])
    def test_rejects_non_string_token_fields(self, field):
        # A numeric refresh_token/token_type/scope is a malformed response, not a
        # usable credential field — surface api_error rather than store a non-string.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, field: 123})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_rejects_fractional_expires_in_in_token_response(self):
        # A fractional token lifetime is malformed under the whole-second contract
        # → api_error, uniform across SDKs (each validates the decoded value).
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "expires_in": 1.5})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_accepts_integer_valued_float_expires_in_in_token_response(self):
        # An integer-valued float (3600.0) is accepted and coerced to int,
        # matching the device-duration rule and OAuthToken's ``int | None`` type.
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "expires_in": 3600.0})])

        token = poll_device_token(
            TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
        )

        assert token.expires_in == 3600
        assert isinstance(token.expires_in, int)

    @respx.mock
    def test_expires_against_injected_clock(self):
        _queue_token_responses([httpx.Response(400, json={"error": "authorization_pending"})])
        # Clock: base at 0, then jumps past the 900s deadline on the first check.
        times = iter([0, 1_000_000])

        with pytest.raises(DeviceFlowError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT,
                "basecamp-cli",
                "dev-code-123",
                interval=5,
                expires_in=900,
                sleep=RecordingSleep(),
                clock=lambda: next(times),
            )
        assert exc_info.value.reason == "expired"
        assert exc_info.value.code == "auth_required"

    @respx.mock
    def test_access_denied_maps_to_auth(self):
        _queue_token_responses([httpx.Response(400, json={"error": "access_denied"})])

        with pytest.raises(DeviceFlowError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.reason == "access_denied"
        assert exc_info.value.code == "auth_required"

    @respx.mock
    def test_expired_token_error_maps_to_expired(self):
        _queue_token_responses([httpx.Response(400, json={"error": "expired_token"})])

        with pytest.raises(DeviceFlowError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.reason == "expired"
        assert exc_info.value.code == "auth_required"

    @respx.mock
    def test_transport_failure_is_network_and_retryable(self):
        _queue_token_responses([httpx.ConnectError("boom")])

        with pytest.raises(DeviceFlowError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.reason == "transport"
        assert exc_info.value.code == "network"
        assert exc_info.value.retryable is True

    @respx.mock
    def test_cancellation_raises_cancelled(self):
        _queue_token_responses([httpx.Response(400, json={"error": "authorization_pending"})])
        cancelled = {"flag": False}

        # Cancel during the first sleep, mirroring an aborted signal.
        def sleep(_seconds: float) -> None:
            cancelled["flag"] = True

        with pytest.raises(DeviceFlowError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT,
                "basecamp-cli",
                "dev-code-123",
                interval=5,
                expires_in=900,
                sleep=sleep,
                should_cancel=lambda: cancelled["flag"],
            )
        assert exc_info.value.reason == "cancelled"
        assert exc_info.value.code == "usage"

    @respx.mock
    def test_cancellation_during_wait_is_prompt(self):
        # A long interval must not delay cancellation: the wait polls should_cancel
        # in small chunks, so a cancel set mid-wait raises without sleeping the whole
        # interval at once (a plain time.sleep is not interruptible).
        from basecamp.oauth.device import _CANCEL_POLL_INTERVAL

        _queue_token_responses([httpx.Response(400, json={"error": "authorization_pending"})])
        recorded: list[float] = []

        def sleep(seconds: float) -> None:
            recorded.append(seconds)

        def should_cancel() -> bool:
            return len(recorded) >= 3  # cancel after the 3rd chunk

        with pytest.raises(DeviceFlowError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT,
                "basecamp-cli",
                "dev-code-123",
                interval=5,
                expires_in=900,
                sleep=sleep,
                should_cancel=should_cancel,
            )
        assert exc_info.value.reason == "cancelled"
        # Chunked into small waits, never one sleep(5).
        assert recorded and all(s <= _CANCEL_POLL_INTERVAL for s in recorded)

    @respx.mock
    def test_unknown_error_maps_to_api_error(self):
        _queue_token_responses([httpx.Response(400, json={"error": "invalid_request"})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"
        assert not isinstance(exc_info.value, DeviceFlowError)

    @respx.mock
    def test_rejects_non_object_token_body_with_http_status(self):
        # A valid-JSON-but-non-object token body must fail api_error AND carry the
        # HTTP status, matching the other token-poll error raises.
        _queue_token_responses([httpx.Response(200, content="[]", headers={"Content-Type": "application/json"})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 200

    @respx.mock
    def test_rejects_non_string_access_token(self):
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "access_token": 12345})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_oversized_body_aborts(self):
        _queue_token_responses([httpx.Response(200, json={**TOKEN_RESPONSE, "pad": "x" * (256 * 1024)})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT,
                "basecamp-cli",
                "dev-code-123",
                interval=5,
                expires_in=900,
                sleep=RecordingSleep(),
                max_body_bytes=8 * 1024,
            )
        assert exc_info.value.code == "api_error"

    @respx.mock
    def test_does_not_follow_redirect(self):
        redirect_target = "https://evil.device-test.example/steal"
        respx.post(TOKEN_ENDPOINT).mock(
            return_value=httpx.Response(302, headers={"Location": redirect_target}, json={})
        )
        target = respx.get(redirect_target).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        with pytest.raises(OAuthError):
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert not target.called  # redirect suppressed — attacker origin never dialed

    @respx.mock
    def test_redirect_with_pending_body_is_api_error(self):
        # A 3xx must never be interpreted as an OAuth outcome: a redirect body
        # carrying {"error": "authorization_pending"} must not keep the poll
        # loop alive — it surfaces as api_error.
        _queue_token_responses([httpx.Response(302, json={"error": "authorization_pending"})])

        with pytest.raises(OAuthError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT, "basecamp-cli", "dev-code-123", interval=5, expires_in=900, sleep=RecordingSleep()
            )
        assert exc_info.value.code == "api_error"
        assert exc_info.value.http_status == 302

    @respx.mock
    def test_deadline_clamps_backoff_wait(self):
        # Every poll times out, so the backoff interval doubles unbounded (5 → 10
        # → 20…). A clock advanced by each recorded sleep proves the final wait is
        # clamped to the remaining deadline and never overshoots expiry.
        _queue_token_responses([httpx.TimeoutException("timed out")] * 5)

        now = {"t": 0.0}
        waits: list[float] = []

        def clock() -> float:
            return now["t"]

        def sleep(seconds: float) -> None:
            waits.append(seconds)
            now["t"] += seconds

        with pytest.raises(DeviceFlowError) as exc_info:
            poll_device_token(
                TOKEN_ENDPOINT,
                "basecamp-cli",
                "dev-code-123",
                interval=5,
                expires_in=20,
                clock=clock,
                sleep=sleep,
            )

        assert exc_info.value.reason == "expired"
        # Waits: 5 (→t=5), 10 (→t=15), then clamped to remaining 5 (→t=20) — the
        # grown 20s backoff never sleeps past the 20s deadline.
        assert waits == [5, 10, 5]
        assert now["t"] <= 20


class TestPerformDeviceLogin:
    @respx.mock
    def test_capability_guard_no_device_grant_is_unavailable_and_does_not_poll(self):
        token_route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))
        config = OAuthConfig(
            issuer=ORIGIN,
            authorization_endpoint=None,
            token_endpoint=TOKEN_ENDPOINT,
            device_authorization_endpoint=DEVICE_ENDPOINT,
            grant_types_supported=["refresh_token"],  # no device_code grant
        )

        with pytest.raises(DeviceFlowError) as exc_info:
            perform_device_login(config, "basecamp-cli", display=lambda _auth: None)

        assert exc_info.value.reason == "unavailable"
        assert exc_info.value.code == "validation"
        assert not token_route.called

    @respx.mock
    def test_capability_guard_rejects_string_grant_types(self):
        # A malformed config carrying the URN as a plain str would substring-match
        # a bare `in` — the guard requires a real list, so this is unavailable.
        device_route = respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(200, json=DEVICE_AUTH_RESPONSE))
        config = OAuthConfig(
            issuer=ORIGIN,
            authorization_endpoint=None,
            token_endpoint=TOKEN_ENDPOINT,
            device_authorization_endpoint=DEVICE_ENDPOINT,
            grant_types_supported=DEVICE_CODE_GRANT_TYPE,  # type: ignore[arg-type]
        )

        with pytest.raises(DeviceFlowError) as exc_info:
            perform_device_login(config, "basecamp-cli", display=lambda _auth: None)

        assert exc_info.value.reason == "unavailable"
        assert not device_route.called

    @respx.mock
    def test_capability_guard_missing_endpoint_is_unavailable(self):
        config = OAuthConfig(
            issuer=ORIGIN,
            authorization_endpoint=None,
            token_endpoint=TOKEN_ENDPOINT,
            device_authorization_endpoint=None,
            grant_types_supported=[DEVICE_CODE_GRANT_TYPE],
        )

        with pytest.raises(DeviceFlowError) as exc_info:
            perform_device_login(config, "basecamp-cli", display=lambda _auth: None)
        assert exc_info.value.reason == "unavailable"

    @respx.mock
    def test_fires_display_hook_then_completes(self):
        respx.post(DEVICE_ENDPOINT).mock(return_value=httpx.Response(200, json=DEVICE_AUTH_RESPONSE))
        respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))
        shown = []

        token = perform_device_login(
            CONFIG,
            "basecamp-cli",
            display=shown.append,
            sleep=RecordingSleep(),
        )

        assert len(shown) == 1
        assert shown[0].user_code == "WDJB-MJHT"
        assert token.access_token == "device_access_token"  # gitleaks:allow

    @respx.mock
    def test_preserves_sub_second_remaining_lifetime(self):
        # After the display hook consumes most of a short lifetime, the fractional
        # remaining (0.4s) must be preserved — flooring it to an int would expire
        # the flow immediately even though time still remains.
        respx.post(DEVICE_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**DEVICE_AUTH_RESPONSE, "expires_in": 1})
        )
        respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))
        # issued_at=0.0, post-display=0.6 → remaining 0.4s; the rest anchor/poll.
        times = iter([0.0, 0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.6])

        token = perform_device_login(
            CONFIG,
            "basecamp-cli",
            display=lambda _auth: None,
            clock=lambda: next(times),
            sleep=RecordingSleep(),
        )

        assert token.access_token == "device_access_token"  # gitleaks:allow


class TestDeviceFlowErrorRetryability:
    """The reason is authoritative for retryability: only ``transport`` retries,
    and a caller-supplied ``retryable`` kwarg must not flip that invariant."""

    @pytest.mark.parametrize(
        ("reason", "expected"),
        [
            ("transport", True),
            ("access_denied", False),
            ("expired", False),
            ("unavailable", False),
            ("cancelled", False),
        ],
    )
    def test_retryable_derives_from_reason(self, reason, expected):
        assert DeviceFlowError(reason, "boom").retryable is expected

    def test_caller_cannot_make_transport_non_retryable(self):
        # A rogue ``retryable=False`` must not defeat transport's retryability.
        assert DeviceFlowError("transport", "boom", retryable=False).retryable is True

    @pytest.mark.parametrize("reason", ["access_denied", "expired", "unavailable", "cancelled"])
    def test_caller_cannot_make_terminal_reason_retryable(self, reason):
        # A rogue ``retryable=True`` must not make a terminal denial retryable.
        assert DeviceFlowError(reason, "boom", retryable=True).retryable is False

    def test_unknown_reason_defaults_to_api_error_not_keyerror(self):
        # An unexpected reason string must not leak a raw KeyError; it maps to
        # api_error (defensive .get(), mirroring OAuthError) and stays non-retryable.
        err = DeviceFlowError("bogus_reason", "boom")  # type: ignore[arg-type]
        assert err.code == "api_error"
        assert err.retryable is False
