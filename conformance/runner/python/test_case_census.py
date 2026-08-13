"""Case-census contract (#602).

The check is green on the real fixture tree by construction, so a live run only
ever proves it can say yes. These cases run it against a SYNTHETIC fixture set
and prove it can say no — the ``mode: "moc"`` case in particular, which every
runner's "mock unless told otherwise" filter drops with nothing printed. That
divergence is asserted end-to-end here: the census and the run loop's own
predicate (``is_mock_mode``, shared with the load path) disagree by one, and
``case_count_failure`` reports it.

Run: ``uv run pytest test_case_census.py``
"""
from __future__ import annotations

import json
from pathlib import Path

import pytest

from runner import case_count_failure, count_non_live_cases, is_mock_mode

# One case of each kind: a plain mock case (no ``mode`` at all, the common
# spelling), a live case the runners are meant to drop, and a typo'd mode that
# nothing recognizes.
CENSUS_FIXTURE = [
    {"name": "plain", "operation": "GetProject"},
    {"name": "live one", "operation": "GetProject", "mode": "live"},
    {"name": "typo", "operation": "GetProject", "mode": "moc"},
]


def write_fixture(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)


def test_census_counts_every_case_that_is_not_explicitly_live(tmp_path: Path):
    write_fixture(tmp_path / "cases.json", json.dumps(CENSUS_FIXTURE))

    assert count_non_live_cases(tmp_path) == 2


def test_a_typoed_mode_makes_the_count_check_fail(tmp_path: Path):
    # The regression this whole check exists for. The runner's own filter keeps
    # one case; the census counts two; the difference is the case executed by
    # nothing.
    write_fixture(tmp_path / "cases.json", json.dumps(CENSUS_FIXTURE))

    ran = len([t for t in CENSUS_FIXTURE if is_mock_mode(t.get("mode"))])
    assert ran == 1, "the run loop should keep only the plain case"

    failure = case_count_failure(ran, count_non_live_cases(tmp_path))

    assert failure is not None, "a case no runner recognizes must fail the count check"
    assert "1 executed by nothing" in failure


def test_census_finds_fixtures_nested_below_the_tests_directory(tmp_path: Path):
    # No runner globs recursively, so a case parked one directory down is run by
    # nothing. The census walks, which is what makes that visible.
    write_fixture(tmp_path / "nested" / "cases.json", json.dumps(CENSUS_FIXTURE))

    assert count_non_live_cases(tmp_path) == 2


def test_census_rejects_a_fixture_that_does_not_parse(tmp_path: Path):
    write_fixture(tmp_path / "broken.json", '[{"name": "truncated"')

    with pytest.raises(RuntimeError):
        count_non_live_cases(tmp_path)


def test_census_rejects_a_fixture_that_is_not_an_array(tmp_path: Path):
    write_fixture(tmp_path / "object.json", '{"name": "not a list"}')

    with pytest.raises(RuntimeError):
        count_non_live_cases(tmp_path)


def test_census_rejects_an_empty_tree(tmp_path: Path):
    # A census that counted nothing certifies nothing: zero on both sides is the
    # shape a broken walk takes.
    with pytest.raises(RuntimeError):
        count_non_live_cases(tmp_path)


def test_census_accepts_a_truncated_fixture_as_zero_cases(tmp_path: Path):
    # ``[]`` parses, so the census counts zero for it — and the runner counts
    # zero too. The mismatch this produces is against the OTHER files' cases,
    # which is why the count is taken over the whole tree rather than per file.
    write_fixture(tmp_path / "empty.json", "[]")
    write_fixture(tmp_path / "cases.json", json.dumps(CENSUS_FIXTURE))

    assert count_non_live_cases(tmp_path) == 2


def test_case_count_failure_accepts_agreement():
    assert case_count_failure(42, 42) is None


def test_case_count_failure_names_an_over_count():
    failure = case_count_failure(43, 42)

    assert failure is not None
    assert "1 more than the fixtures declare" in failure


def test_is_mock_mode_treats_absence_as_mock():
    assert is_mock_mode(None)
    assert is_mock_mode("mock")
    assert not is_mock_mode("live")
    # The census is what catches this one; the filter must not run it.
    assert not is_mock_mode("moc")
