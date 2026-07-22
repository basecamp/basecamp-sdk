"""Tests for the todos merge-safe update / edit / replace surface (sync + async)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import UsageError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"


def _todo(todo_id: int = 42, **overrides) -> dict:
    todo = {
        "id": todo_id,
        "content": "Buy milk",
        "description": "<p>From the store</p>",
        "due_on": "2024-03-01",
        "starts_on": "2024-02-01",
        "assignees": [{"id": 100, "name": "Jane Doe"}],
        "completion_subscribers": [{"id": 555, "name": "Sub Scriber"}],
        "completed": False,
    }
    todo.update(overrides)
    return todo


def _put_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[str] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(f"{info.service}.{info.operation}")


def _sync_todos(hooks: BasecampHooks | None = None):
    return Client(access_token="test-token", hooks=hooks).for_account("12345").todos


def _async_todos(hooks: BasecampHooks | None = None):
    return AsyncClient(access_token="test-token", hooks=hooks).for_account("12345").todos


class TestSyncUpdate:
    @respx.mock
    def test_merges_unset_fields(self):
        get_route = respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        result = _sync_todos().update(todo_id=42, content="Updated task")

        assert result["id"] == 42
        assert get_route.called
        body = _put_body(put_route)
        assert body["content"] == "Updated task"
        assert body["description"] == "<p>From the store</p>"
        assert body["due_on"] == "2024-03-01"
        assert body["starts_on"] == "2024-02-01"
        assert body["assignee_ids"] == [100]
        assert body["completion_subscriber_ids"] == [555]
        assert "notify" not in body

    @respx.mock
    def test_explicit_empty_list_clears(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        _sync_todos().update(todo_id=42, assignee_ids=[])

        body = _put_body(put_route)
        assert body["assignee_ids"] == []
        assert body["completion_subscriber_ids"] == [555]
        assert body["content"] == "Buy milk"

    @respx.mock
    def test_notify_only_when_true(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        _sync_todos().update(todo_id=42, content="ping", notify=True)

        assert _put_body(put_route)["notify"] is True

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        hooks = _RecordingHooks()
        _sync_todos(hooks).update(todo_id=42, content="observed")

        assert hooks.operations == ["todos.get", "todos.replace"]


class TestSyncEdit:
    @respx.mock
    def test_edit_puts_full_state_back(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with _sync_todos().edit(todo_id=42) as t:
            assert t.content == "Buy milk"
            t.content = f"🚨 {t.content}"

        assert t.result["id"] == 42
        body = _put_body(put_route)
        assert body["content"] == "🚨 Buy milk"
        assert body["description"] == "<p>From the store</p>"
        assert body["assignee_ids"] == [100]

    @respx.mock
    def test_clear_date_is_omitted_from_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with _sync_todos().edit(todo_id=42) as t:
            assert t.due_on == "2024-03-01"
            t.due_on = ""

        body = _put_body(put_route)
        assert "due_on" not in body
        assert body["content"] == "Buy milk"

    @respx.mock
    def test_clear_description_and_ids_present_and_empty(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with _sync_todos().edit(todo_id=42) as t:
            t.description = ""
            t.assignee_ids = []
            t.completion_subscriber_ids = []

        body = _put_body(put_route)
        assert body["description"] == ""
        assert body["assignee_ids"] == []
        assert body["completion_subscriber_ids"] == []

    @respx.mock
    def test_exception_aborts_without_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with pytest.raises(RuntimeError, match="abort"), _sync_todos().edit(todo_id=42) as t:
            t.content = "never written"
            raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    def test_none_id_list_raises_usage_error_without_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with pytest.raises(UsageError, match=r"use \[\] to clear"), _sync_todos().edit(todo_id=42) as t:
            t.assignee_ids = None

        assert not put_route.called

    @respx.mock
    def test_result_raises_before_completion(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        edit = _sync_todos().edit(todo_id=42)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        hooks = _RecordingHooks()
        with _sync_todos(hooks).edit(todo_id=42) as t:
            t.content = "observed"

        assert hooks.operations == ["todos.get", "todos.replace"]


class TestSyncReplace:
    @respx.mock
    def test_sparse_replace_issues_no_get_and_omits_unset(self):
        get_route = respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        result = _sync_todos().replace(todo_id=42, content="the whole new todo")

        assert result["id"] == 42
        assert not get_route.called
        body = _put_body(put_route)
        assert body["content"] == "the whole new todo"
        for field in ("description", "assignee_ids", "completion_subscriber_ids", "notify", "due_on", "starts_on"):
            assert field not in body


class TestAsyncUpdate:
    @respx.mock
    @pytest.mark.asyncio
    async def test_merges_unset_fields(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        result = await _async_todos().update(todo_id=42, content="Updated task")

        assert result["id"] == 42
        body = _put_body(put_route)
        assert body["content"] == "Updated task"
        assert body["description"] == "<p>From the store</p>"
        assert body["assignee_ids"] == [100]
        assert body["completion_subscriber_ids"] == [555]

    @respx.mock
    @pytest.mark.asyncio
    async def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        hooks = _RecordingHooks()
        await _async_todos(hooks).update(todo_id=42, content="observed")

        assert hooks.operations == ["todos.get", "todos.replace"]


class TestAsyncEdit:
    @respx.mock
    @pytest.mark.asyncio
    async def test_edit_puts_full_state_back(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        async with _async_todos().edit(todo_id=42) as t:
            t.content = f"🚨 {t.content}"

        assert t.result["id"] == 42
        assert _put_body(put_route)["content"] == "🚨 Buy milk"

    @respx.mock
    @pytest.mark.asyncio
    async def test_exception_aborts_without_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with pytest.raises(RuntimeError, match="abort"):
            async with _async_todos().edit(todo_id=42) as t:
                t.content = "never written"
                raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    @pytest.mark.asyncio
    async def test_result_raises_before_completion(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        edit = _async_todos().edit(todo_id=42)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result


class TestAsyncReplace:
    @respx.mock
    @pytest.mark.asyncio
    async def test_sparse_replace_issues_no_get(self):
        get_route = respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        await _async_todos().replace(todo_id=42, content="verbatim")

        assert not get_route.called
        body = _put_body(put_route)
        assert body["content"] == "verbatim"
        assert "description" not in body
