from __future__ import annotations

import httpx
import pytest
import respx

from basecamp.oauth.errors import OAuthError
from basecamp.oauth.exchange import exchange_code, refresh_token
from basecamp.oauth.token import OAuthToken

TOKEN_ENDPOINT = "https://launchpad.37signals.com/authorization/token"

TOKEN_RESPONSE = {
    "access_token": "BAhbB0kiAbB7ImNsa",  # gitleaks:allow (test fixture)
    "token_type": "Bearer",
    "refresh_token": "BAhbB0kiAbR7ImNsa",  # gitleaks:allow (test fixture)
    "expires_in": 1209600,
    "scope": "read write",
}


class TestExchangeCode:
    @respx.mock
    def test_exchange_code(self):
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        token = exchange_code(
            TOKEN_ENDPOINT,
            code="auth-code-123",
            redirect_uri="https://myapp.com/callback",
            client_id="client-id",
            client_secret="client-secret",
        )

        assert isinstance(token, OAuthToken)
        assert token.access_token == "BAhbB0kiAbB7ImNsa"  # gitleaks:allow
        assert token.token_type == "Bearer"
        assert token.refresh_token == "BAhbB0kiAbR7ImNsa"  # gitleaks:allow
        assert token.expires_in == 1209600
        assert token.scope == "read write"

        # Verify the request used standard grant_type
        request = route.calls[0].request
        body = request.content.decode()
        assert "grant_type=authorization_code" in body
        assert "code=auth-code-123" in body

    @respx.mock
    def test_exchange_code_legacy_format(self):
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        exchange_code(
            TOKEN_ENDPOINT,
            code="auth-code-123",
            redirect_uri="https://myapp.com/callback",
            client_id="client-id",
            use_legacy_format=True,
        )

        request = route.calls[0].request
        body = request.content.decode()
        assert "type=web_server" in body
        assert "grant_type" not in body

    @respx.mock
    def test_exchange_error(self):
        respx.post(TOKEN_ENDPOINT).mock(
            return_value=httpx.Response(
                401,
                json={"error": "invalid_grant", "error_description": "Code expired"},
            )
        )

        with pytest.raises(OAuthError) as exc_info:
            exchange_code(
                TOKEN_ENDPOINT,
                code="bad-code",
                redirect_uri="https://myapp.com/callback",
                client_id="client-id",
            )

        assert exc_info.value.http_status == 401


class TestRefreshToken:
    @respx.mock
    def test_refresh_token(self):
        new_token_response = {
            "access_token": "new-access-token",
            "token_type": "Bearer",
            "expires_in": 1209600,
        }
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=new_token_response))

        token = refresh_token(
            TOKEN_ENDPOINT,
            refresh_tok="refresh-tok-123",
            client_id="client-id",
            client_secret="client-secret",
        )

        assert token.access_token == "new-access-token"
        assert token.expires_in == 1209600

        request = route.calls[0].request
        body = request.content.decode()
        assert "grant_type=refresh_token" in body
        assert "refresh_token=refresh-tok-123" in body

    @respx.mock
    def test_refresh_token_legacy_format(self):
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        refresh_token(
            TOKEN_ENDPOINT,
            refresh_tok="refresh-tok-123",
            use_legacy_format=True,
        )

        request = route.calls[0].request
        body = request.content.decode()
        assert "type=refresh" in body
        assert "grant_type" not in body
