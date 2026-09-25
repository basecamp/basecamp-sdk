"""Tests for the SubtasksService.

A subtask is a CardStep on the wire — ``type`` stays ``"Kanban::Step"`` — reached
through the canonical flat ``/subtasks`` routes bc3 documents (bc3#12659).
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import ForbiddenError, NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"


def _subtask(subtask_id: int, position: int = 1) -> dict:
    return {
        "id": subtask_id,
        "status": "active",
        "visible_to_clients": False,
        "created_at": "2026-07-02T00:23:00.000Z",
        "updated_at": "2026-07-02T00:23:00.000Z",
        "title": "Hero shot on the desk",
        "inherits_status": True,
        "type": "Kanban::Step",
        "url": f"{BASE}/buckets/1/subtasks/{subtask_id}.json",
        "app_url": f"https://3.basecamp.com/12345/buckets/1/todos/200#__recording_{subtask_id}",
        "position": position,
        "completed": False,
        "due_on": None,
        "parent": {"id": 200, "title": "Shot list", "type": "Todo"},
        "bucket": {"id": 1, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 100, "name": "Matt Donahue"},
        "assignees": [],
        "completion_url": f"{BASE}/subtasks/{subtask_id}/completion.json",
    }


def _subtasks():
    return Client(access_token="test-token").for_account("12345").subtasks


class TestList:
    @respx.mock
    def test_lists_in_position_order(self):
        respx.get(f"{BASE}/recordings/200/subtasks.json").mock(
            return_value=httpx.Response(200, json=[_subtask(1, 1), _subtask(2, 2)], headers={"X-Total-Count": "2"})
        )

        result = _subtasks().list(recording_id=200)

        assert [s["id"] for s in result] == [1, 2]
        assert result[0]["type"] == "Kanban::Step"

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/recordings/999/subtasks.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _subtasks().list(recording_id=999)


class TestGet:
    @respx.mock
    def test_returns_the_subtask(self):
        respx.get(f"{BASE}/subtasks/42").mock(return_value=httpx.Response(200, json=_subtask(42)))

        subtask = _subtasks().get(subtask_id=42)

        assert subtask["id"] == 42
        assert subtask["completion_url"] == f"{BASE}/subtasks/42/completion.json"

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/subtasks/999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _subtasks().get(subtask_id=999)


class TestCreate:
    @respx.mock
    def test_sends_the_documented_parameters(self):
        route = respx.post(f"{BASE}/recordings/200/subtasks.json").mock(
            return_value=httpx.Response(201, json=_subtask(99))
        )

        subtask = _subtasks().create(
            recording_id=200, title="Book the room", due_on="2026-09-20", assignee_ids=[30068628, 270913789]
        )

        assert subtask["id"] == 99
        assert json.loads(route.calls.last.request.content) == {
            "title": "Book the room",
            "due_on": "2026-09-20",
            "assignee_ids": [30068628, 270913789],
        }

    @respx.mock
    def test_403_on_a_recording_that_cannot_hold_subtasks(self):
        respx.post(f"{BASE}/recordings/300/subtasks.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        with pytest.raises(ForbiddenError):
            _subtasks().create(recording_id=300, title="Nope")


class TestUpdate:
    @respx.mock
    def test_sends_only_the_fields_given(self):
        route = respx.put(f"{BASE}/subtasks/42").mock(return_value=httpx.Response(200, json=_subtask(42)))

        _subtasks().update(subtask_id=42, title="Book the big room")

        assert json.loads(route.calls.last.request.content) == {"title": "Book the big room"}

    @respx.mock
    def test_clears_assignees_with_an_explicit_empty_list(self):
        route = respx.put(f"{BASE}/subtasks/42").mock(return_value=httpx.Response(200, json=_subtask(42)))

        _subtasks().update(subtask_id=42, assignee_ids=[])

        assert json.loads(route.calls.last.request.content) == {"assignee_ids": []}

    @respx.mock
    def test_422_surfaces_as_validation_error(self):
        respx.put(f"{BASE}/subtasks/42").mock(
            return_value=httpx.Response(422, json={"errors": {"due_on": ["is not a valid date"]}})
        )

        with pytest.raises(ValidationError):
            _subtasks().update(subtask_id=42, due_on="not-a-date")


class TestCompletion:
    @respx.mock
    def test_complete_posts_and_uncomplete_deletes(self):
        complete = respx.post(f"{BASE}/subtasks/42/completion.json").mock(return_value=httpx.Response(204))
        uncomplete = respx.delete(f"{BASE}/subtasks/42/completion.json").mock(return_value=httpx.Response(204))

        assert _subtasks().complete(subtask_id=42) is None
        assert _subtasks().uncomplete(subtask_id=42) is None
        assert complete.called
        assert uncomplete.called

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.post(f"{BASE}/subtasks/999/completion.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _subtasks().complete(subtask_id=999)

    @respx.mock
    def test_uncomplete_404_surfaces_as_not_found(self):
        respx.delete(f"{BASE}/subtasks/999/completion.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _subtasks().uncomplete(subtask_id=999)


class TestReposition:
    @respx.mock
    def test_puts_the_one_based_position(self):
        route = respx.put(f"{BASE}/subtasks/42/position.json").mock(return_value=httpx.Response(204))

        assert _subtasks().reposition(subtask_id=42, position=4) is None
        assert json.loads(route.calls.last.request.content) == {"position": 4}

    @respx.mock
    def test_422_surfaces_as_validation_error(self):
        respx.put(f"{BASE}/subtasks/42/position.json").mock(
            return_value=httpx.Response(422, json={"errors": {"position": ["must be greater than 0"]}})
        )

        with pytest.raises(ValidationError):
            _subtasks().reposition(subtask_id=42, position=0)


class TestDelete:
    @respx.mock
    def test_deletes_with_204(self):
        route = respx.delete(f"{BASE}/subtasks/42").mock(return_value=httpx.Response(204))

        assert _subtasks().delete(subtask_id=42) is None
        assert route.called

    @respx.mock
    def test_403_surfaces_as_forbidden(self):
        respx.delete(f"{BASE}/subtasks/42").mock(return_value=httpx.Response(403, json={"error": "Forbidden"}))

        with pytest.raises(ForbiddenError):
            _subtasks().delete(subtask_id=42)
