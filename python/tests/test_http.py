from __future__ import annotations

import httpx
import pytest
import respx

from basecamp._http import HttpClient
from basecamp.auth import BearerAuth, StaticTokenProvider
from basecamp.config import Config
from basecamp.errors import (
    ApiDisabledError,
    ApiError,
    AuthError,
    NetworkError,
    NotFoundError,
    RateLimitError,
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
    def test_404_with_api_disabled_reason_maps_to_api_disabled(self):
        respx.get("https://3.basecampapi.com/test").mock(
            return_value=httpx.Response(404, headers={"Reason": "API Disabled"})
        )
        client = make_client()
        with pytest.raises(ApiDisabledError) as exc_info:
            client.get("/test")
        assert exc_info.value.http_status == 404
        assert "Adminland" in exc_info.value.hint

    @respx.mock
    def test_404_with_account_inactive_reason_maps_to_not_found(self):
        respx.get("https://3.basecampapi.com/test").mock(
            return_value=httpx.Response(404, headers={"Reason": "Account Inactive"})
        )
        client = make_client()
        with pytest.raises(NotFoundError) as exc_info:
            client.get("/test")
        assert "inactive" in str(exc_info.value)
        assert "expired trial" in exc_info.value.hint

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
