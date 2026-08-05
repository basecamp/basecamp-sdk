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


class TestParseNextLinkAdversarialInput:
    """The Link header is attacker-influenced, so malformed shapes are a contract.

    ``isSameOrigin`` exists to stop SSRF through a poisoned Link header, which
    makes the parser's behaviour on hostile input part of the threat model. The
    same six cases exist in all six SDKs.
    """

    def test_opening_bracket_with_no_closing_bracket(self):
        assert parse_next_link('<https://api.example.com/page2; rel="next"') is None

    def test_closing_bracket_before_opening_bracket(self):
        header = '>x<https://api.example.com/page2>; rel="next"'
        assert parse_next_link(header) == "https://api.example.com/page2"

    def test_url_containing_raw_closing_bracket_truncates_at_the_first(self):
        # Parity with the old <([^>]+)> spelling: [^>] cannot span a ">".
        header = '<https://api.example.com/page2?q=a>b>; rel="next"'
        assert parse_next_link(header) == "https://api.example.com/page2?q=a"

    def test_multiple_bracket_pairs_in_one_part_take_the_first(self):
        # Parity with the old spelling: leftmost match wins.
        header = '<https://api.example.com/a> <https://api.example.com/b>; rel="next"'
        assert parse_next_link(header) == "https://api.example.com/a"

    def test_empty_bracket_pair_is_skipped_not_returned(self):
        # Parity with the old spelling: [^>]+ requires at least one character,
        # so an empty <> is not a match and the scan moves on. A naive
        # find(">", start + 1) without this check would return "".
        header = '<> <https://api.example.com/page2>; rel="next"'
        assert parse_next_link(header) == "https://api.example.com/page2"

    def test_malformed_part_does_not_abandon_a_later_valid_one(self):
        header = '<malformed; rel="next", <https://api.example.com/page2>; rel="next"'
        assert parse_next_link(header) == "https://api.example.com/page2"

    def test_pathological_header(self):
        # Many "<" start positions with no reachable ">" — the shape that
        # punishes a backtracking regex. The re.search spelling took 5.2s at
        # 32k characters; this is 50k. Asserting behaviour and completion, not
        # elapsed time: the suite already has timing flakiness (#655) and a
        # duration bound would add more.
        many = "<" * 50_000
        assert parse_next_link(f'{many}; rel="next"') is None
        # A ">" present but unreachable defeats the literal-prescan shortcut
        # some regex engines use to bail early.
        assert parse_next_link(f'>{many}; rel="next"') is None


class TestParseTotalCount:
    def test_present(self):
        assert parse_total_count({"X-Total-Count": "42"}) == 42

    def test_lowercase(self):
        assert parse_total_count({"x-total-count": "10"}) == 10

    def test_missing(self):
        assert parse_total_count({}) == 0

    def test_non_numeric(self):
        assert parse_total_count({"X-Total-Count": "abc"}) == 0
