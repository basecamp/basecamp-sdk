"""Tests for ``recordings.summarize`` (sync + async).

The conformance fixture ``conformance/tests/recording_summary.json`` pins the
routing matrix, the projection's shape and the request sequence for every routed
type. What it cannot reach lives here: the routing table's own integrity, the
error identities the discovery loop raises, the Campfire cache's lifetime and
refresh floor, the candidate bound, and the promise that hooks see the
constituent reads under their NATIVE names rather than a minted composite one
(SPEC section 18 rule 3).
"""

from __future__ import annotations

import inspect

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import (
    ApiError,
    BucketMismatchError,
    CampfireDiscoveryIncompleteError,
    ForbiddenError,
    NoRecordingTypeError,
    RecordingRoutingError,
    RecordingUnresolvedError,
    UnknownRecordingTypeError,
    UsageError,
)
from basecamp.hooks import BasecampHooks
from basecamp.services import _campfire_index
from basecamp.services._campfire_index import MAX_CAMPFIRE_CANDIDATES, AsyncCampfireIndex, CampfireIndex
from basecamp.services.recordings import _READS, summarizable_event_types, summarizable_recording_types

ACCOUNT = "12345"
BASE = f"https://3.basecampapi.com/{ACCOUNT}"
BUCKET = 2085958499
LINE_ID = 1069479350


def _client(*, now=None, hooks=None) -> Client:
    client = Client(access_token="test-token", hooks=hooks)
    if now is not None:
        # The discovery cache is a lazily-filled slot on the client; seeding it
        # with a controlled clock is how a TTL and a refresh floor become
        # testable at all.
        client._campfire_index = CampfireIndex(now=now)
    return client


def _account(**kwargs):
    return _client(**kwargs).for_account(ACCOUNT)


def _project(*campfire_ids: int) -> dict:
    return {
        "id": BUCKET,
        "name": "The Leto Laptop",
        "dock": [{"id": cid, "name": "chat", "title": "Campfire", "enabled": True} for cid in campfire_ids]
        + [{"id": 999, "name": "message_board", "title": "Message Board", "enabled": True}],
    }


def _campfire(campfire_id: int, bucket_id: int = BUCKET) -> dict:
    return {"id": campfire_id, "type": "Chat::Transcript", "bucket": {"id": bucket_id, "name": "b", "type": "Project"}}


def _line(campfire_id: int, *, line_type: str = "Chat::Lines::Text", content: str = "Hello everyone!") -> dict:
    return {
        "id": LINE_ID,
        "status": "active",
        "type": line_type,
        "title": content,
        "app_url": f"https://3.basecamp.com/{ACCOUNT}/buckets/{BUCKET}/chats/{campfire_id}#__recording_{LINE_ID}",
        "content": content,
        "parent": {"id": campfire_id, "type": "Chat::Transcript"},
        "bucket": {"id": BUCKET, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1049715914, "name": "Victor Cooper"},
        "updated_at": "2022-10-28T15:25:00.000Z",
    }


def _not_found() -> httpx.Response:
    return httpx.Response(404, json={"error": "Record not found"})


class TestRoutingTable:
    def test_every_routed_read_names_a_real_service_method(self):
        # The table is the routing contract, and it addresses the generated
        # services by name. A generated method renamed out from under it would
        # otherwise only surface as an AttributeError on the one type nobody
        # exercised that week.
        account = _account()
        for recording_type, read in _READS.items():
            service = getattr(account, read.service, None)
            assert service is not None, f"{recording_type}: no account.{read.service} accessor"
            method = getattr(service, read.method, None)
            assert callable(method), f"{recording_type}: {read.service} has no {read.method}"

    def test_every_routed_read_takes_its_declared_id_keyword(self):
        account = _account()
        for recording_type, read in _READS.items():
            signature = inspect.signature(getattr(getattr(account, read.service), read.method))
            assert read.param in signature.parameters, (
                f"{recording_type}: {read.service}.{read.method} takes no {read.param}"
            )

    def test_the_documented_type_list_is_the_routed_one(self):
        documented = summarizable_recording_types()
        assert "Chat::Lines::*" in documented
        assert "Comment" in documented and "Kanban::Card" in documented
        assert documented == sorted(documented)
        # A deliberate set, not an exhaustive one: these have id-only reads in
        # the SDK and are still not routed.
        assert "Timesheet::Entry" not in documented
        assert "Gauge::Needle" not in documented

    def test_the_documented_event_subjects_exclude_boost(self):
        assert summarizable_event_types() == [
            "card.*",
            "chat.line.*",
            "comment.*",
            "message.*",
            "todo.*",
        ]


