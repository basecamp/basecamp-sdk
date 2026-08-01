"""Bounds and every-gap contract for the delayBetweenRequests assertion.

Run: `uv run pytest test_delay_gaps.py`

Each case names a behavior that regressed on #563/#568: an assertion that
looked like it covered a timing gap and did not.
"""
from __future__ import annotations

import pytest

from runner import check_delay_gaps


def test_omitted_index_catches_a_later_failing_gap():
    # Three runners measured gap 0 and stopped, so a second backoff that never
    # happened passed unnoticed.
    failure = check_delay_gaps([1000.0, 5.0], 500, None)
    assert failure is not None
    assert "at gap 1" in failure


def test_omitted_index_passes_when_every_gap_clears_the_minimum():
    assert check_delay_gaps([1000.0, 2000.0, 800.0], 500, None) is None


def test_omitted_index_fails_when_there_are_no_gaps_at_all():
    # An "every gap" rule with no gaps left must not wave the run through: a
    # fully dropped retry lands exactly here.
    failure = check_delay_gaps([], 500, None)
    assert failure == "Expected a delay between requests, but only 1 request(s) were made"


def test_named_gap_fails_when_the_run_never_produced_it():
    failure = check_delay_gaps([1000.0], 500, 1)
    assert failure == "Expected a delay at gap 1, but only 2 request(s) were made"


def test_named_gap_fails_on_a_single_request_run():
    failure = check_delay_gaps([], 500, 0)
    assert failure == "Expected a delay at gap 0, but only 1 request(s) were made"


@pytest.mark.parametrize("index", [-1, -3])
def test_negative_gap_index_is_rejected(index):
    # Rejected categorically, not wrapped to the end the way headerPresent's
    # index is: there is no sensible "last gap" when the point is to name one.
    failure = check_delay_gaps([1000.0, 2000.0], 500, index)
    assert failure == f"delayBetweenRequests gap index must be non-negative, got {index}"


def test_enormous_gap_index_fails_rather_than_raising():
    failure = check_delay_gaps([1000.0, 2000.0], 500, 2**63 - 1)
    assert failure is not None
    assert "Expected a delay at gap" in failure


def test_named_gap_passes_when_it_clears_the_minimum():
    assert check_delay_gaps([5.0, 2000.0], 500, 1) is None


def test_named_gap_fails_when_it_is_below_the_minimum():
    failure = check_delay_gaps([2000.0, 5.0], 500, 1)
    assert failure is not None
    assert "at gap 1" in failure


def test_zero_minimum_still_asserts_that_the_gap_exists():
    # `min: 0` is falsy in Python, so a truthiness gate would skip the whole
    # assertion and pass a run with no gaps at all. The minimum is trivially
    # met; the EXISTENCE requirement is not.
    for missing in (0, None):
        assert check_delay_gaps([], missing, None) == (
            "Expected a delay between requests, but only 1 request(s) were made"
        )
        assert check_delay_gaps([], missing, 0) == (
            "Expected a delay at gap 0, but only 1 request(s) were made"
        )
        assert check_delay_gaps([5.0], missing, None) is None
