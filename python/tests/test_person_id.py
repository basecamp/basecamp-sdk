"""The person-id grammar, pinned at every site in the SDK that reads one.

One corpus (:mod:`tests.person_id_corpus`, 74 measured Go verdicts), every site.
The sites read the same string and do different things with the same outcome,
which is why the table carries the OUTCOME rather than a per-site expectation.
"""

from __future__ import annotations

import base64
import json

import httpx
import pytest
import respx

from basecamp import Client
from basecamp._person_id import Refusal, coerce_person_id, normalize_person_ids, parse_int64
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

    def test_the_sync_and_async_walks_are_one_walk(self):
        # Not "behave the same today": the SAME function object. Both the scan
        # and the walk around it were copied into the two base files, and both
        # copies drifted -- the walk by missing a whole pass of the reference.
        assert _base._normalize_person_ids is _async_base._normalize_person_ids
        assert _base._normalize_person_ids is normalize_person_ids

    def test_an_id_that_is_already_a_number_is_left_alone(self):
        obj = {"personable_type": "User", "id": 1049715915}
        coerce_person_id(obj)
        assert obj == {"personable_type": "User", "id": 1049715915}

    def test_an_object_with_no_id_is_left_alone(self):
        obj = {"personable_type": "LocalPerson", "name": "Basecamp"}
        coerce_person_id(obj)
        assert obj == {"personable_type": "LocalPerson", "name": "Basecamp"}


#: Where the walk finds a person, and where the person it found ends up. Go
#: finds one TWO ways -- by `personable_type`, and by structural position: the
#: `creator` object and each `participants` element, at any depth, whether or
#: not they carry that key (`go/pkg/basecamp/normalize.go:83-104`). The three
#: shapes below the first are the ones the `personable_type` pass alone misses,
#: and they were 62/74 divergent until the second pass landed.
_EMBEDDED_SHAPES = {
    "personable_type person": (
        lambda raw: {"id": 42, "creator": {"id": raw, "personable_type": "User"}},
        lambda body: body["unreads"][0]["creator"],
    ),
    "bare creator": (
        lambda raw: {"id": 42, "creator": {"id": raw, "name": "Ann"}},
        lambda body: body["unreads"][0]["creator"],
    ),
    "participants element": (
        lambda raw: {"id": 42, "participants": [{"id": raw, "name": "Ann"}]},
        lambda body: body["unreads"][0]["participants"][0],
    ),
    "nested creator": (
        lambda raw: {"id": 42, "recording": {"comment": {"creator": {"id": raw, "name": "Ann"}}}},
        lambda body: body["unreads"][0]["recording"]["comment"]["creator"],
    ),
}

_READINGS_URL = "https://3.basecampapi.com/12345/my/readings.json"


def _read_notifications(payload: dict) -> dict:
    """One real request through the real service, so the walk is exercised where it runs."""
    from basecamp.generated.services.my_notifications import MyNotificationsService

    respx.get(_READINGS_URL).mock(
        return_value=httpx.Response(
            200,
            json={
                "unreads": [payload],
                "reads": [],
                "memories": [],
                "bubble_ups_count": 0,
                "scheduled_bubble_ups_count": 0,
            },
        )
    )
    client = Client(access_token="test-token")
    try:
        return MyNotificationsService(client.for_account("12345")).get_my_notifications()
    finally:
        client.close()


class TestTheWalkFindsAPersonTwoWays:
    """The second pass: people found by structural position, not by `personable_type`.

    Driven through the request path rather than by calling the walk directly,
    because "does this run on a real response" is half of what was wrong: the
    grammar was right and the coverage was not.
    """

    @respx.mock
    @pytest.mark.parametrize("shape", list(_EMBEDDED_SHAPES), ids=list(_EMBEDDED_SHAPES))
    @pytest.mark.parametrize(("raw", "kind", "value"), PERSON_ID_CORPUS, ids=CORPUS_IDS)
    def test_every_shape_gets_the_same_row_verdict(self, shape, raw, kind, value):
        build, pick = _EMBEDDED_SHAPES[shape]
        person = pick(_read_notifications(build(raw)))
        if kind == "value":
            assert person["id"] == value
            assert "system_label" not in person
        elif kind == "label":
            assert person["id"] == 0
            assert person["system_label"] == raw
        else:
            assert person["id"] == raw
            assert "system_label" not in person

    @respx.mock
    def test_a_creator_that_is_both_kinds_of_person_is_coerced_once(self):
        # The idempotence the one-walk-for-Go's-two rests on. This node is in
        # BOTH passes' sets: Go coerces it twice (a no-op the second time, its
        # id no longer a string), and one walk coerces it once. Same answer, and
        # the `system_label` must not be re-derived from the coerced id.
        person = _read_notifications({"id": 42, "creator": {"id": "basecamp", "personable_type": "LocalPerson"}})
        assert person["unreads"][0]["creator"] == {
            "id": 0,
            "system_label": "basecamp",
            "personable_type": "LocalPerson",
        }

    def test_coercion_is_idempotent_on_every_row(self):
        # Stated directly, because the one-walk argument depends on it and a
        # walk-level test can only reach it by accident.
        for raw, _, _ in PERSON_ID_CORPUS:
            once = {"id": raw}
            coerce_person_id(once)
            twice = dict(once)
            coerce_person_id(twice)
            assert twice == once, raw

    @respx.mock
    @pytest.mark.parametrize(
        "payload",
        [
            # Go's type assertions, mirrored: `.(map[string]any)` on the creator
            # and `.([]any)` on participants. Anything else is not a person and
            # is skipped, not coerced (`normalize.go:85-95`).
            {"id": 42, "creator": "me"},
            {"id": 42, "creator": ["7"]},
            {"id": 42, "creator": 7},
            {"id": 42, "creator": None},
            {"id": 42, "participants": {"id": "7"}},
            {"id": 42, "participants": "everyone"},
            {"id": 42, "participants": ["7", 7, None]},
        ],
    )
    def test_a_creator_or_participants_that_is_not_a_person_shape_is_left_alone(self, payload):
        assert _read_notifications(payload)["unreads"][0] == payload

    @respx.mock
    def test_a_person_key_that_is_not_creator_or_participants_still_needs_personable_type(self):
        # The second pass is keyed on TWO names, not "anything person-shaped".
        # `assignees` is not one of them, and Go does not widen it either, so a
        # string id there stays a string unless the object says what it is.
        body = _read_notifications({"id": 42, "assignees": [{"id": "7", "name": "Ann"}]})
        assert body["unreads"][0]["assignees"][0]["id"] == "7"


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
