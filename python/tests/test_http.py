from __future__ import annotations

import contextlib

import httpx
import pytest
import respx

from basecamp._async_http import AsyncHttpClient
from basecamp._http import HttpClient
from basecamp.async_auth import AsyncBearerAuth, AsyncStaticTokenProvider
from basecamp.auth import BearerAuth, StaticTokenProvider
from basecamp.config import Config
from basecamp.errors import (
    ApiError,
    AuthError,
    NetworkError,
    NotFoundError,
    RateLimitError,
    UsageError,
)
from basecamp.hooks import BasecampHooks


def make_client(max_retries=3, base_delay=0.001, max_jitter=0.0, timeout=30.0):
    config = Config(
        base_url="https://3.basecampapi.com",
        max_retries=max_retries,
        base_delay=base_delay,
        max_jitter=max_jitter,
        timeout=timeout,
    )
    auth = BearerAuth(StaticTokenProvider("test-token"))
    return HttpClient(config, auth, BasecampHooks())


class TestSuccessResponses:
    @respx.mock
    def test_get_success(self):
        respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(200, json={"ok": True}))
        client = make_client()
        resp = client.get("/test")
        assert resp.status_code == 200
        assert resp.json() == {"ok": True}

    @respx.mock
    def test_post_success(self):
        respx.post("https://3.basecampapi.com/items").mock(return_value=httpx.Response(201, json={"id": 1}))
        client = make_client()
        resp = client.post("/items", json_body={"name": "test"})
        assert resp.status_code == 201

    @respx.mock
    def test_put_success(self):
        respx.put("https://3.basecampapi.com/items/1").mock(return_value=httpx.Response(200, json={"id": 1}))
        client = make_client()
        resp = client.put("/items/1", json_body={"name": "updated"})
        assert resp.status_code == 200

    @respx.mock
    def test_delete_success(self):
        respx.delete("https://3.basecampapi.com/items/1").mock(return_value=httpx.Response(204))
        client = make_client()
        resp = client.delete("/items/1")
        assert resp.status_code == 204


class TestErrorMapping:
    @respx.mock
    def test_401_maps_to_auth_error(self):
        respx.get("https://3.basecampapi.com/test").mock(
            return_value=httpx.Response(401, json={"error": "Unauthorized"})
        )
        client = make_client()
        with pytest.raises(AuthError):
            client.get("/test")

    @respx.mock
    def test_404_maps_to_not_found(self):
        respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(404, json={"error": "Not found"}))
        client = make_client()
        with pytest.raises(NotFoundError):
            client.get("/test")

    @respx.mock
    def test_429_maps_to_rate_limit(self):
        respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(429, headers={"Retry-After": "1"}))
        client = make_client(max_retries=1)
        with pytest.raises(RateLimitError):
            client.get("/test")


class TestRetryBehavior:
    @respx.mock
    def test_get_retries_on_429(self):
        route = respx.get("https://3.basecampapi.com/test")
        route.side_effect = [
            httpx.Response(429, headers={"Retry-After": "0"}),
            httpx.Response(429, headers={"Retry-After": "0"}),
            httpx.Response(200, json={"ok": True}),
        ]
        client = make_client(max_retries=3)
        resp = client.get("/test")
        assert resp.status_code == 200
        assert route.call_count == 3

    @respx.mock
    def test_get_retries_on_503(self):
        route = respx.get("https://3.basecampapi.com/test")
        route.side_effect = [
            httpx.Response(503),
            httpx.Response(200, json={"ok": True}),
        ]
        client = make_client(max_retries=3)
        resp = client.get("/test")
        assert resp.status_code == 200
        assert route.call_count == 2

    @respx.mock
    def test_get_retries_on_500(self):
        route = respx.get("https://3.basecampapi.com/test")
        route.side_effect = [
            httpx.Response(500),
            httpx.Response(200, json={"ok": True}),
        ]
        client = make_client(max_retries=3)
        resp = client.get("/test")
        assert resp.status_code == 200

    @respx.mock
    def test_get_gives_up_after_max_retries(self):
        respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(503))
        client = make_client(max_retries=2)
        with pytest.raises(ApiError):
            client.get("/test")

    @respx.mock
    def test_post_does_not_retry_by_default(self):
        route = respx.post("https://3.basecampapi.com/test")
        route.mock(return_value=httpx.Response(503))
        client = make_client(max_retries=3)
        with pytest.raises(ApiError):
            client.post("/test", json_body={"x": 1})
        assert route.call_count == 1


def make_async_client(max_retries=3, base_delay=0.001, max_jitter=0.0, timeout=30.0):
    config = Config(
        base_url="https://3.basecampapi.com",
        max_retries=max_retries,
        base_delay=base_delay,
        max_jitter=max_jitter,
        timeout=timeout,
    )
    auth = AsyncBearerAuth(AsyncStaticTokenProvider("test-token"))
    return AsyncHttpClient(config, auth, BasecampHooks())


