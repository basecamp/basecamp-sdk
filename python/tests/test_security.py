from __future__ import annotations

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
        ],
    )
    def test_non_localhost_false(self, url):
        assert is_localhost(url) is False


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
