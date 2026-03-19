from __future__ import annotations

import threading
import time
from unittest.mock import MagicMock, patch

import httpx
import pytest

from basecamp.auth import BearerAuth, OAuthTokenProvider, StaticTokenProvider
from basecamp.errors import AuthError, NetworkError


class TestStaticTokenProvider:
    def test_returns_fixed_token(self):
        tp = StaticTokenProvider("my-token")
        assert tp.access_token() == "my-token"

    def test_not_refreshable(self):
        tp = StaticTokenProvider("tok")
        assert tp.refreshable is False

    def test_refresh_returns_false(self):
        tp = StaticTokenProvider("tok")
        assert tp.refresh() is False


class TestBearerAuth:
    def test_adds_authorization_header(self):
        tp = StaticTokenProvider("abc123")
        auth = BearerAuth(tp)
        headers: dict[str, str] = {}
        auth.authenticate(headers)
        assert headers["Authorization"] == "Bearer abc123"

    def test_overwrites_existing_header(self):
        tp = StaticTokenProvider("new")
        auth = BearerAuth(tp)
        headers = {"Authorization": "Bearer old"}
        auth.authenticate(headers)
        assert headers["Authorization"] == "Bearer new"


class TestOAuthTokenProvider:
    def test_returns_token_when_not_expired(self):
        tp = OAuthTokenProvider(
            access_token="valid",
            client_id="cid",
            client_secret="csec",
            expires_at=time.time() + 3600,
        )
        assert tp.access_token() == "valid"

    def test_refreshable_with_refresh_token(self):
        tp = OAuthTokenProvider(
            access_token="tok",
            client_id="cid",
            client_secret="csec",
            refresh_token="rtok",
        )
        assert tp.refreshable is True

    def test_not_refreshable_without_refresh_token(self):
        tp = OAuthTokenProvider(
            access_token="tok",
            client_id="cid",
            client_secret="csec",
        )
        assert tp.refreshable is False

    @patch("httpx.post")
    def test_refresh_when_expired(self, mock_post):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "access_token": "new-token",
            "expires_in": 7200,
        }
        mock_post.return_value = mock_response

        tp = OAuthTokenProvider(
            access_token="old",
            client_id="cid",
            client_secret="csec",
            refresh_token="rtok",
            expires_at=time.time() - 10,  # expired
        )
        token = tp.access_token()
        assert token == "new-token"
        mock_post.assert_called_once()

    @patch("httpx.post")
    def test_refresh_failure_raises_auth_error(self, mock_post):
        mock_response = MagicMock()
        mock_response.status_code = 401
        mock_post.return_value = mock_response

        tp = OAuthTokenProvider(
            access_token="old",
            client_id="cid",
            client_secret="csec",
            refresh_token="rtok",
            expires_at=time.time() - 10,
        )
        with pytest.raises(AuthError):
            tp.access_token()

    @patch("httpx.post")
    def test_network_error_during_refresh(self, mock_post):
        mock_post.side_effect = httpx.ConnectError("connection refused")

        tp = OAuthTokenProvider(
            access_token="old",
            client_id="cid",
            client_secret="csec",
            refresh_token="rtok",
            expires_at=time.time() - 10,
        )
        with pytest.raises(NetworkError):
            tp.access_token()

    @patch("httpx.post")
    def test_on_refresh_callback_invoked(self, mock_post):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "access_token": "refreshed",
            "expires_in": 3600,
        }
        mock_post.return_value = mock_response

        callback = MagicMock()
        tp = OAuthTokenProvider(
            access_token="old",
            client_id="cid",
            client_secret="csec",
            refresh_token="rtok",
            expires_at=time.time() - 10,
            on_refresh=callback,
        )
        tp.access_token()
        callback.assert_called_once()

    @patch("httpx.post")
    def test_thread_safety(self, mock_post):
        call_count = 0

        def slow_post(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            time.sleep(0.01)
            resp = MagicMock()
            resp.status_code = 200
            resp.json.return_value = {"access_token": "new", "expires_in": 3600}
            return resp

        mock_post.side_effect = slow_post

        tp = OAuthTokenProvider(
            access_token="old",
            client_id="cid",
            client_secret="csec",
            refresh_token="rtok",
            expires_at=time.time() - 10,
        )

        threads = [threading.Thread(target=tp.access_token) for _ in range(5)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        # Lock serializes access, so refresh called once (first thread refreshes,
        # subsequent threads see non-expired token)
        assert mock_post.call_count == 1
