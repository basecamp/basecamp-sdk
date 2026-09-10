"""JSON-level equality for the requestBody assertion (#854).

Python's native comparison says ``False == 0`` and ``True == 1``, so a body that
sent ``0`` where the fixture says ``false`` passed the path-less whole-body form
and the keyed form alike, while every other runner rejected the wire-type
mismatch. ``_json_equal`` is what both forms compare with now.
"""
from __future__ import annotations

from runner import _json_equal


def test_bool_and_int_are_distinct():
    assert not _json_equal(False, 0)
    assert not _json_equal(True, 1)
    assert not _json_equal({"highlighted": False}, {"highlighted": 0})
    assert _json_equal({"highlighted": False}, {"highlighted": False})


def test_containers_compare_recursively_and_exactly():
    assert _json_equal({"a": [1, {"b": "c"}]}, {"a": [1, {"b": "c"}]})
    assert not _json_equal({"a": 1}, {"a": 1, "b": 2})
    assert not _json_equal([1, 2], [1, 2, 3])
    assert not _json_equal({"a": 1}, [1])


def test_numbers_compare_numerically_and_strings_stay_strings():
    assert _json_equal(1, 1.0)
    assert not _json_equal("42", 42)
    assert _json_equal(None, None)