class TestRoutingRefusals:
    @respx.mock
    def test_boost_is_refused_before_any_request(self):
        route = respx.route(host="3.basecampapi.com")
        with pytest.raises(NoRecordingTypeError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="boost.created")
        assert not route.called
        assert raised.value.code == "no_recording_type"
        assert isinstance(raised.value, RecordingRoutingError)

    @respx.mock
    @pytest.mark.parametrize(
        "pointer",
        [
            {"event_type": "widget.created"},
            {"event_type": "comment"},  # a subject with no action is not a feed type
            {"event_type": "comment."},
            {"event_type": ".created"},
            {"recording_type": "Timesheet::Entry"},
            {"recording_type": "Chat::Line"},  # near-miss on the prefix
            {},
        ],
    )
    def test_an_unroutable_pointer_is_refused_before_any_request(self, pointer):
        route = respx.route(host="3.basecampapi.com")
        with pytest.raises(UnknownRecordingTypeError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, **pointer)
        assert not route.called
        assert raised.value.code == "unknown_recording_type"

    @respx.mock
    def test_a_refusal_reports_the_routing_key_as_given(self):
        # Go's RecordingRoutingError names the field it was handed, untrimmed.
        with pytest.raises(UnknownRecordingTypeError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, recording_type="  Bogus  ")
        assert "'  Bogus  '" in str(raised.value)

        with pytest.raises(NoRecordingTypeError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type=" boost.created ")
        assert "' boost.created '" in str(raised.value)

    @respx.mock
    @pytest.mark.parametrize("separator", ["\x1c", "\x1d", "\x1e", "\x1f"])
    def test_does_not_trim_the_c0_separators_python_strips(self, separator):
        # `str.strip()` removes these four and Go's TrimSpace does not, so a
        # routing key carrying one is unroutable in Go and must be here.
        with pytest.raises(UnknownRecordingTypeError):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, recording_type=f"{separator}Comment")

    @respx.mock
    def test_the_recording_type_wins_over_the_event_type(self):
        # The recording type is the more exact of the two, so a row carrying
        # both routes on it.
        route = respx.get(f"{BASE}/documents/7").mock(
            return_value=httpx.Response(200, json={"id": 7, "type": "Document", "bucket": {"id": BUCKET}})
        )
        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=7, event_type="comment.created", recording_type="Document"
        )
        assert route.called
        assert summary["type"] == "Document"

    @pytest.mark.parametrize(
        "pointer",
        [
            {"bucket_id": 0, "recording_id": 1},
            {"bucket_id": 1, "recording_id": 0},
            {"bucket_id": -1, "recording_id": 1},
            # `bool` is an `int` in Python, so a bare range test would read
            # True as recording 1 and go and fetch it.
            {"bucket_id": True, "recording_id": 1},
            {"bucket_id": 1, "recording_id": True},
            # A non-integer must reach the SDK's own usage error rather than a
            # bare TypeError from the comparison.
            {"bucket_id": "2085958499", "recording_id": 1},
            {"bucket_id": 1, "recording_id": None},
        ],
    )
    def test_a_pointer_that_names_no_recording_is_a_usage_error(self, pointer):
        with pytest.raises(UsageError, match="bucket id and recording id"):
            _account().recordings.summarize(event_type="comment.created", **pointer)


