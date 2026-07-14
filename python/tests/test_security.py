from __future__ import annotations

import httpx
import pytest

from basecamp._security import (
    check_body_size,
    is_localhost,
    redact_headers,
    require_https,
    same_origin,
    truncate,
)
from basecamp.errors import ApiError, UsageError

# URLs crafted so that two URL parsers may disagree about the host (backslash,
# userinfo, fragment, query, default-port tricks). Shared shape with the
# Kotlin and Swift SDK test suites.
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


class TestSameOrigin:
    def test_matching(self):
        assert same_origin("https://example.com/a", "https://example.com/b") is True

    def test_different_scheme(self):
        assert same_origin("https://example.com", "http://example.com") is False

    def test_different_host(self):
        assert same_origin("https://a.com", "https://b.com") is False

    def test_different_port(self):
        assert same_origin("https://example.com:443", "https://example.com:8443") is False

    def test_default_port_matches(self):
        assert same_origin("https://example.com", "https://example.com:443") is True

    def test_http_default_port_matches(self):
        assert same_origin("http://example.com", "http://example.com:80") is True

    def test_missing_scheme_false(self):
        assert same_origin("example.com", "https://example.com") is False


class TestRequireHttps:
    def test_https_ok(self):
        require_https("https://example.com")  # should not raise

    def test_http_raises(self):
        with pytest.raises(UsageError, match="must use HTTPS"):
            require_https("http://example.com")

    def test_custom_label(self):
        with pytest.raises(UsageError, match="base URL must use HTTPS"):
            require_https("http://example.com", "base URL")


class TestIsLocalhost:
    @pytest.mark.parametrize(
        "url",
        [
            "http://localhost",
            "http://localhost:3000",
            "http://127.0.0.1",
            "http://127.0.0.1:8080",
            "http://[::1]",
            "http://app.localhost",
            "http://sub.localhost:3000",
        ],
    )
    def test_localhost_true(self, url):
        assert is_localhost(url) is True

    @pytest.mark.parametrize(
        "url",
        [
            "https://example.com",
            "https://notlocalhost.com",
            "https://api.basecamp.com",
            # Localhost text in userinfo, fragment, or query must not make a
            # foreign host pass the carve-out.
            "http://localhost@evil.example/x",
            "http://localhost:80@evil.example/x",
            "http://evil.example#foo.localhost",
            "http://evil.example?x=.localhost",
            "http://localhost.evil.example/x",
            # A scheme-less string is not an absolute localhost URL.
            "localhost",
        ],
    )
    def test_non_localhost_false(self, url):
        assert is_localhost(url) is False


class TestParserDifferential:
    """A security guard must decide with the SAME parser the transport uses to
    dial (httpx.URL). Whenever the guard blesses a URL, the host httpx would
    actually dial must be the host the guard thought it blessed. Fails loudly
    if anyone reintroduces a second parser into _security.py."""

    BASE = "https://3.basecampapi.com"

    @pytest.mark.parametrize("url", ADVERSARIAL_URLS)
    def test_guard_decides_with_transport_parser(self, url):
        try:
            dialed = httpx.URL(url).host.lower()
        except httpx.InvalidURL:
            dialed = None
        if is_localhost(url):
            assert dialed is not None, f"is_localhost blessed unparseable {url!r}"
            assert dialed in ("localhost", "127.0.0.1", "::1") or dialed.endswith(".localhost"), (
                f"is_localhost blessed {url!r} but the transport dials {dialed!r}"
            )
        if same_origin(url, self.BASE):
            assert dialed == httpx.URL(self.BASE).host.lower(), (
                f"same_origin blessed {url!r} against {self.BASE} but the transport dials {dialed!r}"
            )


class TestTruncate:
    def test_within_limit(self):
        assert truncate("hello", 10) == "hello"

    def test_over_limit(self):
        result = truncate("a" * 100, 10)
        assert len(result.encode()) <= 10
        assert result.endswith("...")

    def test_none_returns_empty(self):
        assert truncate(None) == ""

    def test_exact_limit(self):
        assert truncate("hello", 5) == "hello"

    def test_tiny_max_bytes(self):
        result = truncate("hello", 2)
        assert len(result.encode()) <= 2


class TestRedactHeaders:
    def test_authorization_redacted(self):
        headers = {"Authorization": "Bearer secret", "Content-Type": "application/json"}
        result = redact_headers(headers)
        assert result["Authorization"] == "[REDACTED]"
        assert result["Content-Type"] == "application/json"

    def test_cookie_redacted(self):
        headers = {"Cookie": "session=abc"}
        result = redact_headers(headers)
        assert result["Cookie"] == "[REDACTED]"

    def test_non_sensitive_preserved(self):
        headers = {"X-Custom": "value", "Accept": "text/html"}
        result = redact_headers(headers)
        assert result == headers


class TestCheckBodySize:
    def test_within_limit(self):
        check_body_size(b"small", max_bytes=100)  # should not raise

    def test_over_limit_raises(self):
        with pytest.raises(ApiError, match="body too large"):
            check_body_size(b"x" * 200, max_bytes=100)

    def test_none_body_ok(self):
        check_body_size(None, max_bytes=10)  # should not raise

    def test_string_body_checked(self):
        with pytest.raises(ApiError, match="body too large"):
            check_body_size("x" * 200, max_bytes=100)
