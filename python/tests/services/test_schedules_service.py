"""Tests for schedule entry occurrence operation metadata (sync + async).

The occurrence path ends in ``{date}`` (a string), not an ``Id``-suffixed
param. ``resource_id`` must fall back to the entry id, never the date string.
"""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.hooks import BasecampHooks, OperationInfo

OCCURRENCE_URL = "https://3.basecampapi.com/12345/schedule_entries/789/occurrences/2024-12-22"


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[OperationInfo] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(info)


def _occurrence() -> dict:
    return {"id": 789, "summary": "Team Meeting", "occurrence_date": "2024-12-22"}


class TestSyncScheduleOccurrence:
    @respx.mock
    def test_get_entry_occurrence_scopes_resource_to_entry_id(self):
        respx.get(OCCURRENCE_URL).mock(return_value=httpx.Response(200, json=_occurrence()))

        hooks = _RecordingHooks()
        c = Client(access_token="test-token", hooks=hooks)
        c.for_account("12345").schedules.get_entry_occurrence(entry_id=789, date="2024-12-22")
        c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "schedules"
        assert info.operation == "get_entry_occurrence"
        assert info.resource_id == 789


class TestAsyncScheduleOccurrence:
    @pytest.mark.asyncio
    @respx.mock
    async def test_get_entry_occurrence_scopes_resource_to_entry_id(self):
        respx.get(OCCURRENCE_URL).mock(return_value=httpx.Response(200, json=_occurrence()))

        hooks = _RecordingHooks()
        c = AsyncClient(access_token="test-token", hooks=hooks)
        await c.for_account("12345").schedules.get_entry_occurrence(entry_id=789, date="2024-12-22")
        await c.close()

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "schedules"
        assert info.operation == "get_entry_occurrence"
        assert info.resource_id == 789
