from __future__ import annotations

from urllib.parse import parse_qs, urlparse

from basecamp.oauth.authorize import build_authorization_url
from basecamp.oauth.pkce import generate_pkce


class TestBuildAuthorizationUrl:
    def test_basic_url(self):
        url = build_authorization_url(
            "https://auth.example.com/authorize",
            client_id="my-client",
            redirect_uri="https://myapp.com/callback",
            state="csrf-token",
        )
        parsed = urlparse(url)
        params = parse_qs(parsed.query)

        assert parsed.scheme == "https"
        assert parsed.netloc == "auth.example.com"
        assert parsed.path == "/authorize"
        assert params["response_type"] == ["code"]
        assert params["client_id"] == ["my-client"]
        assert params["redirect_uri"] == ["https://myapp.com/callback"]
        assert params["state"] == ["csrf-token"]
        assert "code_challenge" not in params
        assert "scope" not in params

    def test_with_scope(self):
        url = build_authorization_url(
            "https://auth.example.com/authorize",
            client_id="c",
            redirect_uri="https://app.com/cb",
            state="s",
            scope="read write",
        )
        params = parse_qs(urlparse(url).query)
        assert params["scope"] == ["read write"]

    def test_with_pkce(self):
        pkce = generate_pkce()
        url = build_authorization_url(
            "https://auth.example.com/authorize",
            client_id="c",
            redirect_uri="https://app.com/cb",
            state="s",
            pkce=pkce,
        )
        params = parse_qs(urlparse(url).query)
        assert params["code_challenge"] == [pkce.challenge]
        assert params["code_challenge_method"] == ["S256"]

    def test_special_characters_encoded(self):
        url = build_authorization_url(
            "https://auth.example.com/authorize",
            client_id="client&id",
            redirect_uri="https://app.com/cb?foo=bar",
            state="state=value",
        )
        params = parse_qs(urlparse(url).query)
        assert params["client_id"] == ["client&id"]
        assert params["redirect_uri"] == ["https://app.com/cb?foo=bar"]
        assert params["state"] == ["state=value"]