class TestTotalAttemptSemantics:
    """max_retries is a TOTAL attempt count (including the initial request), not
    the number of retries after the first attempt. 3 → 3 attempts, 0 → a single
    attempt with no retry. Sync and async transports must agree."""

    @pytest.mark.parametrize(
        "max_retries,want_attempts",
        [(0, 1), (1, 1), (3, 3)],
    )
    @respx.mock
    def test_sync_attempt_count(self, max_retries, want_attempts):
        route = respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(503))
        client = make_client(max_retries=max_retries)
        with pytest.raises(ApiError):
            client.get("/test")
        assert route.call_count == want_attempts

    @pytest.mark.parametrize(
        "max_retries,want_attempts",
        [(0, 1), (1, 1), (3, 3)],
    )
    @pytest.mark.asyncio
    @respx.mock
    async def test_async_attempt_count(self, max_retries, want_attempts):
        route = respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(503))
        client = make_async_client(max_retries=max_retries)
        with pytest.raises(ApiError):
            await client.get("/test")
        assert route.call_count == want_attempts

    @respx.mock
    def test_sync_succeeds_on_final_allowed_attempt(self):
        # 3 attempts: two 503s then a 200 — the success on the last allowed
        # attempt proves the loop makes exactly max_retries attempts.
        route = respx.get("https://3.basecampapi.com/test")
        route.side_effect = [
            httpx.Response(503),
            httpx.Response(503),
            httpx.Response(200, json={"ok": True}),
        ]
        client = make_client(max_retries=3)
        resp = client.get("/test")
        assert resp.status_code == 200
        assert route.call_count == 3

    def test_negative_max_retries_rejected_at_construction(self):
        with pytest.raises(ValueError):
            Config(base_url="https://3.basecampapi.com", max_retries=-1)


class TestNetworkErrors:
    @respx.mock
    def test_connection_error_maps_to_network_error(self):
        respx.get("https://3.basecampapi.com/test").mock(side_effect=httpx.ConnectError("connection refused"))
        client = make_client(max_retries=1)
        with pytest.raises(NetworkError):
            client.get("/test")


class TestRetryAfter:
    @respx.mock
    def test_retry_after_header_respected(self):
        from unittest.mock import patch

        route = respx.get("https://3.basecampapi.com/test")
        route.side_effect = [
            httpx.Response(429, headers={"Retry-After": "1"}),
            httpx.Response(200, json={"ok": True}),
        ]
        client = make_client(max_retries=3)
        with patch("time.sleep") as mock_sleep:
            resp = client.get("/test")
        assert resp.status_code == 200
        mock_sleep.assert_called_once_with(1.0)


class TestHeaders:
    @respx.mock
    def test_user_agent_set(self):
        route = respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(200))
        client = make_client()
        client.get("/test")
        request = route.calls[0].request
        assert "basecamp-sdk-python" in request.headers["user-agent"]

    @respx.mock
    def test_authorization_header_set(self):
        route = respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(200))
        client = make_client()
        client.get("/test")
        request = route.calls[0].request
        assert request.headers["authorization"] == "Bearer test-token"


