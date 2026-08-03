"""Tests for timesheet entry operations (sync + async)."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ForbiddenError

ENTRY_ID = 1069479555
# Flat and unscoped: entries hang off the account, not off a bucket.
ENTRY_PATH = f"/12345/timesheet_entries/{ENTRY_ID}"
ENTRY_URL = f"https://3.basecampapi.com{ENTRY_PATH}"


class TestSyncDestroyTimesheetEntry:
    @respx.mock
    def test_destroys_with_204(self):
        route = respx.delete(ENTRY_URL).mock(return_value=httpx.Response(204))

        c = Client(access_token="test-token")
        result = c.for_account("12345").timesheets.destroy(entry_id=ENTRY_ID)
        c.close()

        assert result is None
        assert route.called
        request = route.calls.last.request
        assert request.method == "DELETE"
        assert request.url.path == ENTRY_PATH

    @respx.mock
    def test_403_surfaces_as_forbidden(self):
        respx.delete(ENTRY_URL).mock(return_value=httpx.Response(403, json={"error": "Forbidden"}))

        c = Client(access_token="test-token")
        with pytest.raises(ForbiddenError) as excinfo:
            c.for_account("12345").timesheets.destroy(entry_id=ENTRY_ID)
        c.close()

        assert excinfo.value.http_status == 403
        assert excinfo.value.code == "forbidden"
        assert str(excinfo.value) == "Forbidden"


class TestAsyncDestroyTimesheetEntry:
    @pytest.mark.asyncio
    @respx.mock
    async def test_destroys_with_204(self):
        route = respx.delete(ENTRY_URL).mock(return_value=httpx.Response(204))

        c = AsyncClient(access_token="test-token")
        result = await c.for_account("12345").timesheets.destroy(entry_id=ENTRY_ID)
        await c.close()

        assert result is None
        assert route.called
        request = route.calls.last.request
        assert request.method == "DELETE"
        assert request.url.path == ENTRY_PATH

    @pytest.mark.asyncio
    @respx.mock
    async def test_403_surfaces_as_forbidden(self):
        respx.delete(ENTRY_URL).mock(return_value=httpx.Response(403, json={"error": "Forbidden"}))

        c = AsyncClient(access_token="test-token")
        with pytest.raises(ForbiddenError) as excinfo:
            await c.for_account("12345").timesheets.destroy(entry_id=ENTRY_ID)
        await c.close()

        assert excinfo.value.http_status == 403
        assert excinfo.value.code == "forbidden"
        assert str(excinfo.value) == "Forbidden"
