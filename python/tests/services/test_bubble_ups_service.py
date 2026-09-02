"""Tests for the BubbleUpsService (per-recording bubble-up create/delete)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import ForbiddenError, NotFoundError

BASE = "https://3.basecampapi.com/12345"


def _bubble_ups():
    return Client(access_token="test-token").for_account("12345").bubble_ups


class TestCreateBubbleUp:
    @respx.mock
    def test_schedules_with_at_on_the_wire(self):
        route = respx.post(f"{BASE}/recordings/900/bubble_up.json").mock(
            return_value=httpx.Response(204)
        )

        _bubble_ups().create_bubble_up(recording_id=900, at="2026-09-10T09:00:00Z")

        assert json.loads(route.calls.last.request.content) == {"at": "2026-09-10T09:00:00Z"}

    @respx.mock
    def test_omits_at_when_absent(self):
        route = respx.post(f"{BASE}/recordings/900/bubble_up.json").mock(
            return_value=httpx.Response(204)
        )

        _bubble_ups().create_bubble_up(recording_id=900)

        assert "at" not in json.loads(route.calls.last.request.content)

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.post(f"{BASE}/recordings/999/bubble_up.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _bubble_ups().create_bubble_up(recording_id=999)


class TestDeleteBubbleUp:
    @respx.mock
    def test_pops_the_bubble_up(self):
        route = respx.delete(f"{BASE}/recordings/900/bubble_up.json").mock(
            return_value=httpx.Response(204)
        )

        _bubble_ups().delete_bubble_up(recording_id=900)

        assert route.called

    @respx.mock
    def test_403_surfaces_as_forbidden(self):
        respx.delete(f"{BASE}/recordings/900/bubble_up.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        with pytest.raises(ForbiddenError):
            _bubble_ups().delete_bubble_up(recording_id=900)
