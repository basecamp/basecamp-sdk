"""Tests for the todolist reposition and update/edit/replace surfaces (sync + async)."""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ApiError, NotFoundError, UsageError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"


def _todolist(name: str) -> dict:
    """Minimal Todolist payload for the metadata/hook tests.

    These tests are about operation identity and resource-id scoping, not
    payload shape — but a stub must not contradict the contract either.
    bubble_up_url is @required on Todolist (todolists/_todolist.json.jbuilder
    always passes bubbleupable: true), so it belongs here even though nothing
    below reads it. color and comments_app_url are @required for the same
    reason (#630): the jbuilder emits color in both branches of its
    todolist_group? conditional and comments_app_url from a route helper.
    Python's TypedDict does not decode strictly, so leaving them out would be
    invisible here and still a body BC3 cannot produce. Full-shape coverage
    lives in spec/fixtures, which `make check-fixture-coverage` validates.
    """
    return {
        "id": 2,
        "name": name,
        "description_attachments": [],
        "bubble_up_url": "https://3.basecampapi.com/12345/buckets/1/recordings/2/bubble_up.json",
        "color": "blue",
        "comments_app_url": "https://3.basecamp.com/12345/buckets/1/recordings/2/comments",
    }


def _todolist_full(**overrides) -> dict:
    """Full validated Todolist shape, with the write-path values under test.

    Sourced from the same fixture `make check-fixture-coverage` validates, so
    every required field is present; the overrides are only the values the
    body assertions trace through the GET → PUT round trip.
    """
    return {
        **json.loads((_FIXTURES / "todolists" / "get.json").read_text(encoding="utf-8")),
        "id": 2,
        "name": "Hardware",
        "title": "Hardware",
        "description": "<p>Ship the hardware</p>",
        **overrides,
    }


def _group_full(**overrides) -> dict:
    """Full validated group shape, from the group fixture.

    A group is a Todolist. BC3 has no group model — todolists/groups/{index,
    show}.json.jbuilder render todolists/_todolist.json.jbuilder — so the group
    fixture carries the same `description`/`description_attachments` a list
    does and reports `"type": "Todolist"`. Since #544 there is one declared
    shape for both, and the variants are told apart structurally:
    `group_position_url` here stands in for the list's `groups_url`. Never the
    type string.
    """
    return {
        **json.loads((_FIXTURES / "todolist_groups" / "get.json").read_text(encoding="utf-8")),
        **overrides,
    }


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[OperationInfo] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(info)


def _operation_names(hooks: _RecordingHooks) -> list[str]:
    return [f"{info.service}.{info.operation}" for info in hooks.operations]


def _put_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


def _sync_todolists(hooks: BasecampHooks | None = None):
    return Client(access_token="test-token", hooks=hooks).for_account("12345").todolists


def _async_todolists(hooks: BasecampHooks | None = None):
    return AsyncClient(access_token="test-token", hooks=hooks).for_account("12345").todolists


class TestSyncReposition:
    @respx.mock
    def test_reposition_sends_position(self):
        route = respx.put(f"{BASE}/todosets/todolists/42/position.json").mock(return_value=httpx.Response(204))

        result = _sync_todolists().reposition(todolist_id=42, position=3)

        assert result is None
        assert route.called
        assert _put_body(route)["position"] == 3

    @respx.mock
    def test_reposition_not_found(self):
        respx.put(f"{BASE}/todosets/todolists/999/position.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _sync_todolists().reposition(todolist_id=999, position=1)


class TestAsyncReposition:
    @pytest.mark.asyncio
    @respx.mock
    async def test_reposition_sends_position(self):
        route = respx.put(f"{BASE}/todosets/todolists/42/position.json").mock(return_value=httpx.Response(204))

        result = await _async_todolists().reposition(todolist_id=42, position=3)

        assert result is None
        assert route.called
        assert _put_body(route)["position"] == 3

    @pytest.mark.asyncio
    @respx.mock
    async def test_reposition_not_found(self):
        respx.put(f"{BASE}/todosets/todolists/999/position.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            await _async_todolists().reposition(todolist_id=999, position=1)


# The get/replace path label is the unsuffixed ``{id}``. resource_id must still
# carry it (predicate is ``endswith("Id") or == "id"``); a suffix-only
# regression would silently drop resource_id here.
class TestSyncTodolistMetadata:
    @respx.mock
    def test_get_scopes_resource_to_todolist_id(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist("Sprint Tasks")))

        hooks = _RecordingHooks()
        c = Client(access_token="test-token", hooks=hooks)
        c.for_account("12345").todolists.get(id=2)
        c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "todolists"
        assert info.operation == "get"
        assert info.resource_id == 2

    @respx.mock
    def test_replace_scopes_resource_to_todolist_id(self):
        respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist("Updated List")))

        hooks = _RecordingHooks()
        c = Client(access_token="test-token", hooks=hooks)
        c.for_account("12345").todolists.replace(id=2, name="Updated List")
        c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "todolists"
        assert info.operation == "replace"
        assert info.resource_id == 2


