"""SPEC section 7 "Backoff Ceiling" (#577).

Python's ``**`` promotes rather than overflowing, so ``base_delay * (2 ** (attempt - 1))``
does not wrap the way the compiled SDKs do — it builds an arbitrary-precision
integer. The two failure shapes that produces are both worse than a long sleep:
``OverflowError`` out of the float conversion, aborting the retry with an
exception the caller never sees documented, and, before it gets that big, a
``time.sleep`` measured in millennia.

Both the sync and async transports carry their own copy of ``_calculate_delay``,
so both are exercised here — fixing one and not the other is the shape of bug
this file exists to prevent.
"""

from __future__ import annotations

import pytest

from basecamp._async_http import AsyncHttpClient
from basecamp._http import HttpClient
from basecamp.config import MAX_BACKOFF_DELAY, Config, saturating_backoff

# With the 1.0s default base, attempt 6 is the first whose unclamped term (32s)
# exceeds the 30s ceiling, so every attempt from there on must sit at the cap.
SATURATED_ATTEMPTS = [6, 10, 64, 128, 1024, 10_000, 2**31]


def _clients() -> list[HttpClient | AsyncHttpClient]:
    """A sync and an async client sharing one default Config.

    Built via ``__new__`` because ``_calculate_delay`` reads only
    ``self._config``; constructing real clients would open an httpx pool for a
    pure arithmetic assertion.
    """
    config = Config()
    clients: list[HttpClient | AsyncHttpClient] = []
    for cls in (HttpClient, AsyncHttpClient):
        client = cls.__new__(cls)
        client._config = config
        clients.append(client)
    return clients


@pytest.mark.parametrize("attempt", SATURATED_ATTEMPTS)
def test_calculate_delay_saturates(attempt: int) -> None:
    """Every transport's computed delay lands within the ceiling plus jitter.

    The bound is two-sided: once the exponential term passes the ceiling the
    delay must SIT at the ceiling, so a formula that collapsed to zero would
    fail here too rather than sneaking past a one-sided check.
    """
    config = Config()
    for client in _clients():
        delay = client._calculate_delay(attempt)  # type: ignore[attr-defined]
        assert MAX_BACKOFF_DELAY <= delay <= MAX_BACKOFF_DELAY + config.max_jitter, (
            f"{type(client).__name__}: attempt {attempt} produced {delay}s"
        )


def test_calculate_delay_unchanged_below_the_ceiling() -> None:
    """The delays a shipped configuration actually reaches are untouched."""
    config = Config()
    for client in _clients():
        for attempt, want in ((1, 1.0), (2, 2.0), (3, 4.0)):
            delay = client._calculate_delay(attempt)  # type: ignore[attr-defined]
            assert want <= delay <= want + config.max_jitter, (
                f"{type(client).__name__}: attempt {attempt} produced {delay}s, want ~{want}s"
            )


def test_retry_after_is_exempt_from_the_ceiling() -> None:
    """SPEC section 7 requirement 4: the ceiling governs the local formula only."""
    for client in _clients():
        assert client._calculate_delay(1, 120) == 120.0  # type: ignore[attr-defined]


def test_saturating_backoff_edges() -> None:
    # SPEC section 7 requirement 3: the ceiling binds base_delay itself.
    assert saturating_backoff(600.0, 1) == MAX_BACKOFF_DELAY
    # A zero base delay stays at zero rather than saturating.
    assert saturating_backoff(0.0, 10_000) == 0.0
    # Below the ceiling the exponential term is exact.
    assert saturating_backoff(1.0, 3) == 4.0
