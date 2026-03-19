from __future__ import annotations

import pytest

from basecamp.config import (
    DEFAULT_BASE_URL,
    DEFAULT_MAX_RETRIES,
    DEFAULT_TIMEOUT,
    Config,
)
from basecamp.errors import UsageError


class TestDefaults:
    def test_base_url(self):
        c = Config()
        assert c.base_url == DEFAULT_BASE_URL.rstrip("/")

    def test_timeout(self):
        c = Config()
        assert c.timeout == DEFAULT_TIMEOUT

    def test_max_retries(self):
        c = Config()
        assert c.max_retries == DEFAULT_MAX_RETRIES


class TestFromEnv:
    def test_reads_env_vars(self, monkeypatch):
        monkeypatch.setenv("BASECAMP_BASE_URL", "https://custom.example.com")
        monkeypatch.setenv("BASECAMP_TIMEOUT", "60")
        monkeypatch.setenv("BASECAMP_MAX_RETRIES", "5")
        c = Config.from_env()
        assert c.base_url == "https://custom.example.com"
        assert c.timeout == 60.0
        assert c.max_retries == 5

    def test_defaults_when_env_unset(self, monkeypatch):
        monkeypatch.delenv("BASECAMP_BASE_URL", raising=False)
        monkeypatch.delenv("BASECAMP_TIMEOUT", raising=False)
        monkeypatch.delenv("BASECAMP_MAX_RETRIES", raising=False)
        c = Config.from_env()
        assert c.base_url == DEFAULT_BASE_URL.rstrip("/")
        assert c.timeout == DEFAULT_TIMEOUT
        assert c.max_retries == DEFAULT_MAX_RETRIES


class TestValidation:
    def test_timeout_zero_raises(self):
        with pytest.raises(ValueError, match="timeout must be positive"):
            Config(timeout=0)

    def test_timeout_negative_raises(self):
        with pytest.raises(ValueError, match="timeout must be positive"):
            Config(timeout=-1)

    def test_max_retries_negative_raises(self):
        with pytest.raises(ValueError, match="max_retries must be non-negative"):
            Config(max_retries=-1)

    def test_max_retries_zero_allowed(self):
        c = Config(max_retries=0)
        assert c.max_retries == 0


class TestHttpsEnforcement:
    def test_http_non_localhost_raises(self):
        with pytest.raises(UsageError, match="must use HTTPS"):
            Config(base_url="http://api.example.com")

    def test_localhost_http_allowed(self):
        c = Config(base_url="http://localhost:3000")
        assert c.base_url == "http://localhost:3000"

    def test_https_allowed(self):
        c = Config(base_url="https://custom.example.com")
        assert c.base_url == "https://custom.example.com"


class TestTrailingSlashNormalization:
    def test_trailing_slash_removed(self):
        c = Config(base_url="https://custom.example.com/")
        assert c.base_url == "https://custom.example.com"

    def test_multiple_trailing_slashes_removed(self):
        c = Config(base_url="https://custom.example.com///")
        assert c.base_url == "https://custom.example.com"

    def test_no_trailing_slash_unchanged(self):
        c = Config(base_url="https://custom.example.com")
        assert c.base_url == "https://custom.example.com"
