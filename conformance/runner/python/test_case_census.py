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
import os
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


def test_census_reports_an_unreadable_subtree(tmp_path: Path):
    # `Path.rglob` suppressed the scan error, so the subtree was simply omitted
    # — and the runner's non-recursive glob omits it too, leaving both sides of
    # the census agreeing over cases neither counted. Root reads through a 0o000
    # directory, so under root the assertion is that the cases are still
    # counted; either way they must never be silently dropped.
    write_fixture(tmp_path / "cases.json", json.dumps(CENSUS_FIXTURE))
    write_fixture(tmp_path / "locked" / "nested.json", json.dumps(CENSUS_FIXTURE))
    locked = tmp_path / "locked"
    locked.chmod(0o000)
    try:
        if os.geteuid() == 0:
            assert count_non_live_cases(tmp_path) == 4
        else:
            with pytest.raises(RuntimeError):
                count_non_live_cases(tmp_path)
    finally:
        locked.chmod(0o755)


def test_census_rejects_an_empty_tree(tmp_path: Path):
    # A census that counted nothing certifies nothing: zero on both sides is the
    # shape a broken walk takes.
    with pytest.raises(RuntimeError):
        count_non_live_cases(tmp_path)


def test_census_rejects_an_emptied_fixture(tmp_path: Path):
    # The one truncation both sides read identically: the runner registers
    # nothing from the file and the census would expect nothing, so the totals
    # fall together and no mismatch appears. Counting it as zero is what would
    # make the whole-file guarantee a lie, so the census refuses it instead.
    write_fixture(tmp_path / "cases.json", json.dumps(CENSUS_FIXTURE))
    write_fixture(tmp_path / "emptied.json", "[]")

    with pytest.raises(RuntimeError):
        count_non_live_cases(tmp_path)


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


def test_is_mock_mode_does_not_default_on_falsiness():
    # `(mode or "mock")` read this as an absent key and ran it, where the four
    # null-coalescing runners refuse it. The census counts "" as non-live either
    # way, so the divergence would have stayed green in Python alone.
    assert not is_mock_mode("")