class TestAsyncTodolistMetadata:
    @pytest.mark.asyncio
    @respx.mock
    async def test_get_scopes_resource_to_todolist_id(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist("Sprint Tasks")))

        hooks = _RecordingHooks()
        c = AsyncClient(access_token="test-token", hooks=hooks)
        await c.for_account("12345").todolists.get(id=2)
        await c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "todolists"
        assert info.operation == "get"
        assert info.resource_id == 2

    @pytest.mark.asyncio
    @respx.mock
    async def test_replace_scopes_resource_to_todolist_id(self):
        respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist("Updated List")))

        hooks = _RecordingHooks()
        c = AsyncClient(access_token="test-token", hooks=hooks)
        await c.for_account("12345").todolists.replace(id=2, name="Updated List")
        await c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "todolists"
        assert info.operation == "replace"
        assert info.resource_id == 2


class TestSyncGroupRead:
    """`todolists.get` answers for a group too, through the one flat shape (#544).

    Before #544 the spec declared three shapes for this one wire body, and the
    group arm modelled no `description` at all — so a group read through it lost
    the field outright. There is now a single `Todolist`, and the group route is
    the same route: `todolists/groups/show.json.jbuilder` renders
    `todolists/_todolist.json.jbuilder`.
    """

    @respx.mock
    def test_get_returns_a_group_with_its_description_and_position_url(self):
        group = _group_full()
        respx.get(f"{BASE}/todolists/7").mock(return_value=httpx.Response(200, json=group))

        result = _sync_todolists().get(id=7)

        # The field the pre-#544 group projection had nowhere to put.
        assert result["description"] == "<div>Phase one hardware work</div>"
        assert result["description_attachments"] == []
        assert result["name"] == "Phase 1"
        # Discrimination is structural, never the type string: a group reports
        # "Todolist" like any list and is told apart by which URL it carries.
        assert result["type"] == "Todolist"
        assert result["group_position_url"] == group["group_position_url"]
        assert "groups_url" not in result

    @respx.mock
    def test_get_returns_a_list_with_groups_url_and_no_group_position_url(self):
        """The other side of the XOR, so the discriminator is pinned both ways."""
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        result = _sync_todolists().get(id=2)

        assert result["type"] == "Todolist"
        assert result["groups_url"]
        assert "group_position_url" not in result
        assert result["description"] == "<p>Ship the hardware</p>"


class TestAsyncGroupRead:
    @pytest.mark.asyncio
    @respx.mock
    async def test_get_returns_a_group_with_its_description_and_position_url(self):
        group = _group_full()
        respx.get(f"{BASE}/todolists/7").mock(return_value=httpx.Response(200, json=group))

        result = await _async_todolists().get(id=7)

        assert result["description"] == "<div>Phase one hardware work</div>"
        assert result["description_attachments"] == []
        assert result["name"] == "Phase 1"
        assert result["type"] == "Todolist"
        assert result["group_position_url"] == group["group_position_url"]
        assert "groups_url" not in result