class TestProjection:
    @respx.mock
    def test_a_read_from_another_bucket_is_refused(self):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(
                200, json={"id": 1, "type": "Comment", "content": "", "bucket": {"id": 777, "name": "Elsewhere"}}
            )
        )
        with pytest.raises(BucketMismatchError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        assert raised.value.code == "bucket_mismatch"
        assert raised.value.bucket_id == 777
        assert raised.value.requested_bucket_id == BUCKET

    @respx.mock
    def test_a_read_carrying_no_bucket_is_not_a_mismatch(self):
        respx.get(f"{BASE}/comments/1").mock(return_value=httpx.Response(200, json={"id": 1, "type": "Comment"}))
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        assert summary["bucket"] is None

    # Parametrized, not looped: respx keeps the FIRST route registered for a
    # pattern, so re-mocking the same URL inside one `respx.mock` leaves every
    # iteration after the first answering with iteration one's body -- a loop
    # here tests its first value and nothing else.
    @respx.mock
    @pytest.mark.parametrize("bad", [float(BUCKET), str(BUCKET), True, 2**63])
    def test_a_bucket_id_that_is_not_an_int64_fails_the_read(self, bad):
        # The float is the dangerous shape: `2085958499.0 == 2085958499` is
        # True in Python, so an untyped id would wave a recording from another
        # project straight through the one check that exists to stop it.
        #
        # It is a DECODE failure, not a mismatch. Reporting it as a
        # BucketMismatchError named a bucket that does not exist and put a str
        # or a float into a field declared `int`; Go fails the read, and a
        # caller branching on `.bucket_id` was being told a number the payload
        # never carried.
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", "content": "", "bucket": {"id": bad}})
        )
        with pytest.raises(ApiError, match="bucket id was not an int64"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

    @respx.mock
    @pytest.mark.parametrize("bad", ["Elsewhere", [{"id": BUCKET}], 7])
    def test_a_bucket_that_is_not_an_object_fails_the_read(self, bad):
        # `*Bucket` refuses these at decode. Reading `.get` off them raised a
        # bare AttributeError from inside the safety check, and skipping the
        # check instead would wave the recording through.
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", "content": "", "bucket": bad})
        )
        with pytest.raises(ApiError, match="bucket was not an object"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

    @respx.mock
    @pytest.mark.parametrize(
        ("assignees", "expected"),
        [
            # `[]Person`. A STRING is the bad one: `list("oops")` invented four
            # assignees out of it and put them in the returned summary, which
            # is worse than any refusal. A number raised a bare TypeError.
            (None, []),
            ([], []),
            ([None], [{}]),  # a null element is Go's zero Person, not None
            ([{"id": 7}], [{"id": 7}]),
        ],
    )
    def test_assignees_decode_as_a_person_list(self, assignees, expected):
        respx.get(f"{BASE}/todos/9").mock(
            return_value=httpx.Response(200, json={"id": 9, "type": "Todo", "assignees": assignees})
        )
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=9, event_type="todo.created")
        assert summary["assignees"] == expected

    @respx.mock
    @pytest.mark.parametrize("assignees", ["oops", {"a": 1}, 7, True, [7], ["x"]])
    def test_assignees_that_are_not_people_fail_the_read(self, assignees):
        respx.get(f"{BASE}/todos/9").mock(
            return_value=httpx.Response(200, json={"id": 9, "type": "Todo", "assignees": assignees})
        )
        with pytest.raises(ApiError, match="assignees was not an array|assignee was not an object"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=9, event_type="todo.created")

    @respx.mock
    @pytest.mark.parametrize(
        ("field", "value", "match"),
        [
            # The body guard was TOP-LEVEL only, so junk one field down sailed
            # through and landed in the summary verbatim.
            ("parent", "oops", "parent was not an object"),
            ("parent", [], "parent was not an object"),
            ("creator", 7, "creator was not an object"),
            ("creator", True, "creator was not an object"),
            ("content", 7, "content was not a string"),
            ("title", [], "title was not a string"),
            ("status", 7, "status was not a string"),
            ("app_url", {}, "app_url was not a string"),
        ],
    )
    def test_a_nested_field_of_the_wrong_type_fails_the_read(self, field, value, match):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", field: value})
        )
        with pytest.raises(ApiError, match=match):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

    # Every row below is Go's own answer, read off a linked oracle driving the
    # real generated types -- not derived from this implementation.
    @respx.mock
    @pytest.mark.parametrize(
        ("creator_id", "fails"),
        [
            # `Person.Id` is FlexibleInt64, so a numeric STRING resolves and a
            # non-numeric one is the system-actor 0 rather than a failure.
            ("7", False),
            ("basecamp", False),
            ("", False),
            ("007", False),
            ("+7", False),
            (7, False),
            (-7, False),
            # ...but `null` IS an error here, where a plain int64 field reads
            # it as 0. The two rules are neighbours and differ.
            (None, True),
            (True, True),
            (7.0, True),
            (2**63, True),
            # The magnitude check happens INSIDE Go's scan and against uint64,
            # so the first disqualifying thing wins. These three rows are the
            # whole point: a junk-free corpus never contains them, and "is it
            # all digits? then parse" gets the last one wrong -- answering the
            # system-actor 0 where Go fails the read.
            ("9223372036854775807x", False),
            ("18446744073709551615x", False),
            ("18446744073709551616x", True),
            ("9223372036854775808", True),
            ("18446744073709551615", True),
            ("-9223372036854775808", False),
            ("-9223372036854775809", True),
            ("-9223372036854775809x", False),
        ],
    )
    def test_a_creator_id_follows_the_flexible_int64_rule(self, creator_id, fails):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", "creator": {"id": creator_id}})
        )
        if fails:
            with pytest.raises(ApiError, match="int64"):
                _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        else:
            summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
            assert summary["creator"] == {"id": creator_id}

    @respx.mock
    @pytest.mark.parametrize(
        ("path", "body", "event", "bad_key"),
        [
            ("messages/2", {"id": 2, "type": "Message", "title": "ok", "subject": 7}, "message.created", "subject"),
            (
                "todos/2",
                {"id": 2, "type": "Todo", "title": "T", "content": 7, "description": "d"},
                "todo.created",
                "content",
            ),
        ],
    )
    def test_a_later_title_key_is_decoded_even_when_an_earlier_one_answers(self, path, body, event, bad_key):
        # `_text` returns the first non-empty key, so returning early left the
        # LATER keys unread and a number there sailed through. Go has decoded
        # the whole struct before `firstNonEmpty` runs, so it fails the read.
        # Eleven routed types have a two-key title or content tuple.
        respx.get(f"{BASE}/{path}").mock(return_value=httpx.Response(200, json=body))
        with pytest.raises(ApiError, match=f"{bad_key} was not a string"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=2, event_type=event)

    @respx.mock
    def test_a_parent_id_is_a_plain_int64_not_a_flexible_one(self):
        # The asymmetry that makes one shared rule impossible: `"7"` resolves
        # for a creator and FAILS THE READ for a parent, because
        # `RecordingParent.Id` is `int64` where `Person.Id` is FlexibleInt64.
        respx.get(f"{BASE}/todos/9").mock(
            return_value=httpx.Response(200, json={"id": 9, "type": "Todo", "parent": {"id": "7"}})
        )
        with pytest.raises(ApiError, match="parent id was not an int64"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=9, event_type="todo.created")

    @respx.mock
    @pytest.mark.parametrize(
        ("value", "fails"),
        [(None, False), ("2026-09-15T20:00:00Z", False), ("oops", False), (7, True), ([], True), ({}, True)],
    )
    def test_updated_at_is_type_checked_but_not_parsed(self, value, fails):
        # `time.Time` in Go: a number or an object fails the read. A string it
        # could not parse as RFC 3339 fails there too and does NOT here -- the
        # port keeps the API's own string rather than an instant, so the "oops"
        # row is a KNOWN divergence recorded in Appendix F, not an oversight.
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", "updated_at": value})
        )
        if fails:
            with pytest.raises(ApiError, match="updated_at was not a string"):
                _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        else:
            summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
            assert summary["updated_at"] == value

    @respx.mock
    def test_a_null_body_is_a_zero_recording_not_a_failed_read(self):
        # `json.Unmarshal` of `null` into a struct is a NO-OP at any depth: no
        # error, the zero value left in place. So Go projects a null body into
        # a recording with every field empty and returns it. Refusing it here
        # would reject a body the contract accepts -- the converse of the hole
        # the body guard was added to close.
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(
                200, content=b"null", headers={"Content-Type": "application/json; charset=utf-8"}
            )
        )
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        assert summary["id"] == 0
        assert summary["type"] == ""
        assert summary["bucket"] is None
        assert summary["mentioned_person_ids"] == []

    @respx.mock
    @pytest.mark.parametrize(("body", "shown"), [(b"[]", "list"), (b'"oops"', "str"), (b"7", "int")])
    def test_a_body_that_is_not_an_object_is_a_failed_read(self, body, shown):
        # The other half of the same asymmetry: every non-object that is not
        # `null` IS a decode error in Go and the read never returns. Projecting
        # one would build a summary out of something that is not a recording,
        # and `"oops".get` raised a bare AttributeError from outside the
        # taxonomy.
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, content=body, headers={"Content-Type": "application/json; charset=utf-8"})
        )
        with pytest.raises(ApiError, match=f"not an object: {shown}"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

    @respx.mock
    def test_a_bucket_id_of_zero_is_not_a_mismatch(self):
        # Go: `summary.Bucket.ID != 0 && summary.Bucket.ID != ref.BucketID`.
        # A zero id is the decoded absence of one, not a different project.
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", "content": "", "bucket": {"id": 0}})
        )
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        assert summary["bucket"] == {"id": 0}

    @respx.mock
    def test_a_payload_with_no_id_projects_zero_not_none(self):
        # `id` is declared `int`, and Go cannot produce anything but 0 here.
        respx.get(f"{BASE}/vaults/9").mock(return_value=httpx.Response(200, json={"type": "Vault"}))
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=9, recording_type="Vault")
        assert summary["id"] == 0

    @respx.mock
    def test_absent_fields_project_as_empties_rather_than_missing_keys(self):
        respx.get(f"{BASE}/vaults/9").mock(return_value=httpx.Response(200, json={"id": 9, "type": "Vault"}))
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=9, recording_type="Vault")
        assert summary == {
            "id": 9,
            "status": "",
            "type": "Vault",
            "title": "",
            "app_url": "",
            "parent": None,
            "bucket": None,
            "creator": None,
            "assignees": [],
            "mentioned_person_ids": [],
            "content": "",
            "updated_at": None,
            "campfire_id": None,
        }

    @respx.mock
    @pytest.mark.parametrize(
        ("line_type", "expected"),
        [
            ("Chat::Lines::RichText", [1049715915]),
            ("Chat::Lines::Integration", [1049715915]),
            ("Chat::Lines::Text", []),
            ("Chat::Lines::Code", []),
        ],
    )
    def test_only_a_rich_text_chat_line_can_mention_anyone(self, line_type, expected):
        # A Text line's content is HTML-escaped on the way out and a Code line's
        # is served verbatim, so a literal bc-attachment in either is text BC3
        # never read as markup.
        sgid = (
            "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7"
            "AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"
        )
        content = f'<div><bc-attachment sgid="{sgid}"></bc-attachment> hi</div>'
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1)))
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(
            return_value=httpx.Response(200, json=_line(1, line_type=line_type, content=content))
        )
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, recording_type=line_type)
        assert summary["mentioned_person_ids"] == expected


