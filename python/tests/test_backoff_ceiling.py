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


# Bases spanning the range Config accepts. 1e-30 is the case a fixed exponent
# cap of 64 gets wrong; the denormal-adjacent ones prove the derived bound does
# not simply move the plateau further out.
TINY_BASE_DELAYS = [1e-3, 1e-9, 1e-30, 1e-100, 1e-300, 5e-324]


@pytest.mark.parametrize("base_delay", TINY_BASE_DELAYS)
def test_saturating_backoff_reaches_the_ceiling_for_any_positive_base(base_delay: float) -> None:
    """The term saturates AT the ceiling, whatever the base — no plateau below it.

    A fixed exponent cap plus a trailing ``min(..., MAX_BACKOFF_DELAY)`` bounds
    the intermediate but not the outcome. With a cap of 64 and
    ``base_delay=1e-30``, attempt 65 and every attempt after it returned
    ~1.84e-11s forever: a tight retry loop against a server already answering
    429/503, which is precisely what SPEC section 7 requirement 1 forbids.

    ``Config`` accepts these bases — ``__post_init__`` validates only
    ``base_delay >= 0`` — so the bound has to come from the base, not a constant.
    """
    Config(base_delay=base_delay)  # the configuration under test is a legal one

    for attempt in (1_100, 5_000, 2**31):
        assert saturating_backoff(base_delay, attempt) == MAX_BACKOFF_DELAY, (
            f"base_delay={base_delay!r} attempt={attempt} plateaued below the ceiling"
        )


@pytest.mark.parametrize("base_delay", TINY_BASE_DELAYS)
def test_saturating_backoff_is_monotonic_up_to_the_ceiling(base_delay: float) -> None:
    """It grows to the ceiling rather than jumping — SPEC section 7's "and then stops".

    Sampling every attempt from 1 to 1100 covers the whole exponent range any
    positive double can need, so a formula that stalled anywhere in the middle
    (the fixed-cap plateau) is caught wherever it stalls.
    """
    previous = 0.0
    for attempt in range(1, 1_101):
        delay = saturating_backoff(base_delay, attempt)
        assert delay >= previous, f"base_delay={base_delay!r} went backwards at attempt {attempt}"
        assert delay <= MAX_BACKOFF_DELAY, f"base_delay={base_delay!r} exceeded the ceiling at {attempt}"
        previous = delay

    assert previous == MAX_BACKOFF_DELAY


def test_saturating_backoff_tracks_the_term_for_a_denormal_adjacent_base() -> None:
    """The last attempts before the ceiling are the specified term, not the ceiling.

    Monotonicity and eventual saturation both hold for a formula that saturates
    EARLY, so neither catches this. ``MAX_BACKOFF_DELAY / 1e-307`` overflows to
    infinity, and the fixed-1023 fallback that used to backstop it returned 30.0
    for attempt 1024 when the specified term is ~8.99s — a sleep more than three
    times longer than the formula asks for, with the numeric backstop rather than
    the ceiling deciding it.
    """
    base_delay = 1e-307

    # Exact: these are the products the exponential term is defined to produce.
    assert saturating_backoff(base_delay, 1_024) == 8.988465674311579
    assert saturating_backoff(base_delay, 1_025) == 17.976931348623157
    # And only then does it reach the ceiling.
    assert saturating_backoff(base_delay, 1_026) == MAX_BACKOFF_DELAY
    assert saturating_backoff(base_delay, 2**31) == MAX_BACKOFF_DELAY
