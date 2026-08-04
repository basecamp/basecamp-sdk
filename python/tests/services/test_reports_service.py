"""Tests for the upcoming-schedule report (sync + async).

`GetUpcomingSchedule` renders BC3's reduced calendar partials
(``app/views/api/schedules/calendar/``), not the per-resource ones. Until #635
the spec declared the shared ``ScheduleEntry`` and a half-modelled
``Assignable`` instead. Python is lenient — the generated method hands back the
parsed body verbatim — so nothing here would have thrown; what it would have
done is type a response that Swift could not decode at all.

The body is the shared fixture, which ``make check-fixture-coverage`` validates
against the generated schema. An invented body is how the mismatch survived six
SDKs, so these read the real one from disk.
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client

UPCOMING_PATH = "/12345/reports/schedules/upcoming.json"
UPCOMING_URL = f"https://3.basecampapi.com{UPCOMING_PATH}"

FIXTURE = json.loads(
    (Path(__file__).resolve().parents[3] / "spec" / "fixtures" / "schedules" / "upcoming.json").read_text(
        encoding="utf-8"
    )
)

EMPTY_ENVELOPE = {
    "schedule_entries": [],
    "recurring_schedule_entry_occurrences": [],
    "assignables": [],
}


def _assert_reduced_projection(result: dict) -> None:
    assert set(result) == {
        "schedule_entries",
        "recurring_schedule_entry_occurrences",
        "assignables",
    }

    entry = result["schedule_entries"][0]
    assert entry["summary"] == "Team Meeting"
    # Emitted only by the calendar partial, and the flag that separates the two
    # entry arrays: BC3 selects schedule_entries with recurrence_schedule IS
    # NULL and the occurrences with it NOT NULL.
    assert entry["recurring"] is False
    # id + name only — the calendar partial writes
    # json.(recording.bucket, :id, :name), so there is no `type`.
    assert set(entry["bucket"]) == {"id", "name"}

    occurrence = result["recurring_schedule_entry_occurrences"][0]
    assert occurrence["recurring"] is True
    assert occurrence["all_day"] is True
    # An all-day entry reads back as a bare date, not a timestamp.
    assert occurrence["starts_at"] == "2026-06-08"

    todo, card = result["assignables"]
    # BC3 spells the item text `content`, never `title`.
    assert todo["content"] == "Ship the hardware"
    assert "title" not in todo
    # Lowercase short recordable name, unlike the CamelCase `type` other
    # recording projections carry.
    assert todo["type"] == "todo"
    assert todo["completion"]["creator"]["name"] == "Steve Marsh"
    # The partial's one conditional key: absent, not null.
    assert "completion" not in card
    assert card["starts_on"] is None
    # Non-to-dos get bucket_step_completions_path — a `_path` helper, no host.
    assert card["completion_url"] == "/999/buckets/2085958499/steps/1069479526/completions.json"


class TestSyncUpcomingSchedule:
    @respx.mock
    def test_returns_the_reduced_calendar_projection(self):
        route = respx.get(UPCOMING_URL).mock(return_value=httpx.Response(200, json=FIXTURE))

        c = Client(access_token="test-token")
        result = c.for_account("12345").reports.upcoming(window_starts_on="2026-06-01", window_ends_on="2026-06-30")
        c.close()

        assert route.called
        request = route.calls.last.request
        assert request.url.path == UPCOMING_PATH
        # Both bounds are required — BC3 reads them with params.require and
        # answers a bodiless 400 without them — so they always reach the wire.
        assert dict(request.url.params) == {
            "window_starts_on": "2026-06-01",
            "window_ends_on": "2026-06-30",
        }
        _assert_reduced_projection(result)

    @respx.mock
    def test_empty_window_is_three_empty_arrays(self):
        respx.get(UPCOMING_URL).mock(return_value=httpx.Response(200, json=EMPTY_ENVELOPE))

        c = Client(access_token="test-token")
        result = c.for_account("12345").reports.upcoming(window_starts_on="2026-01-01", window_ends_on="2026-01-31")
        c.close()

        assert result == EMPTY_ENVELOPE

    def test_both_window_bounds_are_required(self):
        c = Client(access_token="test-token")
        with pytest.raises(TypeError):
            c.for_account("12345").reports.upcoming(window_starts_on="2026-01-01")
        c.close()


class TestAsyncUpcomingSchedule:
    @respx.mock
    @pytest.mark.asyncio
    async def test_returns_the_reduced_calendar_projection(self):
        respx.get(UPCOMING_URL).mock(return_value=httpx.Response(200, json=FIXTURE))

        c = AsyncClient(access_token="test-token")
        result = await c.for_account("12345").reports.upcoming(
            window_starts_on="2026-06-01", window_ends_on="2026-06-30"
        )
        await c.close()

        _assert_reduced_projection(result)