class TestChatLineDiscovery:
    @respx.mock
    def test_the_dock_answers_without_a_listing(self):
        # The listing is the expensive request; a project's own dock names its
        # Campfire, so it is never reached for one.
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        # Mounted as a FAILURE, not an empty success: "not requested" proves
        # only that the call did not NEED the listing, while a 503 proves the
        # verdict does not DEPEND on it — the distinction matters precisely
        # because a non-404 from a discovery source is meant to pass through.
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(503, json={"error": "down"}))
        line = respx.get(f"{BASE}/chats/77/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(77)))

        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert project.called and line.called
        assert not listing.called
        assert summary["campfire_id"] == 77

    @respx.mock
    def test_a_non_404_from_a_candidate_stops_the_search_and_is_raised(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1, 2)))
        first = respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(
            return_value=httpx.Response(403, json={"error": "Access denied"})
        )
        second = respx.get(f"{BASE}/chats/2/lines/{LINE_ID}").mock(return_value=_not_found())

        with pytest.raises(ForbiddenError):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert first.called
        assert not second.called, "a permission failure must not be tried past as if it meant 'not here'"

    @respx.mock
    def test_every_candidate_404ing_is_unresolved_not_not_found(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1, 2)))
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))

        with pytest.raises(RecordingUnresolvedError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.code == "recording_unresolved"
        assert raised.value.campfire_ids == [1, 2]
        assert raised.value.bucket_id == BUCKET

    @respx.mock
    def test_a_bucket_that_is_not_a_project_falls_through_to_the_listing(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(200, json=[_campfire(5), _campfire(6, bucket_id=999)])
        )
        elsewhere = respx.get(f"{BASE}/chats/6/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(6)))
        respx.get(f"{BASE}/chats/5/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(5)))

        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert listing.called
        assert summary["campfire_id"] == 5
        assert not elsewhere.called, "a Campfire in another bucket is never a candidate"

    @respx.mock
    def test_more_candidates_than_the_budget_is_incomplete_never_unresolved(self):
        # Nothing left unsearched is ever reported absent.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(200, json=[_campfire(i) for i in range(1, MAX_CAMPFIRE_CANDIDATES + 11)])
        )
        lines = respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())

        with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.code == "campfire_discovery_incomplete"
        assert str(MAX_CAMPFIRE_CANDIDATES) in raised.value.reason
        assert lines.call_count == MAX_CAMPFIRE_CANDIDATES

    @respx.mock
    def test_a_dock_that_spends_the_budget_exactly_never_fetches_the_listing(self):
        # The exact boundary: the budget reaches zero without `skipped` ever
        # being set, because no 51st candidate was observed. Go guards on the
        # BUDGET, not on `skipped`, so the listing is not fetched — a request
        # that cannot help, and whose failure would replace a deterministic
        # "incomplete" with a transient error a consumer retries forever.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(200, json=_project(*range(1, MAX_CAMPFIRE_CANDIDATES + 1)))
        )
        lines = respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(500, json={"error": "boom"}))

        with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert lines.call_count == MAX_CAMPFIRE_CANDIDATES
        assert not listing.called, "a spent budget must not buy a request that cannot help"
        assert "was spent before the account listing was consulted" in raised.value.reason

    @respx.mock
    def test_a_cached_dock_that_spends_the_budget_exactly_is_not_re_read(self):
        # The `search.budget > 0` half of the pass-2 dock guard, which is the
        # ONLY half the other budget tests reach: they drive a freshly fetched
        # dock, where `dock.cached` is False and the budget term is never
        # evaluated. Reaching it needs a CACHED dock holding exactly the budget
        # in candidates, on a second call, past the refresh floor -- otherwise
        # the floor declines the re-read and a guard that has been deleted
        # looks identical to one that holds.
        clock = [0.0]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(200, json=_project(*range(1, MAX_CAMPFIRE_CANDIDATES + 1)))
        )
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(500, json={"error": "boom"}))
        account = _account(now=lambda: clock[0])

        # First call fetches the dock and spends the whole budget on it.
        with pytest.raises(CampfireDiscoveryIncompleteError):
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        assert project.call_count == 1

        # Past the refresh floor, so a re-read WOULD issue a request; still
        # inside the TTL, so the dock is served from cache and `dock.cached`
        # is True. The budget is spent again on the 50 cached candidates, and
        # the guard is what stops the re-read.
        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert project.call_count == 1, "a spent budget must not buy a dock re-read that cannot help"
        assert "was spent before the account listing was consulted" in raised.value.reason

    @respx.mock
    def test_a_spent_budget_with_the_listing_already_consulted_is_unresolved(self):
        # The other half of Go's rule: when BOTH sources were consulted,
        # running out of budget is not "something was left unsearched" — every
        # candidate that exists was tried, so the verdict is unresolved.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(200, json=[_campfire(i) for i in range(1, MAX_CAMPFIRE_CANDIDATES + 1)])
        )
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/\d+").mock(return_value=_not_found())
        account = _account()

        # First call fills the listing cache and spends its own budget.
        with pytest.raises(RecordingUnresolvedError):
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        # Second call consults the cached listing in pass 1, spends the budget
        # there, and must conclude unresolved rather than incomplete.
        with pytest.raises(RecordingUnresolvedError) as raised:
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID + 1, event_type="chat.line.created")
        assert len(raised.value.campfire_ids) == MAX_CAMPFIRE_CANDIDATES

    @respx.mock
    def test_a_listing_failure_that_is_not_an_overflow_passes_through(self):
        # Only a listing OVER ITS CAP is a settled verdict about where the line
        # is. A 403 says nothing about that, so it is raised as itself rather
        # than becoming "discovery incomplete". Named after Go's
        # TestSummarize_ChatLineListingFailurePassesThrough.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(403, json={"error": "Denied"}))

        with pytest.raises(ForbiddenError):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

    @respx.mock
    def test_the_dock_refresh_gets_its_say_before_the_listing_is_fetched(self):
        # A listing that is down must never stand between a project's line and
        # the one project read that finds it. Named after Go's
        # TestSummarize_ChatLineDockRefreshIsNotBlockedByTheListing.
        clock = [0.0]
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            side_effect=[httpx.Response(200, json=_project(1)), httpx.Response(200, json=_project(1, 2))]
        )
        respx.get(url__regex=rf"{BASE}/chats/1/lines/\d+").mock(return_value=_not_found())
        respx.get(url__regex=rf"{BASE}/chats/2/lines/\d+").mock(return_value=httpx.Response(200, json=_line(2)))
        listing = respx.get(f"{BASE}/chats.json").mock(
            side_effect=[httpx.Response(200, json=[]), httpx.Response(500, json={"error": "boom"})]
        )
        account = _account(now=lambda: clock[0])

        # Prime both caches; the line is under neither source yet.
        with pytest.raises(RecordingUnresolvedError):
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        assert listing.call_count == 1

        # The Campfire now exists. The dock refresh finds it, and the listing —
        # which would answer 500 — is never consulted again.
        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        summary = account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert summary["campfire_id"] == 2
        assert listing.call_count == 1, "a failing listing must not block the dock's refresh"

    @respx.mock
    def test_a_listing_entry_missing_an_id_is_tried_as_zero(self):
        # It is NOT skipped: an absent `id` decodes to the int64 zero value and
        # Go's listing loop appends `c.ID` with no test, so the oracle shows
        # `GET /chats/0/lines/N`. Skipping it spends none of the candidate
        # budget Go spends, which moves the verdict and not merely the ids.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(
                200,
                json=[{"type": "Chat::Transcript", "bucket": {"id": BUCKET}}, _campfire(5)],
            )
        )
        zero = respx.get(f"{BASE}/chats/0/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats/5/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(5)))

        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert summary["campfire_id"] == 5
        assert zero.called, "Go issues this request, so the budget it spends must be spent here too"

    # The two halves of the id rule are separated on purpose. The TYPE half is
    # ours: nothing typed stands between the composite and the wire, so a
    # `true` would otherwise become a request path. The VALUE half is Go's, and
    # it is DIFFERENT AT EVERY SITE -- a single `<= 0` looked like Go's rule,
    # was not, and silently dropped candidates Go tries. Each site's value rows
    # below were taken from a Go oracle driving `Summarize` end to end and
    # printing the request sequence, not from reading this implementation.

    @respx.mock
    @pytest.mark.parametrize("bad_id", [True, False, "5", 5.0, {"id": 5}, 2**63])
    def test_a_listing_entry_whose_id_is_not_an_int64_fails_the_read(self, bad_id):
        # Go decodes the listing into `[]Campfire` with an `int64` id, so each
        # of these fails the WHOLE response and the read never returns -- it
        # does not skip the entry. Skipping turned a read failure into a
        # definitive "not there", which are the two answers this composite
        # exists to keep apart. `2**63` is in the list because the RANGE is
        # part of the type: Go refuses it exactly as it refuses a string.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(
                200,
                json=[{"id": bad_id, "type": "Chat::Transcript", "bucket": {"id": BUCKET}}, _campfire(5)],
            )
        )
        any_line = respx.get(url__regex=rf"{BASE}/chats/.+/lines/\d+").mock(return_value=_not_found())

        with pytest.raises(ApiError, match="was not an int64|was not an object"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert not any_line.called, "a read that failed to decode issues no line reads at all"

    @respx.mock
    @pytest.mark.parametrize("listed_id", [0, -1])
    def test_a_listing_entry_keeps_an_id_go_does_not_screen(self, listed_id):
        # Go's listing loader tests `c.Bucket == nil || c.Bucket.ID == 0` and
        # appends `c.ID` UNVALIDATED -- oracle: `{"id": 0, "bucket": {...}}`
        # produces `GET /chats/0/lines/N`. Screening these here would cost a
        # unit of the candidate budget that Go spends, which moves the verdict.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(
                200,
                json=[{"id": listed_id, "type": "Chat::Transcript", "bucket": {"id": BUCKET}}, _campfire(5)],
            )
        )
        tried = respx.get(f"{BASE}/chats/{listed_id}/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats/5/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(5)))

        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert summary["campfire_id"] == 5
        assert tried.called, "Go issues this request, so the budget it spends must be spent here too"

    @respx.mock
    @pytest.mark.parametrize("bad_id", [True, "5", 5.0, 2**63])
    def test_a_dock_entry_whose_id_is_not_an_int64_fails_the_read(self, bad_id):
        # `Project.Dock` is `[]DockItem` with an `int64` id: the oracle shows
        # the project read failing outright, with no line read attempted. A
        # null or absent id is different and is covered separately -- that one
        # decodes to 0 and is skipped by Go's own `item.ID != 0`.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(
                200,
                json={"id": BUCKET, "dock": [{"id": bad_id, "name": "chat"}, {"id": 7, "name": "chat"}]},
            )
        )
        any_line = respx.get(url__regex=rf"{BASE}/chats/.+/lines/\d+").mock(return_value=_not_found())

        with pytest.raises(ApiError, match="was not an int64"):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert not any_line.called

    @respx.mock
    def test_a_dock_entry_of_zero_is_skipped_and_a_negative_one_is_tried(self):
        # Go's dock rule is `item.ID != 0` -- so 0 (which is also what a missing
        # or null id decodes to) is skipped, and -1 IS tried. The two halves
        # travel together because a single test asserting "both skipped" is
        # what the `<= 0` mis-port would have passed.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(
                200,
                json={
                    "id": BUCKET,
                    "dock": [
                        {"id": 0, "name": "chat"},
                        {"id": -1, "name": "chat"},
                        {"id": 7, "name": "chat"},
                    ],
                },
            )
        )
        zero = respx.get(f"{BASE}/chats/0/lines/{LINE_ID}").mock(return_value=_not_found())
        negative = respx.get(f"{BASE}/chats/-1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats/7/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(7)))

        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert summary["campfire_id"] == 7
        assert not zero.called, "Go's `item.ID != 0` skips this one"
        assert negative.called, "Go's `item.ID != 0` does NOT skip this one"

    @respx.mock
    def test_a_listing_over_its_cap_is_incomplete_and_is_not_cached(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        overflowing = [_campfire(i) for i in range(1, _campfire_index.MAX_CAMPFIRE_LISTING + 1)]
        listing = respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(
                200,
                json=overflowing,
                headers={"Link": f'<{BASE}/chats.json?page=2>; rel="next"'},
            )
        )
        account = _account()

        for _ in range(2):
            with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
                account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
            # The same text every runner reports: Go spells the constant,
            # not its value, and `reason` is a public field a shared
            # fixture can pin.
            assert raised.value.reason == "campfire listing exceeds MaxCampfireListing"

        assert listing.call_count == 2, "a truncated listing must not be cached as if it were the whole set"


class TestDiscoveryCache:
    @respx.mock
    def test_a_second_line_in_the_same_bucket_reuses_the_dock(self):
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        respx.get(url__regex=rf"{BASE}/chats/77/lines/\d+").mock(return_value=httpx.Response(200, json=_line(77)))
        account = _account()

        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID + 1, event_type="chat.line.created")

        assert project.call_count == 1

    @respx.mock
    def test_the_cache_expires(self):
        clock = [0.0]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        respx.get(url__regex=rf"{BASE}/chats/77/lines/\d+").mock(return_value=httpx.Response(200, json=_line(77)))
        account = _account(now=lambda: clock[0])

        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        clock[0] = _campfire_index.CAMPFIRE_INDEX_TTL + 1
        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert project.call_count == 2

    @respx.mock
    def test_a_miss_refreshes_the_cached_dock_before_concluding(self):
        # A Campfire created after the cache filled must be tried before the
        # line is called unresolvable.
        clock = [0.0]
        docks = [_project(1), _project(1, 2)]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(
            side_effect=[httpx.Response(200, json=dock) for dock in docks]
        )
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats/2/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(2)))
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = _account(now=lambda: clock[0])

        # Fill the dock cache with the snapshot that does not hold the line.
        with pytest.raises(RecordingUnresolvedError):
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        assert project.call_count == 1, "a freshly loaded source is not re-read in the same call"

        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        summary = account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert project.call_count == 2
        assert summary["campfire_id"] == 2

    @respx.mock
    def test_the_refresh_floor_bounds_a_run_of_unresolvable_lines(self):
        clock = [0.0]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1)))
        respx.get(url__regex=rf"{BASE}/chats/1/lines/\d+").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = _account(now=lambda: clock[0])

        for recording_id in (LINE_ID, LINE_ID + 1, LINE_ID + 2):
            with pytest.raises(RecordingUnresolvedError) as raised:
                account.recordings.summarize(
                    bucket_id=BUCKET, recording_id=recording_id, event_type="chat.line.created"
                )

        assert project.call_count == 1, "the floor keeps a run of misses from becoming a read per line"
        assert listing.call_count == 1
        assert raised.value.refreshed is False

    @respx.mock
    def test_a_candidate_the_refreshed_sources_dropped_is_reported_as_stale(self):
        # BC3 answers 404 for a Campfire the caller may no longer see, exactly
        # as it does for a line that is not there -- so "unresolved" reports
        # which candidates lost visibility rather than pretending to know.
        clock = [0.0]
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            side_effect=[httpx.Response(200, json=_project(1)), httpx.Response(200, json=_project())]
        )
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = _account(now=lambda: clock[0])

        with pytest.raises(RecordingUnresolvedError):
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        with pytest.raises(RecordingUnresolvedError) as raised:
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.refreshed is True
        assert raised.value.stale_campfire_ids == [1]

    @respx.mock
    def test_the_cache_does_not_cross_accounts(self):
        other = "https://3.basecampapi.com/999"
        first = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        second = respx.get(f"{other}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        respx.get(url__regex=r"https://3\.basecampapi\.com/\d+/chats/77/lines/\d+").mock(
            return_value=httpx.Response(200, json=_line(77))
        )
        client = _client()

        client.for_account(ACCOUNT).recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )
        client.for_account("999").recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert first.call_count == 1 and second.call_count == 1

    @respx.mock
    def test_a_failed_dock_read_is_raised_and_not_cached(self):
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(403, json={"error": "Access denied"})
        )
        account = _account()

        for _ in range(2):
            with pytest.raises(ForbiddenError):
                account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert project.call_count == 2, "a failed load leaves the cache as it was"

    @respx.mock
    def test_a_second_line_answers_from_the_cached_listing(self):
        # The listing is cached per account, so a burst of lines in a
        # non-project bucket costs one listing, not one per line.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[_campfire(5)]))
        respx.get(url__regex=rf"{BASE}/chats/5/lines/\d+").mock(return_value=httpx.Response(200, json=_line(5)))
        account = _account()

        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        summary = account.recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID + 1, event_type="chat.line.created"
        )

        assert listing.call_count == 1
        assert summary["campfire_id"] == 5

    @respx.mock
    def test_a_dock_over_the_budget_is_incomplete_before_the_listing_is_fetched(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(200, json=_project(*range(1, MAX_CAMPFIRE_CANDIDATES + 6)))
        )
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        # A 503, so the assertion pins independence rather than absence: were
        # the listing consulted, its failure would replace the deterministic
        # "incomplete" verdict with a transient one.
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(503, json={"error": "down"}))

        with pytest.raises(CampfireDiscoveryIncompleteError):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert not listing.called, "a spent budget must not buy a request that cannot help"


