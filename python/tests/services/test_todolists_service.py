"""Tests for the todolist reposition surface (sync + async)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import NotFoundError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[OperationInfo] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(info)


def _put_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


def _sync_todolists():
    return Client(access_token="test-token").for_account("12345").todolists


def _async_todolists():
    return AsyncClient(access_token="test-token").for_account("12345").todolists


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


# The get/update path label is the unsuffixed ``{id}``. resource_id must still
# carry it (predicate is ``endswith("Id") or == "id"``); a suffix-only
# regression would silently drop resource_id here.
class TestSyncTodolistMetadata:
    @respx.mock
    def test_get_scopes_resource_to_todolist_id(self):
        respx.get(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json={"id": 2, "name": "Sprint Tasks", "description_attachments": []})
        )

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
    def test_update_scopes_resource_to_todolist_id(self):
        respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json={"id": 2, "name": "Updated List", "description_attachments": []})
        )

        hooks = _RecordingHooks()
        c = Client(access_token="test-token", hooks=hooks)
        c.for_account("12345").todolists.update(id=2, name="Updated List")
        c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "todolists"
        assert info.operation == "update"
        assert info.resource_id == 2


class TestAsyncTodolistMetadata:
    @pytest.mark.asyncio
    @respx.mock
    async def test_get_scopes_resource_to_todolist_id(self):
        respx.get(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json={"id": 2, "name": "Sprint Tasks", "description_attachments": []})
        )

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
    async def test_update_scopes_resource_to_todolist_id(self):
        respx.put(f"{BASE}/todolists/2").mock(
            return_value=httpx.Response(200, json={"id": 2, "name": "Updated List", "description_attachments": []})
        )

        hooks = _RecordingHooks()
        c = AsyncClient(access_token="test-token", hooks=hooks)
        await c.for_account("12345").todolists.update(id=2, name="Updated List")
        await c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "todolists"
        assert info.operation == "update"
        assert info.resource_id == 2
