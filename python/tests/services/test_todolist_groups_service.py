"""Tests for the todolist-groups service (sync + async).

A group is a Todolist. BC3 has no group model and no group write route:
`todolists/groups/{index,show}.json.jbuilder` render
`todolists/_todolist.json.jbuilder`, and a group is just a `Todolist` whose
parent is a `Todolist`. Since #544 the spec says the same thing — `Todolist`,
`TodolistGroup` and the `TodolistOrGroup` union are one flat structure — so
`ListTodolistGroups` carries the full `Todolist` shape rather than a group-only
projection that modelled no `description`.

The bodies here come from `spec/fixtures/todolist_groups/*.json`, the fixtures
`make check-fixture-coverage` validates, so what is asserted is the shape the
API actually returns rather than a stub that agrees with the assertions.

Note what these tests do NOT establish: Python's generated methods return
`dict[str, Any]`, so this SDK parses JSON and hands it back with no decode step
of its own. The flat shape changes what the API returns, not what Python
validates — giving Python a structural decoder is #578.
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import NotFoundError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"


def _group_list() -> list[dict]:
    """The shipped group-list fixture: two groups in the flat Todolist shape."""
    return json.loads((_FIXTURES / "todolist_groups" / "list.json").read_text(encoding="utf-8"))


def _group_get() -> dict:
    return json.loads((_FIXTURES / "todolist_groups" / "get.json").read_text(encoding="utf-8"))


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[OperationInfo] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(info)


def _sync_groups(hooks: BasecampHooks | None = None):
    return Client(access_token="test-token", hooks=hooks).for_account("12345").todolist_groups


def _async_groups(hooks: BasecampHooks | None = None):
    return AsyncClient(access_token="test-token", hooks=hooks).for_account("12345").todolist_groups


class TestSyncList:
    @respx.mock
    def test_list_returns_the_flat_todolist_shape(self):
        respx.get(f"{BASE}/todolists/2/groups.json").mock(return_value=httpx.Response(200, json=_group_list()))

        result = _sync_groups().list(todolist_id=2)

        assert len(result) == 2
        first = result[0]
        # The field a group-only projection never modelled, so every group read
        # through it came back without one (#544).
        assert first["description"] == "<div>Phase one hardware work</div>"
        assert first["description_attachments"] == []
        assert first["name"] == "Phase 1"
        # Discrimination is structural. Both variants report "Todolist", so the
        # type string is exactly what nothing may branch on; the group is the
        # one carrying group_position_url instead of groups_url.
        assert first["type"] == "Todolist"
        assert first["group_position_url"].endswith("/todolists/groups/1069479600/position.json")
        assert "groups_url" not in first
        # A group's description can legitimately be empty; that is a value, not
        # an absence, and it survives the read as one.
        assert result[1]["description"] == ""

    @respx.mock
    def test_list_scopes_the_resource_to_the_parent_todolist(self):
        respx.get(f"{BASE}/todolists/2/groups.json").mock(return_value=httpx.Response(200, json=_group_list()))

        hooks = _RecordingHooks()
        c = Client(access_token="test-token", hooks=hooks)
        c.for_account("12345").todolist_groups.list(todolist_id=2)
        c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "todolistgroups"
        assert info.operation == "list"
        assert info.is_mutation is False
        assert info.resource_id == 2

    @respx.mock
    def test_list_not_found(self):
        respx.get(f"{BASE}/todolists/999/groups.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _sync_groups().list(todolist_id=999)


class TestAsyncList:
    @pytest.mark.asyncio
    @respx.mock
    async def test_list_returns_the_flat_todolist_shape(self):
        respx.get(f"{BASE}/todolists/2/groups.json").mock(return_value=httpx.Response(200, json=_group_list()))

        result = await _async_groups().list(todolist_id=2)

        assert len(result) == 2
        first = result[0]
        assert first["description"] == "<div>Phase one hardware work</div>"
        assert first["name"] == "Phase 1"
        assert first["type"] == "Todolist"
        assert first["group_position_url"].endswith("/todolists/groups/1069479600/position.json")
        assert "groups_url" not in first


class TestSyncCreate:
    @respx.mock
    def test_create_sends_the_name_and_returns_the_group(self):
        route = respx.post(f"{BASE}/todolists/2/groups.json").mock(return_value=httpx.Response(201, json=_group_get()))

        result = _sync_groups().create(todolist_id=2, name="Phase 1")

        assert route.called
        assert json.loads(route.calls[-1].request.content) == {"name": "Phase 1"}
        # Create answers with the same flat shape a read does.
        assert result["type"] == "Todolist"
        assert result["description"] == "<div>Phase one hardware work</div>"
        assert result["group_position_url"]


class TestAsyncCreate:
    @pytest.mark.asyncio
    @respx.mock
    async def test_create_sends_the_name_and_returns_the_group(self):
        route = respx.post(f"{BASE}/todolists/2/groups.json").mock(return_value=httpx.Response(201, json=_group_get()))

        result = await _async_groups().create(todolist_id=2, name="Phase 1")

        assert route.called
        assert json.loads(route.calls[-1].request.content) == {"name": "Phase 1"}
        assert result["description"] == "<div>Phase one hardware work</div>"


class TestSyncReposition:
    @respx.mock
    def test_reposition_sends_position(self):
        route = respx.put(f"{BASE}/todolists/groups/7/position.json").mock(return_value=httpx.Response(204))

        result = _sync_groups().reposition(group_id=7, position=3)

        assert result is None
        assert route.called
        assert json.loads(route.calls[-1].request.content)["position"] == 3


class TestAsyncReposition:
    @pytest.mark.asyncio
    @respx.mock
    async def test_reposition_sends_position(self):
        route = respx.put(f"{BASE}/todolists/groups/7/position.json").mock(return_value=httpx.Response(204))

        result = await _async_groups().reposition(group_id=7, position=3)

        assert result is None
        assert route.called
        assert json.loads(route.calls[-1].request.content)["position"] == 3
