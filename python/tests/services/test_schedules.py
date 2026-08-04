"""Tests for the schedule-entry merge-safe update / edit / replace surface (sync + async).

``PUT /schedule_entries/{id}`` is a full replace: BC3 rebuilds the
``Schedule::Entry`` from the permitted params and swaps the recordable
wholesale. Omitting ``summary`` leaves the entry reading back as "Untitled";
omitting ``description`` clears it; omitting ``all_day`` turns an all-day event
into a midnight-to-midnight timed one. None of those is a 422, so nothing on the
wire tells you the sparse PUT went wrong — only the next GET does.

Three fields are the exception. BC3 seeds ``participant_ids``, ``url`` and
``highlighted`` from the existing recordable when the request does not address
them, so for those an omission *preserves*. That makes the two halves of this
file symmetric and opposite:

* the five full-state fields must appear on **every** PUT, empties included;
* the four addressed-only fields must appear on **no** PUT the caller did not
  address — and must appear, exactly as given, on every PUT the caller did,
  including when the value they were given equals the one the GET returned.
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ApiError, UsageError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"
ENTRY_URL = f"{BASE}/schedule_entries/5001"

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"


def load_fixture(rel: str) -> dict:
    return json.loads((_FIXTURES / rel).read_text(encoding="utf-8"))


def _entry(entry_id: int = 5001, **overrides) -> dict:
    # Source the full validated fixture for shape (every required ScheduleEntry
    # field is present), then pin the test-critical values the assertions trace
    # through to the PUT body. `url` is deliberately the entry's own Basecamp
    # API URL and `join_url` the meeting link: the composite must never confuse
    # them.
    return {
        **load_fixture("schedules/entry_get.json"),
        "id": entry_id,
        "summary": "Team Meeting",
        "starts_at": "2026-06-05T06:00:00Z",
        "ends_at": "2026-06-05T08:30:00Z",
        "description": "<div>Agenda in the doc.</div>",
        "all_day": False,
        "url": f"https://3.basecampapi.com/12345/buckets/1/schedule_entries/{entry_id}.json",
        "join_url": "https://meet.example.com/team",
        "highlighted": True,
        **overrides,
    }


def _put_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[str] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(f"{info.service}.{info.operation}")


def _sync_schedules(hooks: BasecampHooks | None = None):
    return Client(access_token="test-token", hooks=hooks).for_account("12345").schedules


def _async_schedules(hooks: BasecampHooks | None = None):
    return AsyncClient(access_token="test-token", hooks=hooks).for_account("12345").schedules


def _routes(entry: dict | None = None):
    body = _entry() if entry is None else entry
    get_route = respx.get(ENTRY_URL).mock(return_value=httpx.Response(200, json=body))
    put_route = respx.put(ENTRY_URL).mock(return_value=httpx.Response(200, json=_entry()))
    return get_route, put_route


# --- full state: always resent ----------------------------------------------


class TestSyncUpdateFullState:
    @respx.mock
    def test_summary_only_update_preserves_the_other_four(self):
        get_route, put_route = _routes()

        result = _sync_schedules().update_entry(entry_id=5001, summary="Team Meeting & Kickoff")

        assert result["id"] == 5001
        assert get_route.called
        body = _put_body(put_route)
        assert body["summary"] == "Team Meeting & Kickoff"
        # The four the caller never mentioned ride back verbatim. A sparse PUT
        # here would have been a silent 200 that cleared or defaulted each.
        assert body["starts_at"] == "2026-06-05T06:00:00Z"
        assert body["ends_at"] == "2026-06-05T08:30:00Z"
        assert body["description"] == "<div>Agenda in the doc.</div>"
        assert body["all_day"] is False

    @respx.mock
    def test_all_day_true_survives_an_update_that_never_mentions_it(self):
        # The read guard for `all_day` cannot be a truthiness test, and this is
        # the shape that proves it in the other direction: an all-day entry
        # whose times are bare dates. `starts_at`/`ends_at` are round-tripped
        # verbatim, never parsed.
        _, put_route = _routes(_entry(all_day=True, starts_at="2026-06-05", ends_at="2026-06-05"))

        _sync_schedules().update_entry(entry_id=5001, summary="Company Offsite")

        body = _put_body(put_route)
        assert body["all_day"] is True
        assert body["starts_at"] == "2026-06-05"
        assert body["ends_at"] == "2026-06-05"

    @respx.mock
    def test_all_day_false_survives_an_update_that_never_mentions_it(self):
        _, put_route = _routes(_entry(all_day=False))

        _sync_schedules().update_entry(entry_id=5001, summary="Team Meeting")

        body = _put_body(put_route)
        assert "all_day" in body, "a full-replace PUT names all_day even when it is False"
        assert body["all_day"] is False

    @respx.mock
    def test_explicit_all_day_true_is_sent(self):
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, all_day=True)

        assert _put_body(put_route)["all_day"] is True

    @respx.mock
    def test_explicit_empty_description_clears_it(self):
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, description="")

        body = _put_body(put_route)
        assert "description" in body, "a clear is an empty string, never an omission"
        assert body["description"] == ""
        assert body["summary"] == "Team Meeting"

    @respx.mock
    def test_absent_description_stays_genuinely_empty(self):
        # `description` is optional and nullable on the response, so absent is
        # not malformed — it is empty, and the PUT says so explicitly.
        entry = _entry()
        entry.pop("description", None)
        _, put_route = _routes(entry)

        _sync_schedules().update_entry(entry_id=5001, summary="set by the caller")

        assert _put_body(put_route)["description"] == ""

    @respx.mock
    def test_never_sends_json_null(self):
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, summary="Q3 Kickoff", description="")

        body = _put_body(put_route)
        assert all(value is not None for value in body.values())

    @respx.mock
    def test_hooks_observe_get_entry_then_replace_entry(self):
        _routes()

        hooks = _RecordingHooks()
        _sync_schedules(hooks).update_entry(entry_id=5001, summary="observed")

        assert hooks.operations == ["schedules.get_entry", "schedules.replace_entry"]


# --- addressed-only: never echoed, always sent when addressed ----------------


class TestSyncUpdateCarveOuts:
    @respx.mock
    def test_a_populated_read_back_is_never_echoed(self):
        # The GET carries a join link, a highlight and two participants. BC3
        # preserves all three when the body omits them, so resending is
        # redundant at best and wrong if the read raced a concurrent change.
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, summary="Team Meeting & Kickoff")

        body = _put_body(put_route)
        assert "participant_ids" not in body
        assert "highlighted" not in body
        assert "notify" not in body
        # The response's `url` is the entry's own Basecamp API URL; the
        # request's `url` is the join link the response spells `join_url`.
        # Echoing one into the other writes an API URL into the meeting link.
        assert "url" not in body

    @respx.mock
    def test_addressed_url_and_highlight_reach_the_wire(self):
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, url="https://meet.example.com/new-room", highlighted=True)

        body = _put_body(put_route)
        assert body["url"] == "https://meet.example.com/new-room"
        assert body["highlighted"] is True
        # Independent, not all-or-nothing: this caller said nothing about
        # participants, so BC3 keeps them.
        assert "participant_ids" not in body
        assert body["summary"] == "Team Meeting"
        assert body["starts_at"] == "2026-06-05T06:00:00Z"

    @respx.mock
    def test_explicit_empties_clear_each_carve_out(self):
        # The falsey-value trap: "" / [] / False are addresses, not absences,
        # and each must survive body compaction.
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, url="", highlighted=False, participant_ids=[])

        body = _put_body(put_route)
        assert body["url"] == ""
        assert body["highlighted"] is False
        assert body["participant_ids"] == []

    @respx.mock
    def test_addressed_participant_ids_reach_the_wire(self):
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, participant_ids=[1049715914])

        assert _put_body(put_route)["participant_ids"] == [1049715914]

    @respx.mock
    def test_notify_is_a_directive_sent_only_when_addressed(self):
        _, put_route = _routes()

        _sync_schedules().update_entry(entry_id=5001, summary="quiet")
        assert "notify" not in _put_body(put_route)

        _sync_schedules().update_entry(entry_id=5001, notify=True)
        assert _put_body(put_route)["notify"] is True


# --- edit: setter-invocation dirty tracking ---------------------------------


class TestSyncEdit:
    @respx.mock
    def test_edit_puts_full_state_back(self):
        _, put_route = _routes()

        with _sync_schedules().edit_entry(entry_id=5001) as e:
            assert e.summary == "Team Meeting"
            assert e.starts_at == "2026-06-05T06:00:00Z"
            assert e.all_day is False
            e.summary = f"🚨 {e.summary}"

        assert e.result["id"] == 5001
        body = _put_body(put_route)
        assert body["summary"] == "🚨 Team Meeting"
        assert body["ends_at"] == "2026-06-05T08:30:00Z"
        assert body["description"] == "<div>Agenda in the doc.</div>"
        assert body["all_day"] is False

    @respx.mock
    def test_clear_description_present_and_empty(self):
        _, put_route = _routes()

        with _sync_schedules().edit_entry(entry_id=5001) as e:
            e.description = ""

        body = _put_body(put_route)
        assert "description" in body
        assert body["description"] == ""
        assert body["summary"] == "Team Meeting"

    @respx.mock
    def test_untouched_carve_outs_stay_off_the_wire(self):
        # The edit view is seeded with the join link, highlight and
        # participants so a block can inspect them — reading is not writing.
        _, put_route = _routes()

        with _sync_schedules().edit_entry(entry_id=5001) as e:
            assert e.url == "https://meet.example.com/team"
            assert e.highlighted is True
            assert e.participant_ids == [1049715914, 1049715915]
            e.summary = "Team Sync"

        body = _put_body(put_route)
        assert body["summary"] == "Team Sync"
        assert "url" not in body
        assert "highlighted" not in body
        assert "participant_ids" not in body
        assert "notify" not in body

    @respx.mock
    def test_assigning_the_read_backs_own_value_still_sends_it(self):
        # The reason the contract is setter-invocation dirty tracking rather
        # than a snapshot diff. A value comparison concludes nothing changed
        # here and omits both keys — handing the write back to BC3's carve-out,
        # which preserves. Intent is not recoverable from the value:
        # `e.url = e.url` is a write.
        _, put_route = _routes()

        with _sync_schedules().edit_entry(entry_id=5001) as e:
            e.url = e.url
            e.highlighted = e.highlighted

        body = _put_body(put_route)
        assert body["url"] == "https://meet.example.com/team"
        assert body["highlighted"] is True
        assert body["summary"] == "Team Meeting"
        assert "participant_ids" not in body

    @respx.mock
    def test_assigning_the_read_backs_own_participants_still_sends_them(self):
        _, put_route = _routes()

        with _sync_schedules().edit_entry(entry_id=5001) as e:
            e.participant_ids = e.participant_ids

        assert _put_body(put_route)["participant_ids"] == [1049715914, 1049715915]

    @respx.mock
    def test_clearing_carve_outs_in_a_block_reaches_the_wire(self):
        _, put_route = _routes()

        with _sync_schedules().edit_entry(entry_id=5001) as e:
            e.url = ""
            e.highlighted = False
            e.participant_ids = []

        body = _put_body(put_route)
        assert body["url"] == ""
        assert body["highlighted"] is False
        assert body["participant_ids"] == []

    @respx.mock
    def test_absent_join_url_and_highlight_seed_empty(self):
        entry = _entry()
        entry.pop("join_url", None)
        entry.pop("highlighted", None)
        _, put_route = _routes(entry)

        with _sync_schedules().edit_entry(entry_id=5001) as e:
            assert e.url == ""
            assert e.highlighted is False
            e.summary = "still fine"

        assert "url" not in _put_body(put_route)

    @respx.mock
    def test_assigning_none_to_a_carve_out_is_a_usage_error(self):
        # None is neither a value nor an absence on a full write: compaction
        # would silently drop it, which reads as "not addressed" and hands the
        # write back to BC3.
        _, put_route = _routes()

        with (
            pytest.raises(UsageError, match="url was set to None"),
            _sync_schedules().edit_entry(entry_id=5001) as e,
        ):
            e.url = None

        assert not put_route.called

    @respx.mock
    def test_exception_aborts_without_put(self):
        _, put_route = _routes()

        with pytest.raises(RuntimeError, match="abort"), _sync_schedules().edit_entry(entry_id=5001) as e:
            e.summary = "never written"
            raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    def test_result_raises_before_completion(self):
        _routes()

        edit = _sync_schedules().edit_entry(entry_id=5001)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result

    @respx.mock
    def test_hooks_observe_get_entry_then_replace_entry(self):
        _routes()

        hooks = _RecordingHooks()
        with _sync_schedules(hooks).edit_entry(entry_id=5001) as e:
            e.summary = "observed"

        assert hooks.operations == ["schedules.get_entry", "schedules.replace_entry"]


class TestSyncReplace:
    @respx.mock
    def test_sparse_replace_issues_no_get_and_omits_unset(self):
        get_route, put_route = _routes()

        result = _sync_schedules().replace_entry(
            entry_id=5001,
            summary="Team Meeting",
            starts_at="2026-06-05T06:00:00Z",
            ends_at="2026-06-05T08:30:00Z",
        )

        assert result["id"] == 5001
        assert not get_route.called, "replace_entry is the deliberate overwrite — it reads nothing first"
        assert respx.calls.call_count == 1
        body = _put_body(put_route)
        assert body["summary"] == "Team Meeting"
        # Unaddressed carve-outs stay off the wire and BC3 preserves them.
        assert "participant_ids" not in body
        assert "url" not in body
        assert "highlighted" not in body

    @respx.mock
    def test_replace_sends_explicit_empties(self):
        _, put_route = _routes()

        _sync_schedules().replace_entry(
            entry_id=5001,
            summary="Team Meeting",
            starts_at="2026-06-05T06:00:00Z",
            ends_at="2026-06-05T08:30:00Z",
            participant_ids=[],
            url="",
            highlighted=False,
        )

        body = _put_body(put_route)
        assert body["participant_ids"] == []
        assert body["url"] == ""
        assert body["highlighted"] is False


# --- the read guards: a malformed GET must never reach the PUT --------------
#
# `update_entry`/`edit_entry` GET the entry, read each writable field, and PUT
# the FULL representation back, so every value read is written — including one
# the caller never mentioned. Python has no typed decoder between the GET and
# the read (`get_entry` returns `dict[str, Any]`), so the refusal is explicit.
#
# The assertion that matters is the ORDERING: exactly one request may leave the
# client. A guard that fires after the PUT has already lost the field.

_MALFORMED_STRINGS = [
    pytest.param(False, id="false"),
    pytest.param(0, id="zero"),
    pytest.param([], id="empty-list"),
    pytest.param(42, id="number"),
    pytest.param(True, id="true"),
    pytest.param(["x"], id="list"),
    pytest.param({"a": 1}, id="dict"),
]


class TestMalformedResponseFields:
    @respx.mock
    @pytest.mark.parametrize("field", ["summary", "starts_at", "ends_at", "description"])
    @pytest.mark.parametrize("value", _MALFORMED_STRINGS)
    def test_update_refuses_a_non_string_before_writing(self, field, value):
        get_route, put_route = _routes(_entry(**{field: value}))

        with pytest.raises(ApiError) as excinfo:
            _sync_schedules().update_entry(entry_id=5001, summary="New summary")

        assert f"ScheduleEntry field {field!r} is not a string" in str(excinfo.value)
        # api_error, not usage: the value arrived in a successful response.
        assert excinfo.value.code == "api_error"
        assert get_route.called
        assert not put_route.called, "the guard must fire BEFORE the full-replace PUT"
        assert respx.calls.call_count == 1

    # `Schedule::Entry#summary` is `super.presence || "Untitled"`, so BC3 can
    # never render it blank. Absent, null or blank on a 2xx read is a MALFORMED
    # RESPONSE, not an empty summary — and coalescing it to "" would blank the
    # real summary on a call that only touched the description. Same defect
    # class as a forwarded non-string, in the one shape `or ""` looks correct.
    @respx.mock
    @pytest.mark.parametrize("mangle", ["absent", "null", "blank", "whitespace"])
    def test_update_refuses_an_absent_summary_before_writing(self, mangle):
        entry = _entry()
        if mangle == "absent":
            entry.pop("summary", None)
        else:
            entry["summary"] = {"null": None, "blank": "", "whitespace": "   "}[mangle]
        _, put_route = _routes(entry)

        with pytest.raises(ApiError, match=r'field "summary" is required'):
            _sync_schedules().update_entry(entry_id=5001, description="<div>New body.</div>")

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("field", ["starts_at", "ends_at"])
    @pytest.mark.parametrize("mangle", ["absent", "null", "blank"])
    def test_update_refuses_an_absent_time_before_writing(self, field, mangle):
        entry = _entry()
        if mangle == "absent":
            entry.pop(field, None)
        else:
            entry[field] = {"null": None, "blank": ""}[mangle]
        _, put_route = _routes(entry)

        with pytest.raises(ApiError, match=rf'field "{field}" is required'):
            _sync_schedules().update_entry(entry_id=5001, summary="New summary")

        assert not put_route.called
        assert respx.calls.call_count == 1

    # `all_day` is NOT NULL DEFAULT false and every partial emits it, so absent
    # or null is malformed. This is the guard that cannot be a truthiness test:
    # False is the value it most needs to admit, so `body.get("all_day") or
    # False` reports the same answer for "it is false" and "it is missing" —
    # and silently converts an all-day event into a midnight-to-midnight timed
    # one on a call that only changed the summary.
    @respx.mock
    @pytest.mark.parametrize("mangle", ["absent", "null"])
    def test_update_refuses_a_missing_all_day_before_writing(self, mangle):
        entry = _entry()
        if mangle == "absent":
            entry.pop("all_day", None)
        else:
            entry["all_day"] = None
        _, put_route = _routes(entry)

        with pytest.raises(ApiError, match=r'field "all_day" is required'):
            _sync_schedules().update_entry(entry_id=5001, summary="New summary")

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize(
        "value",
        [
            pytest.param("true", id="string-true"),
            pytest.param("", id="empty-string"),
            pytest.param(1, id="one"),
            pytest.param(0, id="zero"),
            pytest.param([], id="list"),
        ],
    )
    def test_update_refuses_a_non_boolean_all_day_before_writing(self, value):
        # 0/1 are refused rather than coerced: JSON has a boolean type and the
        # server uses it.
        _, put_route = _routes(_entry(all_day=value))

        with pytest.raises(ApiError, match=r"field 'all_day' is not a boolean"):
            _sync_schedules().update_entry(entry_id=5001, summary="New summary")

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("mangle", ["absent", "blank", "wrong-type"])
    def test_edit_refuses_a_malformed_summary_before_writing(self, mangle):
        entry = _entry()
        if mangle == "absent":
            entry.pop("summary", None)
        else:
            entry["summary"] = {"blank": "", "wrong-type": 42}[mangle]
        _, put_route = _routes(entry)

        with pytest.raises(ApiError), _sync_schedules().edit_entry(entry_id=5001) as e:
            e.description = "<div>New body.</div>"

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    def test_edit_refuses_a_missing_all_day_before_writing(self):
        entry = _entry()
        entry.pop("all_day", None)
        _, put_route = _routes(entry)

        with (
            pytest.raises(ApiError, match=r'field "all_day" is required'),
            _sync_schedules().edit_entry(entry_id=5001) as e,
        ):
            e.summary = "New summary"

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize(
        "participants",
        [
            pytest.param("nobody", id="string"),
            pytest.param([42], id="non-object-element"),
            pytest.param([{"name": "no id"}], id="missing-id"),
            pytest.param([{"id": "1049715914"}], id="string-id"),
            pytest.param([{"id": True}], id="bool-id"),
        ],
    )
    def test_edit_refuses_malformed_participants_before_writing(self, participants):
        _, put_route = _routes(_entry(participants=participants))

        with pytest.raises(ApiError), _sync_schedules().edit_entry(entry_id=5001) as e:
            e.summary = "New summary"

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("value", [42, {"href": "x"}, True])
    def test_edit_refuses_a_non_string_join_url_before_writing(self, value):
        # Seeded for reading, but `e.url = e.url` is a write, so the seed is
        # guarded like everything else that can reach the PUT.
        _, put_route = _routes(_entry(join_url=value))

        with pytest.raises(ApiError), _sync_schedules().edit_entry(entry_id=5001) as e:
            e.summary = "New summary"

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("value", ["yes", 1, []])
    def test_edit_refuses_a_non_boolean_highlighted_before_writing(self, value):
        _, put_route = _routes(_entry(highlighted=value))

        with pytest.raises(ApiError), _sync_schedules().edit_entry(entry_id=5001) as e:
            e.summary = "New summary"

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("raw", [b"[]", b'"entry"', b"42", b"null", b"true"])
    def test_update_refuses_a_non_object_response_before_writing(self, raw):
        # One level up from the field guards: a successful GET can return a
        # scalar, a list or null, and `body.get(key)` would raise a raw
        # AttributeError instead of the documented statusless api_error. A
        # recurring entry's 302 lands here too.
        get_route = respx.get(ENTRY_URL).mock(
            return_value=httpx.Response(200, content=raw, headers={"Content-Type": "application/json"})
        )
        put_route = respx.put(ENTRY_URL).mock(return_value=httpx.Response(200, json=_entry()))

        with pytest.raises(ApiError) as excinfo:
            _sync_schedules().update_entry(entry_id=5001, summary="New summary")

        assert "GetScheduleEntry returned" in str(excinfo.value)
        assert excinfo.value.code == "api_error"
        assert get_route.called
        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("raw", [b"[]", b"null"])
    def test_edit_refuses_a_non_object_response_before_writing(self, raw):
        respx.get(ENTRY_URL).mock(
            return_value=httpx.Response(200, content=raw, headers={"Content-Type": "application/json"})
        )
        put_route = respx.put(ENTRY_URL).mock(return_value=httpx.Response(200, json=_entry()))

        with pytest.raises(ApiError), _sync_schedules().edit_entry(entry_id=5001) as e:
            e.summary = "New summary"

        assert not put_route.called
        assert respx.calls.call_count == 1


# --- async twins -------------------------------------------------------------


class TestAsyncUpdate:
    @respx.mock
    @pytest.mark.asyncio
    async def test_summary_only_update_preserves_the_other_four(self):
        get_route, put_route = _routes()

        result = await _async_schedules().update_entry(entry_id=5001, summary="Team Meeting & Kickoff")

        assert result["id"] == 5001
        assert get_route.called
        body = _put_body(put_route)
        assert body["summary"] == "Team Meeting & Kickoff"
        assert body["starts_at"] == "2026-06-05T06:00:00Z"
        assert body["ends_at"] == "2026-06-05T08:30:00Z"
        assert body["description"] == "<div>Agenda in the doc.</div>"
        assert body["all_day"] is False

    @respx.mock
    @pytest.mark.asyncio
    async def test_a_populated_read_back_is_never_echoed(self):
        _, put_route = _routes()

        await _async_schedules().update_entry(entry_id=5001, summary="Team Meeting & Kickoff")

        body = _put_body(put_route)
        assert "url" not in body
        assert "highlighted" not in body
        assert "participant_ids" not in body

    @respx.mock
    @pytest.mark.asyncio
    async def test_explicit_empties_clear_each_carve_out(self):
        _, put_route = _routes()

        await _async_schedules().update_entry(entry_id=5001, url="", highlighted=False, participant_ids=[])

        body = _put_body(put_route)
        assert body["url"] == ""
        assert body["highlighted"] is False
        assert body["participant_ids"] == []

    @respx.mock
    @pytest.mark.asyncio
    async def test_refuses_a_missing_all_day_before_writing(self):
        entry = _entry()
        entry.pop("all_day", None)
        _, put_route = _routes(entry)

        with pytest.raises(ApiError, match=r'field "all_day" is required'):
            await _async_schedules().update_entry(entry_id=5001, summary="New summary")

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.asyncio
    async def test_hooks_observe_get_entry_then_replace_entry(self):
        _routes()

        hooks = _RecordingHooks()
        await _async_schedules(hooks).update_entry(entry_id=5001, summary="observed")

        assert hooks.operations == ["schedules.get_entry", "schedules.replace_entry"]


class TestAsyncEdit:
    @respx.mock
    @pytest.mark.asyncio
    async def test_edit_puts_full_state_back(self):
        _, put_route = _routes()

        async with _async_schedules().edit_entry(entry_id=5001) as e:
            assert e.description == "<div>Agenda in the doc.</div>"
            e.summary = f"🚨 {e.summary}"

        assert e.result["id"] == 5001
        body = _put_body(put_route)
        assert body["summary"] == "🚨 Team Meeting"
        assert body["all_day"] is False

    @respx.mock
    @pytest.mark.asyncio
    async def test_untouched_carve_outs_stay_off_the_wire(self):
        _, put_route = _routes()

        async with _async_schedules().edit_entry(entry_id=5001) as e:
            assert e.url == "https://meet.example.com/team"
            e.summary = "Team Sync"

        body = _put_body(put_route)
        assert "url" not in body
        assert "highlighted" not in body
        assert "participant_ids" not in body

    @respx.mock
    @pytest.mark.asyncio
    async def test_assigning_the_read_backs_own_value_still_sends_it(self):
        _, put_route = _routes()

        async with _async_schedules().edit_entry(entry_id=5001) as e:
            e.url = e.url
            e.highlighted = e.highlighted

        body = _put_body(put_route)
        assert body["url"] == "https://meet.example.com/team"
        assert body["highlighted"] is True

    @respx.mock
    @pytest.mark.asyncio
    async def test_exception_aborts_without_put(self):
        _, put_route = _routes()

        with pytest.raises(RuntimeError, match="abort"):
            async with _async_schedules().edit_entry(entry_id=5001) as e:
                e.summary = "never written"
                raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    @pytest.mark.asyncio
    async def test_result_raises_before_completion(self):
        _routes()

        edit = _async_schedules().edit_entry(entry_id=5001)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result

    @respx.mock
    @pytest.mark.asyncio
    async def test_refuses_a_malformed_summary_before_writing(self):
        _, put_route = _routes(_entry(summary=""))

        with pytest.raises(ApiError):
            async with _async_schedules().edit_entry(entry_id=5001) as e:
                e.summary = e.summary

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.asyncio
    async def test_hooks_observe_get_entry_then_replace_entry(self):
        _routes()

        hooks = _RecordingHooks()
        async with _async_schedules(hooks).edit_entry(entry_id=5001) as e:
            e.summary = "observed"

        assert hooks.operations == ["schedules.get_entry", "schedules.replace_entry"]


class TestAsyncReplace:
    @respx.mock
    @pytest.mark.asyncio
    async def test_sparse_replace_issues_no_get(self):
        get_route, put_route = _routes()

        await _async_schedules().replace_entry(
            entry_id=5001,
            summary="verbatim",
            starts_at="2026-06-05T06:00:00Z",
            ends_at="2026-06-05T08:30:00Z",
        )

        assert not get_route.called
        assert respx.calls.call_count == 1
        body = _put_body(put_route)
        assert body["summary"] == "verbatim"
        assert "url" not in body
