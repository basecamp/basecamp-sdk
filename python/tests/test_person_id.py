"""The person-id grammar, pinned at every site in the SDK that reads one.

One corpus (:mod:`tests.person_id_corpus`, 74 measured Go verdicts), every site.
The sites read the same string and do different things with the same outcome,
which is why the table carries the OUTCOME rather than a per-site expectation.
"""

from __future__ import annotations

import base64
import json

import pytest

from basecamp._person_id import Refusal, coerce_person_id, parse_int64
from basecamp.errors import ApiError
from basecamp.generated.services import _async_base, _base
from basecamp.mentions import person_id_from_sgid
from basecamp.services._campfire_index import _decoded_flexible_int64
from tests.person_id_corpus import PERSON_ID_CORPUS

CORPUS_IDS = [repr(raw) for raw, _, _ in PERSON_ID_CORPUS]


def test_the_corpus_is_the_whole_measured_table():
    # A row silently dropped from the table is a rule silently unpinned, and
    # the rows that discriminate are the ones a reader is tempted to prune.
    assert len(PERSON_ID_CORPUS) == 74
    assert len({raw for raw, _, _ in PERSON_ID_CORPUS}) == 74


class TestTheScan:
    """`parse_int64` itself: Go's `strconv.ParseInt(s, 10, 64)`."""

    @pytest.mark.parametrize(("raw", "kind", "value"), PERSON_ID_CORPUS, ids=CORPUS_IDS)
    def test_a_row_parses_as_go_parses_it(self, raw, kind, value):
        parsed = parse_int64(raw)
        if kind == "value":
            assert parsed == value
        elif kind == "label":
            assert parsed is Refusal.SYNTAX
        else:
            assert parsed is Refusal.RANGE

    @pytest.mark.parametrize(
        ("raw", "refusal"),
        [
            # The pair the whole hand-rolled loop exists for: one digit apart,
            # opposite refusals, because the magnitude is checked INSIDE the
            # scan and against uint64. Getting this backwards is silent -- the
            # syntax refusal reads as the system actor rather than failing.
            ("18446744073709551615x", Refusal.SYNTAX),
            ("18446744073709551616x", Refusal.RANGE),
        ],
    )
    def test_the_scan_order_decides_which_refusal(self, raw, refusal):
        assert parse_int64(raw) is refusal

    def test_python_int_would_have_accepted_the_rows_go_refuses(self):
        # Not a tautology: it is the measurement of WHY this function exists.
        # Every one of these is `int()` saying yes where Go says no, and every
        # one of them either mints an id BC3 never wrote or collapses a real
        # person onto the system-actor 0.
        accepted_by_int = []
        for raw, kind, _ in PERSON_ID_CORPUS:
            if kind == "value":
                continue
            try:
                int(raw)
            except ValueError:
                continue
            accepted_by_int.append(raw)
        assert set(accepted_by_int) == {
            # whitespace: `int()` strips it, Go trims nothing
            " 7",
            "7 ",
            " 7 ",
            "\n7",
            "7\n",
            "\t7",
            "7\t",
            # PEP 515 underscores: a base-0 feature in Go, never a base-10 one
            "1_0",
            "1_2",
            # Unicode decimal digits: Go tests the ASCII range on bytes
            "１２３",
            "７",
            "٠١٢",
            "٠",
            "৭",
            "۷",
            "৭7",
            "7৭",
            # arbitrary precision: the wire id is an int64
            "9223372036854775808",
            "9223372036854775809",
            "-9223372036854775809",
            "+9223372036854775808",
            "18446744073709551614",
            "18446744073709551615",
            "18446744073709551616",
            "99999999999999999999999",
            "00000000000000000000018446744073709551616",
        }


