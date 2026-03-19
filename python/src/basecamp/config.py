from __future__ import annotations

import os
from dataclasses import dataclass

from basecamp import _security

DEFAULT_BASE_URL = "https://3.basecampapi.com"
DEFAULT_TIMEOUT = 30.0
DEFAULT_MAX_RETRIES = 3
DEFAULT_BASE_DELAY = 1.0
DEFAULT_MAX_JITTER = 0.1
DEFAULT_MAX_PAGES = 10_000


@dataclass(frozen=True)
class Config:
    """Configuration for the Basecamp API client."""

    base_url: str = DEFAULT_BASE_URL
    timeout: float = DEFAULT_TIMEOUT
    max_retries: int = DEFAULT_MAX_RETRIES
    base_delay: float = DEFAULT_BASE_DELAY
    max_jitter: float = DEFAULT_MAX_JITTER
    max_pages: int = DEFAULT_MAX_PAGES

    def __post_init__(self) -> None:
        # Normalize trailing slash
        object.__setattr__(self, "base_url", self.base_url.rstrip("/"))

        # HTTPS enforcement (skip for default URL and localhost)
        if self.base_url != DEFAULT_BASE_URL.rstrip("/") and not _security.is_localhost(self.base_url):
            _security.require_https(self.base_url, "base URL")

        # Validation
        if self.timeout <= 0:
            raise ValueError("timeout must be positive")
        if self.max_retries < 0:
            raise ValueError("max_retries must be non-negative")
        if self.base_delay < 0:
            raise ValueError("base_delay must be non-negative")
        if self.max_jitter < 0:
            raise ValueError("max_jitter must be non-negative")
        if self.max_pages <= 0:
            raise ValueError("max_pages must be positive")

    @classmethod
    def from_env(cls) -> Config:
        """Create a Config from environment variables."""
        return cls(
            base_url=os.environ.get("BASECAMP_BASE_URL", DEFAULT_BASE_URL),
            timeout=float(os.environ.get("BASECAMP_TIMEOUT", DEFAULT_TIMEOUT)),
            max_retries=int(os.environ.get("BASECAMP_MAX_RETRIES", DEFAULT_MAX_RETRIES)),
        )
