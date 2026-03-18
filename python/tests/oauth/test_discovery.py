from __future__ import annotations

import httpx
import pytest
import respx

from basecamp.errors import UsageError
from basecamp.oauth.config import OAuthConfig
from basecamp.oauth.discovery import LAUNCHPAD_BASE_URL, discover, discover_launchpad
from basecamp.oauth.errors import OAuthError

DISCOVERY_RESPONSE = {
    "issuer": "https://launchpad.37signals.com",
    "authorization_endpoint": "https://launchpad.37signals.com/authorization/new",
    "token_endpoint": "https://launchpad.37signals.com/authorization/token",
    "registration_endpoint": "https://launchpad.37signals.com/integrations",
    "scopes_supported": ["read", "write"],
}


class TestDiscover:
    @respx.mock
    def test_discover_success(self):
        base = "https://launchpad.37signals.com"
        respx.get(f"{base}/.well-known/oauth-authorization-server").mock(
            return_value=httpx.Response(200, json=DISCOVERY_RESPONSE)
        )

        config = discover(base)

        assert isinstance(config, OAuthConfig)
        assert config.issuer == "https://launchpad.37signals.com"
        assert config.authorization_endpoint == "https://launchpad.37signals.com/authorization/new"
        assert config.token_endpoint == "https://launchpad.37signals.com/authorization/token"
        assert config.registration_endpoint == "https://launchpad.37signals.com/integrations"
        assert config.scopes_supported == ["read", "write"]

    def test_discover_https_enforcement(self):
        with pytest.raises(UsageError, match="must use HTTPS"):
            discover("http://example.com")

    @respx.mock
    def test_discover_localhost_allowed(self):
        respx.get("http://localhost:3000/.well-known/oauth-authorization-server").mock(
            return_value=httpx.Response(200, json=DISCOVERY_RESPONSE)
        )

        config = discover("http://localhost:3000")

        assert config.issuer == DISCOVERY_RESPONSE["issuer"]

    @respx.mock
    def test_discover_missing_fields(self):
        base = "https://example.com"
        respx.get(f"{base}/.well-known/oauth-authorization-server").mock(
            return_value=httpx.Response(200, json={"issuer": "https://example.com"})
        )

        with pytest.raises(OAuthError, match="missing required fields"):
            discover(base)

    @respx.mock
    def test_discover_launchpad(self):
        url = f"{LAUNCHPAD_BASE_URL}/.well-known/oauth-authorization-server"
        respx.get(url).mock(return_value=httpx.Response(200, json=DISCOVERY_RESPONSE))

        config = discover_launchpad()

        assert config.issuer == DISCOVERY_RESPONSE["issuer"]

    @respx.mock
    def test_discover_network_error(self):
        base = "https://example.com"
        respx.get(f"{base}/.well-known/oauth-authorization-server").mock(
            side_effect=httpx.ConnectError("connection refused")
        )

        with pytest.raises(OAuthError) as exc_info:
            discover(base)

        assert exc_info.value.oauth_type == "network"
        assert exc_info.value.retryable is True
