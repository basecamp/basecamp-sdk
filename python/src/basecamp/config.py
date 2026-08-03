from __future__ import annotations

import math
import os
from dataclasses import dataclass

from basecamp import _security

DEFAULT_BASE_URL = "https://3.basecampapi.com"
DEFAULT_TIMEOUT = 30.0
DEFAULT_MAX_RETRIES = 3
DEFAULT_BASE_DELAY = 1.0
DEFAULT_MAX_JITTER = 0.1
DEFAULT_MAX_PAGES = 10_000

# Ceiling on the backoff term (SPEC section 7, "Backoff Ceiling"), in seconds.
# Jitter is added after the clamp, so the longest single backoff sleep is this
# plus max_jitter.
MAX_BACKOFF_DELAY = 30.0

# Hard ceiling on the exponent, past which the backoff term saturates without
# being computed. It exists only to keep ``2 ** exponent`` inside the float
# range — Python integers are unbounded, so ``2 ** 10_000`` is a real
# three-kilobyte integer that raises OverflowError on conversion, not a wrap.
#
# 1023 is the largest exponent whose power is a finite float. It is deliberately
# NOT the operative bound: _saturating_exponent below derives the real one from
# the configured base, so this only binds for a base below ~1e-304 seconds,
# which is not a duration.
_MAX_BACKOFF_EXPONENT = 1023


def _saturating_exponent(base_delay: float) -> int:
    """Smallest exponent ``e`` with ``base_delay * 2**e >= MAX_BACKOFF_DELAY``.

    Derived from the *configured* base rather than assumed. A fixed exponent cap
    plus a trailing ``min(..., MAX_BACKOFF_DELAY)`` looks equivalent and is not:
    for a small enough base the capped product never reaches the ceiling, so the
    delay plateaus below it forever. At ``base_delay=1e-30`` a cap of 64 pins
    every attempt from 65 on at ~1.84e-11s — a tight retry loop, which is the
    failure SPEC section 7's ceiling exists to prevent, not an instance of it.
    """
    ratio = MAX_BACKOFF_DELAY / base_delay
    if not math.isfinite(ratio):
        # base_delay is denormal-small; no finite power of two gets there.
        return _MAX_BACKOFF_EXPONENT
    return min(math.ceil(math.log2(ratio)), _MAX_BACKOFF_EXPONENT)


def saturating_backoff(base_delay: float, attempt: int) -> float:
    """Exponential backoff for a 1-based attempt, saturating at MAX_BACKOFF_DELAY.

    The clamp is load-bearing rather than defensive. Python's ``**`` does not
    overflow — it promotes — so ``base_delay * (2 ** (attempt - 1))`` for a long
    failure streak either raises ``OverflowError`` converting an arbitrary-
    precision integer to a float, or hands ``time.sleep`` a delay measured in
    geological time. Neither is a retry.

    The exponent is compared against the point where the term reaches the
    ceiling *before* the power is evaluated, so no intermediate leaves the float
    range and the term saturates AT the ceiling for every positive base — the
    same contract Go, Kotlin and Swift get from comparing their multiplier
    against ``MAX_BACKOFF_DELAY / base`` before multiplying.
    """
    if base_delay <= 0:
        return 0.0
    exponent = max(attempt - 1, 0)
    if exponent >= _saturating_exponent(base_delay):
        return MAX_BACKOFF_DELAY
    return min(base_delay * (2**exponent), MAX_BACKOFF_DELAY)


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
        # bool is an int subclass; exclude it so True/False don't masquerade as a
        # retry count. A non-integer max_retries would make the total-attempt
        # bound fractional (e.g. 0.5 → zero requests), so reject it here.
        if isinstance(self.max_retries, bool) or not isinstance(self.max_retries, int):
            raise ValueError("max_retries must be an integer")
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
