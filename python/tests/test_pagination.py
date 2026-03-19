from __future__ import annotations

from basecamp._pagination import ListMeta, ListResult, parse_next_link, parse_total_count


class TestListResult:
    def test_len(self):
        result = ListResult([1, 2, 3])
        assert len(result) == 3

    def test_index(self):
        result = ListResult(["a", "b", "c"])
        assert result[0] == "a"
        assert result[2] == "c"

    def test_iteration(self):
        result = ListResult([10, 20, 30])
        assert list(result) == [10, 20, 30]

    def test_empty(self):
        result = ListResult([])
        assert len(result) == 0

    def test_is_list(self):
        result = ListResult([1])
        assert isinstance(result, list)

    def test_default_meta(self):
        result = ListResult([])
        assert result.meta.total_count == 0
        assert result.meta.truncated is False

    def test_custom_meta(self):
        meta = ListMeta(total_count=42, truncated=True)
        result = ListResult([1, 2], meta=meta)
        assert result.meta.total_count == 42
        assert result.meta.truncated is True

    def test_repr(self):
        result = ListResult([1, 2])
        r = repr(result)
        assert "ListResult" in r
        assert "[1, 2]" in r


class TestParseNextLink:
    def test_standard_link_header(self):
        header = '<https://api.example.com/page2>; rel="next"'
        assert parse_next_link(header) == "https://api.example.com/page2"

    def test_multiple_rels(self):
        header = '<https://api.example.com/first>; rel="first", <https://api.example.com/page2>; rel="next"'
        assert parse_next_link(header) == "https://api.example.com/page2"

    def test_no_next_rel(self):
        header = '<https://api.example.com/prev>; rel="prev"'
        assert parse_next_link(header) is None

    def test_none_header(self):
        assert parse_next_link(None) is None

    def test_empty_header(self):
        assert parse_next_link("") is None


class TestParseTotalCount:
    def test_present(self):
        assert parse_total_count({"X-Total-Count": "42"}) == 42

    def test_lowercase(self):
        assert parse_total_count({"x-total-count": "10"}) == 10

    def test_missing(self):
        assert parse_total_count({}) == 0

    def test_non_numeric(self):
        assert parse_total_count({"X-Total-Count": "abc"}) == 0