class TestSameOriginGuard:
    @respx.mock
    def test_foreign_origin_absolute_rejected_without_egress(self):
        route = respx.get("https://evil.example/steal.json").mock(return_value=httpx.Response(200))
        client = make_client()
        with pytest.raises(UsageError):
            client.get("https://evil.example/steal.json")
        assert route.call_count == 0

    @respx.mock
    def test_foreign_origin_post_rejected(self):
        route = respx.post("https://evil.example/x").mock(return_value=httpx.Response(200))
        client = make_client()
        with pytest.raises(UsageError):
            client.post("https://evil.example/x", json_body={"a": 1})
        assert route.call_count == 0

    @respx.mock
    def test_foreign_origin_put_rejected(self):
        route = respx.put("https://evil.example/x").mock(return_value=httpx.Response(200))
        client = make_client()
        with pytest.raises(UsageError):
            client.put("https://evil.example/x", json_body={"a": 1})
        assert route.call_count == 0

    @respx.mock
    def test_foreign_origin_delete_rejected(self):
        route = respx.delete("https://evil.example/x").mock(return_value=httpx.Response(204))
        client = make_client()
        with pytest.raises(UsageError):
            client.delete("https://evil.example/x")
        assert route.call_count == 0

    @respx.mock
    def test_same_origin_absolute_carries_token(self):
        route = respx.get("https://3.basecampapi.com/page2.json").mock(return_value=httpx.Response(200, json={}))
        client = make_client()
        resp = client.get("https://3.basecampapi.com/page2.json")
        assert resp.status_code == 200
        assert route.calls[0].request.headers["authorization"] == "Bearer test-token"

    @respx.mock
    def test_relative_path_resolves(self):
        route = respx.get("https://3.basecampapi.com/projects.json").mock(return_value=httpx.Response(200, json=[]))
        client = make_client()
        resp = client.get("/projects.json")
        assert resp.status_code == 200
        assert route.call_count == 1

    @respx.mock
    def test_localhost_base_allows_absolute(self):
        config = Config(
            base_url="https://localhost:3000",
            max_retries=1,
            base_delay=0.001,
            max_jitter=0.0,
            timeout=5.0,
        )
        client = HttpClient(config, BearerAuth(StaticTokenProvider("test-token")), BasecampHooks())
        route = respx.get("https://localhost:3000/x.json").mock(return_value=httpx.Response(200, json={}))
        resp = client.get("https://localhost:3000/x.json")
        assert resp.status_code == 200
        assert route.call_count == 1

    @respx.mock
    def test_get_absolute_allows_cross_origin_launchpad(self):
        route = respx.get("https://launchpad.37signals.com/authorization.json").mock(
            return_value=httpx.Response(200, json={"ok": True})
        )
        client = make_client()
        resp = client.get_absolute("https://launchpad.37signals.com/authorization.json")
        assert resp.status_code == 200
        assert route.calls[0].request.headers["authorization"] == "Bearer test-token"

    @respx.mock
    def test_get_absolute_rejects_foreign_origin(self):
        # get_absolute must not be a blanket origin-guard bypass: only the
        # trusted Launchpad authorization endpoint may receive credentials
        # cross-origin. Any other foreign origin is rejected before egress.
        route = respx.get("https://evil.example/steal").mock(return_value=httpx.Response(200, json={}))
        client = make_client()
        with pytest.raises(UsageError, match="different origin"):
            client.get_absolute("https://evil.example/steal")
        assert route.call_count == 0

    def test_get_absolute_rejects_non_http_scheme_for_localhost(self):
        # The localhost carve-out is limited to HTTP(S): any other scheme must
        # fail closed before credentials could be attached.
        client = make_client()
        with pytest.raises(UsageError, match="HTTPS"):
            client.get_absolute("ws://localhost:3000/x")

    def test_build_url_uppercase_scheme_treated_as_absolute(self):
        # Schemes are case-insensitive (RFC 3986): an uppercase-scheme URL is
        # still absolute — same-origin passes through, foreign is rejected
        # rather than joined onto the base URL.
        client = make_client()
        assert client._build_url("HTTPS://3.basecampapi.com/x.json") == "HTTPS://3.basecampapi.com/x.json"
        with pytest.raises(UsageError, match="origin"):
            client._build_url("HTTPS://evil.example/x.json")


class TestParserDifferentialEgress:
    """End-to-end parser-differential regression: every adversarial URL, driven
    through the real token-attach path, must either be rejected by the guard or
    egress only to a host the token may reach — NEVER to a foreign host
    carrying Authorization. Guards against the class of bug where the guard
    extracts the host with one parser while httpx dials with another."""

    ADVERSARIAL_URLS = [
        "http://evil.example\\.localhost/x",
        "http://localhost@evil.example/x",
        "http://evil.example#foo.localhost",
        "http://evil.example?x=.localhost",
        "http://localhost:80@evil.example/x",
        "https://3.basecampapi.com:443@evil.example/x",
        "http://[::1]/x",
        "HTTPS://localhost/x",
        "https://3.basecampapi.com:443/x",
        "http://localhost.evil.example/x",
    ]

    @staticmethod
    def _token_may_reach(host: str) -> bool:
        # The configured base origin plus the localhost carve-out.
        return host in ("3.basecampapi.com", "localhost", "127.0.0.1", "::1") or host.endswith(".localhost")

    @respx.mock
    def test_adversarial_urls_never_egress_token_to_foreign_host(self):
        respx.route().mock(return_value=httpx.Response(200, json={}))
        client = make_client(max_retries=1)
        for url in self.ADVERSARIAL_URLS:
            # Rejection before egress is a passing outcome.
            with contextlib.suppress(UsageError, httpx.InvalidURL):
                client.get(url)
        for call in respx.calls:
            host = call.request.url.host.lower()
            auth = call.request.headers.get("authorization")
            assert self._token_may_reach(host) or auth is None, f"Bearer token egressed to foreign host {host!r}"


# Per-operation retry ceiling: metadata carries a retry.max per operation, and
# the effective attempts are min(client cap, op max). The ceiling can only
# reduce attempts below the client cap, never raise them, matching how TS/Swift/
# Kotlin drive their loops from the per-op max while still honoring a Go/Python
# client that lowered its cap. Sync and async transports must agree.
_PEROP_META = {
    "CapTwoOp": {"idempotent": True, "retry": {"max": 2}},
    "CapThreeOp": {"idempotent": True, "retry": {"max": 3}},
}


