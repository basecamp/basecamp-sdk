"""Tests for the CalendarsService (per-account calendars, show + update)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"

_CALENDAR = {
    "id": 2085958497,
    "type": "Calendar",
    "name": "Honcho Design Calendar",
    "color": "blue",
    "created_at": "2026-05-28T17:22:22.133Z",
    "updated_at": "2026-07-20T04:05:52.374Z",
    "url": "https://3.basecampapi.com/12345/calendars/2085958497.json",
    "app_url": "https://3.basecamp.com/12345/calendars/2085958497",
    "schedule_url": "https://3.basecampapi.com/12345/schedules/1069478892.json",
}


def _calendars():
    return Client(access_token="test-token").for_account("12345").calendars


class TestGetCalendar:
    @respx.mock
    def test_returns_the_calendar(self):
        respx.get(f"{BASE}/calendars/2085958497").mock(return_value=httpx.Response(200, json=_CALENDAR))

        calendar = _calendars().get_calendar(calendar_id=2085958497)

        assert calendar["id"] == 2085958497
        assert calendar["color"] == "blue"

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/calendars/999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _calendars().get_calendar(calendar_id=999)


class TestUpdateCalendar:
    @respx.mock
    def test_sends_nested_envelope(self):
        route = respx.put(f"{BASE}/calendars/2085958497").mock(
            return_value=httpx.Response(200, json={**_CALENDAR, "color": "green"})
        )

        calendar = _calendars().update_calendar(calendar_id=2085958497, calendar={"color": "green"})

        assert json.loads(route.calls[-1].request.content) == {"calendar": {"color": "green"}}
        assert calendar["color"] == "green"

    @respx.mock
    def test_field_keyed_422_surfaces_as_validation_error(self):
        respx.put(f"{BASE}/calendars/2085958497").mock(
            return_value=httpx.Response(422, json={"errors": {"color": ["is not a valid color"]}})
        )

        with pytest.raises(ValidationError):
            _calendars().update_calendar(calendar_id=2085958497, calendar={"color": "chartreuse"})