class RecordingHooks(BasecampHooks):
    def __init__(self):
        self.operations: list[str] = []

    def on_operation_start(self, info):
        self.operations.append(f"{info.service}.{info.operation}")


class TestHookIdentities:
    @respx.mock
    def test_hooks_see_the_constituent_reads_under_their_own_names(self):
        # SPEC section 18 rule 3: a composite mints no operation identity.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[_campfire(5)]))
        respx.get(f"{BASE}/chats/5/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(5)))
        hooks = RecordingHooks()

        _account(hooks=hooks).recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert hooks.operations == ["projects.get", "campfires.list", "campfires.get_line"]
        assert not any("summarize" in operation for operation in hooks.operations)

    @respx.mock
    def test_a_typed_read_reaches_hooks_as_that_service(self):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", "bucket": {"id": BUCKET}})
        )
        hooks = RecordingHooks()

        _account(hooks=hooks).recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

        assert hooks.operations == ["comments.get"]


@pytest.mark.asyncio
class TestAsync:
    @respx.mock
    async def test_routes_a_typed_read(self):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(
                200, json={"id": 1, "type": "Comment", "content": "hi", "bucket": {"id": BUCKET}}
            )
        )
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        summary = await account.recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

        assert summary["type"] == "Comment"
        assert summary["mentioned_person_ids"] == []

    @respx.mock
    async def test_discovers_a_chat_line_and_caches_the_dock(self):
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1, 2)))
        respx.get(url__regex=rf"{BASE}/chats/1/lines/\d+").mock(return_value=_not_found())
        respx.get(url__regex=rf"{BASE}/chats/2/lines/\d+").mock(return_value=httpx.Response(200, json=_line(2)))
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        summary = await account.recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )
        await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID + 1, event_type="chat.line.created")

        assert summary["campfire_id"] == 2
        assert project.call_count == 1

    @respx.mock
    async def test_unresolved_is_its_own_identity(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1)))
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        with pytest.raises(RecordingUnresolvedError):
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

    async def test_refuses_an_unroutable_pointer(self):
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)
        with pytest.raises(NoRecordingTypeError):
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="boost.created")

    @respx.mock
    async def test_a_dock_over_the_budget_is_incomplete(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(200, json=_project(*range(1, MAX_CAMPFIRE_CANDIDATES + 6)))
        )
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.code == "campfire_discovery_incomplete"

    @respx.mock
    async def test_a_miss_refreshes_the_cached_dock_before_concluding(self):
        clock = [0.0]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(
            side_effect=[httpx.Response(200, json=_project(1)), httpx.Response(200, json=_project(1, 2))]
        )
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats/2/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(2)))
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        client = AsyncClient(access_token="test-token")
        client._campfire_index = AsyncCampfireIndex(now=lambda: clock[0])
        account = client.for_account(ACCOUNT)

        with pytest.raises(RecordingUnresolvedError):
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        summary = await account.recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert project.call_count == 2
        assert summary["campfire_id"] == 2
