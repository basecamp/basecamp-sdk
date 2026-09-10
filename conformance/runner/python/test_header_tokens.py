"""The `{{httpdate+Ns}}` header token (SPEC section 19, conformance/schema.json).

Run: `uv run pytest test_header_tokens.py`

A static fixture has no clock, so the positive half of SPEC section 6's
HTTP-date branch was unpinnable until this token (#780). These cases pin the
resolver's arithmetic against a frozen instant so the fixture's one-sided
timing floor rests on a deterministic contract.
"""
from __future__ import annotations

import pytest

from runner import resolve_header_value

# A quarter-second into 10:18:14 UTC, so floor and round-up differ.
NOW = 1623233894.25


@pytest.mark.parametrize("value", ["", "2", "Wed, 09 Jun 2021 10:18:14 GMT", "application/json", "{not a token}"])
def test_plain_values_pass_through(value: str) -> None:
    assert resolve_header_value(value, NOW) == value


@pytest.mark.parametrize(
    "token,expected",
    [
        ("{{httpdate+2s}}", "Wed, 09 Jun 2021 10:18:17 GMT"),
        ("{{httpdate+0s}}", "Wed, 09 Jun 2021 10:18:15 GMT"),
        ("{{httpdate+10s}}", "Wed, 09 Jun 2021 10:18:25 GMT"),
    ],
)
def test_httpdate_resolves_to_the_whole_second_past_n(token: str, expected: str) -> None:
    assert resolve_header_value(token, NOW) == expected


@pytest.mark.parametrize("value", ["{{httpdate}}", "{{httpdate+2}}", "{{httpdate-2s}}", "{{now}}", "{{}}"])
def test_unknown_tokens_are_errors_not_literals(value: str) -> None:
    with pytest.raises(ValueError, match="unrecognised header token"):
        resolve_header_value(value, NOW)