def make_client_meta(metadata, max_retries=3):
    config = Config(
        base_url="https://3.basecampapi.com",
        max_retries=max_retries,
        base_delay=0.001,
        max_jitter=0.0,
        timeout=30.0,
    )
    auth = BearerAuth(StaticTokenProvider("test-token"))
    return HttpClient(config, auth, BasecampHooks(), metadata=metadata)


def make_async_client_meta(metadata, max_retries=3):
    config = Config(
        base_url="https://3.basecampapi.com",
        max_retries=max_retries,
        base_delay=0.001,
        max_jitter=0.0,
        timeout=30.0,
    )
    auth = AsyncBearerAuth(AsyncStaticTokenProvider("test-token"))
    return AsyncHttpClient(config, auth, BasecampHooks(), metadata=metadata)


class TestPerOperationRetryMax:
    # (operation, client_cap, want_attempts)
    CASES = [
        ("CapTwoOp", 3, 2),  # op ceiling binds below default cap: min(3, 2)
        ("CapTwoOp", 5, 2),  # op ceiling binds below a raised cap: min(5, 2)
        ("CapTwoOp", 1, 1),  # client cap below op ceiling honored: min(1, 2)
        ("CapThreeOp", 3, 3),  # op ceiling equals cap (control): min(3, 3)
    ]

    @pytest.mark.parametrize("operation,cap,want", CASES)
    @respx.mock
    def test_sync_ceiling(self, operation, cap, want):
        route = respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(503))
        client = make_client_meta(_PEROP_META, max_retries=cap)
        with pytest.raises(ApiError):
            client.get("/test", operation=operation)
        assert route.call_count == want

    @pytest.mark.parametrize("operation,cap,want", CASES)
    @pytest.mark.asyncio
    @respx.mock
    async def test_async_ceiling(self, operation, cap, want):
        route = respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(503))
        client = make_async_client_meta(_PEROP_META, max_retries=cap)
        with pytest.raises(ApiError):
            await client.get("/test", operation=operation)
        assert route.call_count == want

    @respx.mock
    def test_no_operation_uses_client_cap(self):
        # A GET with no operation name (or an unknown one) keeps the client cap.
        route = respx.get("https://3.basecampapi.com/test").mock(return_value=httpx.Response(503))
        client = make_client_meta(_PEROP_META, max_retries=3)
        with pytest.raises(ApiError):
            client.get("/test")
        assert route.call_count == 3


# Integration: exercise the per-op ceiling through REAL generated services with
# the REAL bundled metadata.json, so the canonical (PascalCase) operationId
# plumbing is covered end-to-end. OperationInfo.operation is snake_case and is
# NOT a metadata key; the transport must receive the canonical operationId
# (threaded via the generated `operation=` kwarg) for GET/list/pagination too —
# otherwise a client cap above an op's max would over-retry those reads.
from basecamp.async_client import AsyncClient  # noqa: E402
from basecamp.client import Client  # noqa: E402


def _integration_config(max_retries):
    return Config(
        base_url="https://3.basecampapi.com",
        max_retries=max_retries,
        base_delay=0.001,
        max_jitter=0.0,
        timeout=30.0,
    )


class TestPerOperationRetryMaxIntegration:
    @respx.mock
    def test_sync_get_list_respects_op_ceiling(self):
        # ListProjects has max:3; a client cap of 5 must still stop at 3.
        route = respx.route(method="GET").mock(return_value=httpx.Response(503))
        client = Client(config=_integration_config(5), access_token="tok")
        with pytest.raises(ApiError):
            client.for_account("999").projects.list()
        assert route.call_count == 3
        client.close()

    @respx.mock
    def test_sync_mutation_respects_op_ceiling(self):
        # UpdateAccountName has max:2; a client cap of 5 must still stop at 2.
        route = respx.route(method="PUT").mock(return_value=httpx.Response(503))
        client = Client(config=_integration_config(5), access_token="tok")
        with pytest.raises(ApiError):
            client.for_account("999").account.update_account_name(name="x")
        assert route.call_count == 2
        client.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_get_list_respects_op_ceiling(self):
        route = respx.route(method="GET").mock(return_value=httpx.Response(503))
        client = AsyncClient(config=_integration_config(5), access_token="tok")
        with pytest.raises(ApiError):
            await client.for_account("999").projects.list()
        assert route.call_count == 3
        await client.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_mutation_respects_op_ceiling(self):
        route = respx.route(method="PUT").mock(return_value=httpx.Response(503))
        client = AsyncClient(config=_integration_config(5), access_token="tok")
        with pytest.raises(ApiError):
            await client.for_account("999").account.update_account_name(name="x")
        assert route.call_count == 2
        await client.close()