class TestThePreDecodeNormalizer:
    """`_normalize_person_ids`, which every response body walks."""

    @staticmethod
    def _normalized(normalize, raw):
        obj = {"personable_type": "User", "id": raw}
        normalize(obj)
        return obj

    @pytest.mark.parametrize("normalize", [_base._normalize_person_ids, _async_base._normalize_person_ids])
    @pytest.mark.parametrize(("raw", "kind", "value"), PERSON_ID_CORPUS, ids=CORPUS_IDS)
    def test_a_row_normalizes_as_go_normalizes_it(self, normalize, raw, kind, value):
        normalized = self._normalized(normalize, raw)
        if kind == "value":
            # CONVERTED, and with no label: a real person, named by number.
            assert normalized == {"personable_type": "User", "id": value}
        elif kind == "label":
            # The system-actor sentinel, with the raw string kept alongside.
            assert normalized == {"personable_type": "User", "id": 0, "system_label": raw}
        else:
            # Left verbatim, so the readers with a struct behind them refuse it
            # rather than a Python bigint standing in for a wire int64.
            assert normalized == {"personable_type": "User", "id": raw}

    def test_the_sync_and_async_walks_share_one_rule(self):
        # Not "behave the same today": the SAME function object. They were two
        # copies of the scan, and two copies drift.
        assert _base.coerce_person_id is _async_base.coerce_person_id
        assert _base.coerce_person_id is coerce_person_id

    def test_an_id_that_is_already_a_number_is_left_alone(self):
        obj = {"personable_type": "User", "id": 1049715915}
        coerce_person_id(obj)
        assert obj == {"personable_type": "User", "id": 1049715915}

    def test_an_object_with_no_id_is_left_alone(self):
        obj = {"personable_type": "LocalPerson", "name": "Basecamp"}
        coerce_person_id(obj)
        assert obj == {"personable_type": "LocalPerson", "name": "Basecamp"}


class TestTheFlexibleReader:
    """`_decoded_flexible_int64`, the port of `types.FlexibleInt64`."""

    @pytest.mark.parametrize(("raw", "kind", "value"), PERSON_ID_CORPUS, ids=CORPUS_IDS)
    def test_a_row_reads_as_go_reads_it(self, raw, kind, value):
        if kind == "refuse":
            with pytest.raises(ApiError, match="int64"):
                _decoded_flexible_int64(raw, "the id")
        else:
            # A syntax refusal is the system actor's 0 here; there is no
            # `system_label` at a reader, because Go's int64 field has nowhere
            # to put one.
            assert _decoded_flexible_int64(raw, "the id") == (value if kind == "value" else 0)


class TestTheGidRuleIsADifferentRule:
    """Rule A: `person_id_from_sgid`, which must NOT be unified with the above.

    The reference deliberately carries two rules. The gid path walks the bytes
    and refuses anything outside 0-9 before parsing
    (`go/pkg/basecamp/mentions.go:252-256`), then refuses `id <= 0` (`:258`);
    the flexible path is `strconv.ParseInt(s, 10, 64)` whole
    (`go/pkg/types/flexible_int64.go:34`). Hoisting either into the other breaks
    the reference at one of the two sites -- the `+77` defect PR #886 closed is
    what "fixing" the gid path to match looks like.
    """

    @staticmethod
    def _sgid(gid: str) -> str:
        envelope = {"_rails": {"data": gid, "pur": "attachable"}}
        payload = base64.urlsafe_b64encode(json.dumps(envelope).encode()).decode().rstrip("=")
        return f"{payload}--0123456789abcdef"

    @pytest.mark.parametrize(
        ("raw", "flexible", "gid"),
        [
            # The sign the gid pre-walk refuses and ParseInt accepts.
            ("+7", 7, None),
            ("+007", 7, None),
            # The non-positive ids the gid path refuses outright, where a
            # person id read off the wire keeps them: 0 IS the system actor.
            ("0", 0, None),
            ("-7", -7, None),
            # Where they agree: plain digits, leading zeros and all.
            ("7", 7, 7),
            ("007", 7, 7),
            ("9223372036854775807", 9223372036854775807, 9223372036854775807),
            # And where they agree to refuse, by different routes.
            ("basecamp", 0, None),
            ("9223372036854775808", None, None),
        ],
    )
    def test_the_two_rules_answer_the_same_string_differently(self, raw, flexible, gid):
        if flexible is None:
            with pytest.raises(ApiError, match="int64"):
                _decoded_flexible_int64(raw, "the id")
        else:
            assert _decoded_flexible_int64(raw, "the id") == flexible
        assert person_id_from_sgid(self._sgid(f"gid://bc3/Person/{raw}")) == gid
