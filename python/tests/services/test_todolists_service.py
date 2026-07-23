"""Tests for the todolist reposition surface (sync + async)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import NotFoundError

BASE = "https://3.basecampapi.com/12345"


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