class TestMalformedWritableFields:
    """A malformed writable field must abort before the PUT, never be coerced.

    Python has no typed decoder between the GET and the field read, so a plain
    ``or ""`` turns every falsey non-string into ``""``. Because this endpoint
    is full-replace, that value is then written back over the real one — the
    composite erases the field it exists to preserve, on a call that never
    mentioned it. Truthy non-strings are just as wrong: they reach the wire
    verbatim. The shipped Todos/Cards analogue takes the same refusal from
    the ``_merge_safe`` guards #576 closed with.
    """

    @pytest.mark.parametrize("malformed", [False, 0, [], {}, 42, True, ["x"], {"a": 1}])
    @respx.mock
    def test_update_refuses_a_malformed_description(self, malformed):
        get_route = respx.get(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=malformed))
        )
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(ApiError) as excinfo:
            _sync_todolists().update(id=2, name="Renamed list")

        assert "'description' is not a string" in str(excinfo.value)
        assert get_route.call_count == 1
        assert put_route.call_count == 0, "the PUT must never be issued for a malformed description"

    @pytest.mark.parametrize("malformed", [False, 0, [], {}, 42])
    @respx.mock
    def test_edit_refuses_a_malformed_name(self, malformed):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full(name=malformed)))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(ApiError) as excinfo, _sync_todolists().edit(id=2) as tl:
            tl.description = "<p>New</p>"

        assert "'name' is not a string" in str(excinfo.value)
        assert put_route.call_count == 0, "the PUT must never be issued for a malformed name"

    @pytest.mark.parametrize(
        "body",
        [
            {"id": 2},
            {"id": 2, "name": None},
            {"id": 2, "name": ""},
        ],
    )
    @respx.mock
    def test_absent_null_or_empty_name_is_a_malformed_response(self, body):
        """`name` is required and presence-validated, so all three are malformed.

        Classification is by ORIGIN: this name came off the wire, so it is an
        ApiError. The caller supplying an empty name is a UsageError, asserted
        separately. Before this, absent/null collapsed to "" and that empty
        name was PUT over the real one.
        """
        body = {**body, "description": "<p>Ship the hardware</p>"}
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=body))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(ApiError) as excinfo:
            _sync_todolists().update(id=2, description="<p>New</p>")

        assert "'name'" in str(excinfo.value)
        assert get_route.call_count == 1
        assert put_route.call_count == 0, "a malformed name must never reach the PUT"

    @pytest.mark.parametrize("body", [42, "nope", None, ["a"], True])
    @respx.mock
    def test_refuses_a_non_object_response_body(self, body):
        """Row 9: the malformed envelope, one level up from malformed fields."""
        # json=None emits an EMPTY body, which fails transport decode before the
        # guard ever runs; send the JSON literal so the guard is what is tested.
        respx.get(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, content=json.dumps(body), headers={"content-type": "application/json"})
        )
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(ApiError) as excinfo:
            _sync_todolists().update(id=2, name="Renamed")

        assert "where a todolist object was expected" in str(excinfo.value)
        assert put_route.call_count == 0

    @pytest.mark.parametrize(
        "body",
        [
            {"todolist": None},
            {"todolist": 42},
            {"todolist": ["a"]},
            {"group": None},
            {"group": "nope"},
        ],
    )
    @respx.mock
    def test_refuses_an_enveloped_body_rather_than_unwrapping_it(self, body):
        """The arm layer is gone with the union (#544); the outcome is not.

        These bodies used to be caught one level down, by an arm guard that
        existed only because the spec declared a `TodolistOrGroup` oneOf. With
        one flat shape there is no arm to guard and nothing unwraps: an
        enveloped body is simply a body carrying no `name`, which the
        required-field guard refuses. Two levels now — body then scalar — where
        there were three. What must not change is that the read fails and the
        full-replace PUT is never issued.
        """
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=body))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(ApiError) as excinfo:
            _sync_todolists().update(id=2, name="Renamed")

        assert "'name' is missing from the response" in str(excinfo.value)
        assert put_route.call_count == 0

    @respx.mock
    def test_reports_an_unrenderable_caller_value_without_losing_the_diagnosis(self):
        """Row 10: the guard's own error path must not throw. repr is user code."""

        class Hostile:
            def __repr__(self):
                raise RuntimeError("nope")

        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(UsageError) as excinfo, _sync_todolists().edit(id=2) as tl:
            tl.description = Hostile()

        assert "Hostile" in str(excinfo.value), "the type name survives even when repr fails"
        assert put_route.call_count == 0

    @respx.mock
    def test_malformed_message_is_capped(self):
        """SPEC section 9 caps error messages; the value is embedded in them."""
        huge = ["x"] * 50_000
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full(description=huge)))

        with pytest.raises(ApiError) as excinfo:
            _sync_todolists().update(id=2, name="Renamed")

        assert len(str(excinfo.value).encode()) <= 500

    @pytest.mark.parametrize("bad", [42, True, [], {}, ["x"], {"a": 1}, 0, False])
    @respx.mock
    def test_edit_refuses_a_caller_supplied_non_string(self, bad):
        """The mirror of the read step: caller provenance, so UsageError.

        `edit` hands the caller a mutable view of the full writable state and
        Python enforces nothing about what comes back, so a closure assigning a
        non-string would otherwise walk straight into the full-replace PUT.
        """
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(UsageError) as excinfo, _sync_todolists().edit(id=2) as tl:
            tl.description = bad

        assert "must be a string" in str(excinfo.value)
        assert put_route.call_count == 0, "a non-string must never reach the PUT"

    @respx.mock
    def test_caller_supplied_empty_name_is_a_usage_error(self):
        """The mirror case: same value, caller origin, so UsageError not ApiError."""
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(UsageError):
            _sync_todolists().update(id=2, name="")

        assert put_route.call_count == 0

    @pytest.mark.parametrize(
        ("label", "body"),
        [
            ("absent", {k: v for k, v in _todolist_full().items() if k != "description"}),
            ("null", _todolist_full(description=None)),
        ],
    )
    @respx.mock
    def test_absent_and_null_description_are_malformed_responses(self, label, body):
        """`description` is @required and never null, so both are malformed.

        BC3's ``format_api_content`` funnels a blank rich text through
        ``call_pipeline``, which returns ``""`` rather than nil, so a
        description-less list still carries the key. Reading an absent or null
        one as ``""`` was the data-loss case this composite exists to remove:
        the full-replace PUT then wrote that ``""`` over the record's real
        description on a call that only renamed it. Refuse before the PUT.
        """
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=body))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(ApiError) as excinfo:
            _sync_todolists().update(id=2, name="Renamed list")

        expected = "is missing from the response" if label == "absent" else "is null in the response"
        assert f"'description' {expected}" in str(excinfo.value)
        assert get_route.call_count == 1
        assert put_route.call_count == 0, "an absent or null description must never reach the PUT"

    @pytest.mark.parametrize(
        ("label", "body"),
        [
            ("absent", {k: v for k, v in _todolist_full().items() if k != "description"}),
            ("null", _todolist_full(description=None)),
        ],
    )
    @respx.mock
    def test_edit_refuses_an_absent_or_null_description_before_the_block(self, label, body):
        """The same via ``edit``, the path that hands the value to caller code."""
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=body))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        entered = False
        with pytest.raises(ApiError), _sync_todolists().edit(id=2) as tl:
            entered = True
            tl.name = "Renamed list"

        assert not entered, "the edit block must not run on a malformed response"
        assert put_route.call_count == 0, "an absent or null description must never reach the PUT"

    @respx.mock
    def test_present_and_empty_description_round_trips(self):
        """The case the refusals must not swallow, and by far the common one.

        A description-less list carries a present-and-empty description. ``""``
        is a real value, so it round-trips and reaches the PUT; refusing it
        would break every list without a description.
        """
        get_route = respx.get(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=""))
        )
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=""))
        )

        _sync_todolists().update(id=2, name="Renamed list")

        assert get_route.call_count == 1
        assert put_route.call_count == 1
        assert _put_body(put_route) == {"name": "Renamed list", "description": ""}


class TestSyncUpdate:
    @respx.mock
    def test_name_only_update_preserves_the_description(self):
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(name="Renamed list"))
        )

        result = _sync_todolists().update(id=2, name="Renamed list")

        assert result["name"] == "Renamed list"
        assert get_route.call_count == 1
        assert put_route.call_count == 1
        # BC3 rebuilds the todolist from the permitted params, so the sparse
        # PUT this replaces would have erased the description outright.
        assert _put_body(put_route) == {"name": "Renamed list", "description": "<p>Ship the hardware</p>"}

    @respx.mock
    def test_description_only_update_preserves_the_name(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        _sync_todolists().update(id=2, description="<p>New plan</p>")

        assert _put_body(put_route) == {"name": "Hardware", "description": "<p>New plan</p>"}

    @respx.mock
    def test_explicit_empty_description_clears(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=""))
        )

        _sync_todolists().update(id=2, description="")

        # Present-and-empty, never JSON null and never absent: the generated
        # layer's _compact strips None, which would read as "clear" only by
        # accident (SPEC §18 body compaction).
        body = _put_body(put_route)
        assert body["description"] == ""
        assert body["name"] == "Hardware"

    @respx.mock
    def test_already_empty_description_still_goes_out(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full(description="")))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=""))
        )

        _sync_todolists().update(id=2, name="Renamed list")

        assert _put_body(put_route) == {"name": "Renamed list", "description": ""}

    @respx.mock
    def test_enveloped_get_response_is_refused_without_writing(self):
        # BC3 answers this route with the flat recordable JSON and, since #544,
        # so does the spec: there is no envelope and nothing unwraps one. A
        # well-formed envelope is therefore a body with no readable `name`, and
        # refusing it is the whole contract — the composite must not degrade
        # into writing an empty name over the real one.
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json={"todolist": _todolist_full()}))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(ApiError) as excinfo:
            _sync_todolists().update(id=2, description="<p>New plan</p>")

        assert "'name' is missing from the response" in str(excinfo.value)
        assert put_route.call_count == 0

    @respx.mock
    def test_a_nested_group_object_is_ordinary_data(self):
        # The unwrap that could once have hijacked a flat body went out with the
        # union (#544). This pins that its removal left a nested dict keyed
        # "group" exactly where it was: data on the record that no code path
        # reads, with the top-level name/description preserved.
        respx.get(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(
                200, json=_todolist_full(group={"id": 9, "name": "Decoy", "description": "<p>Decoy</p>"})
            )
        )
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        _sync_todolists().update(id=2, name="Renamed list")

        assert _put_body(put_route) == {"name": "Renamed list", "description": "<p>Ship the hardware</p>"}

    @respx.mock
    def test_group_variant_is_preserved_without_type_sniffing(self):
        # The same URI addresses a to-do list or a group inside one; BC3 renders
        # both through todolists/_todolist.json.jbuilder, so a group reports
        # "type": "Todolist" and differs only by group_position_url standing in
        # for groups_url. Read from the group fixture, whose description is real
        # data a group-only projection would have had nowhere to put — the
        # composite must resend it, not drop it.
        group = _group_full()
        assert group["type"] == "Todolist", "the type string is identical, which is why nothing branches on it"
        assert "groups_url" not in group and group["group_position_url"]
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=group))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json={**group, "name": "Renamed group"})
        )

        _sync_todolists().update(id=2, name="Renamed group")

        assert _put_body(put_route) == {
            "name": "Renamed group",
            "description": "<div>Phase one hardware work</div>",
        }

    @respx.mock
    def test_empty_name_raises_usage_error_without_put(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(UsageError, match=r"name must be a non-empty string") as excinfo:
            _sync_todolists().update(id=2, name="")

        assert "BC3 presence-validates it" in str(excinfo.value)
        assert not put_route.called

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        hooks = _RecordingHooks()
        _sync_todolists(hooks).update(id=2, name="Renamed list")

        assert _operation_names(hooks) == ["todolists.get", "todolists.replace"]


class TestSyncEdit:
    @respx.mock
    def test_edit_clears_the_description_and_keeps_the_name(self):
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=""))
        )

        with _sync_todolists().edit(id=2) as tl:
            assert tl.name == "Hardware"
            assert tl.description == "<p>Ship the hardware</p>"
            tl.description = ""

        assert get_route.call_count == 1
        assert put_route.call_count == 1
        assert _put_body(put_route) == {"name": "Hardware", "description": ""}
        assert tl.result["description"] == ""

    @respx.mock
    def test_edit_renames_and_resends_the_description(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with _sync_todolists().edit(id=2) as tl:
            tl.name = f"🚨 {tl.name}"

        assert _put_body(put_route) == {"name": "🚨 Hardware", "description": "<p>Ship the hardware</p>"}

    @respx.mock
    def test_exception_aborts_without_put(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(RuntimeError, match="abort"), _sync_todolists().edit(id=2) as tl:
            tl.name = "never written"
            raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    def test_empty_name_raises_usage_error_without_put(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with (
            pytest.raises(UsageError, match=r"name must be a non-empty string"),
            _sync_todolists().edit(id=2) as tl,
        ):
            tl.name = ""

        assert not put_route.called

    @respx.mock
    def test_result_raises_before_completion(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        edit = _sync_todolists().edit(id=2)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        hooks = _RecordingHooks()
        with _sync_todolists(hooks).edit(id=2) as tl:
            tl.name = "observed"

        assert _operation_names(hooks) == ["todolists.get", "todolists.replace"]


class TestSyncReplace:
    @respx.mock
    def test_sparse_replace_issues_no_get_and_omits_the_description(self):
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(name="The whole new list", description=""))
        )

        result = _sync_todolists().replace(id=2, name="The whole new list")

        assert result["name"] == "The whole new list"
        assert not get_route.called
        assert put_route.call_count == 1
        # Verbatim: what you omit, the server clears. That destructiveness is
        # the point of the name.
        assert _put_body(put_route) == {"name": "The whole new list"}


class TestAsyncUpdate:
    @pytest.mark.asyncio
    @respx.mock
    async def test_name_only_update_preserves_the_description(self):
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(name="Renamed list"))
        )

        result = await _async_todolists().update(id=2, name="Renamed list")

        assert result["name"] == "Renamed list"
        assert get_route.call_count == 1
        assert put_route.call_count == 1
        assert _put_body(put_route) == {"name": "Renamed list", "description": "<p>Ship the hardware</p>"}

    @pytest.mark.asyncio
    @respx.mock
    async def test_explicit_empty_description_clears(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=""))
        )

        await _async_todolists().update(id=2, description="")

        assert _put_body(put_route) == {"name": "Hardware", "description": ""}

    @pytest.mark.asyncio
    @respx.mock
    async def test_empty_name_raises_usage_error_without_put(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(UsageError, match=r"name must be a non-empty string") as excinfo:
            await _async_todolists().update(id=2, name="")

        assert "BC3 presence-validates it" in str(excinfo.value)
        assert not put_route.called

    @pytest.mark.asyncio
    @respx.mock
    async def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        hooks = _RecordingHooks()
        await _async_todolists(hooks).update(id=2, name="Renamed list")

        assert _operation_names(hooks) == ["todolists.get", "todolists.replace"]


class TestAsyncEdit:
    @pytest.mark.asyncio
    @respx.mock
    async def test_edit_clears_the_description_and_keeps_the_name(self):
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(description=""))
        )

        async with _async_todolists().edit(id=2) as tl:
            assert tl.name == "Hardware"
            tl.description = ""

        assert get_route.call_count == 1
        assert put_route.call_count == 1
        assert _put_body(put_route) == {"name": "Hardware", "description": ""}
        assert tl.result["description"] == ""

    @pytest.mark.asyncio
    @respx.mock
    async def test_exception_aborts_without_put(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(RuntimeError, match="abort"):
            async with _async_todolists().edit(id=2) as tl:
                tl.name = "never written"
                raise RuntimeError("abort")

        assert not put_route.called

    @pytest.mark.asyncio
    @respx.mock
    async def test_empty_name_raises_usage_error_without_put(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        with pytest.raises(UsageError, match=r"name must be a non-empty string"):
            async with _async_todolists().edit(id=2) as tl:
                tl.name = ""

        assert not put_route.called

    @pytest.mark.asyncio
    @respx.mock
    async def test_result_raises_before_completion(self):
        respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))

        edit = _async_todolists().edit(id=2)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result


class TestAsyncReplace:
    @pytest.mark.asyncio
    @respx.mock
    async def test_sparse_replace_issues_no_get_and_omits_the_description(self):
        get_route = respx.get(f"{BASE}/todolists/2").mock(return_value=httpx.Response(200, json=_todolist_full()))
        put_route = respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json=_todolist_full(name="The whole new list", description=""))
        )

        result = await _async_todolists().replace(id=2, name="The whole new list")

        assert result["name"] == "The whole new list"
        assert not get_route.called
        assert put_route.call_count == 1
        assert _put_body(put_route) == {"name": "The whole new list"}
