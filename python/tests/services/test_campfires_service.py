"""Tests for campfire line operations (sync + async)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ValidationError


def _line(line_id: int = 300, content: str = "Hello everyone!") -> dict:
    return {
        "id": line_id,
        "status": "active",
        "type": "Chat::Lines::Text",
        "content": content,
    }


class TestSyncCampfireLines:
    @respx.mock
    def test_create_line(self):
        route = respx.post("https://3.basecampapi.com/12345/chats/200/lines.json").mock(
            return_value=httpx.Response(201, json=_line(999, "New message"))
        )

        c = Client(access_token="test-token")
        line = c.for_account("12345").campfires.create_line(campfire_id=200, content="New message")
        c.close()

        assert route.called
        assert line["id"] == 999
        assert line["content"] == "New message"

    @respx.mock
    def test_get_line(self):
        route = respx.get("https://3.basecampapi.com/12345/chats/200/lines/300").mock(
            return_value=httpx.Response(200, json=_line())
        )

        c = Client(access_token="test-token")
        line = c.for_account("12345").campfires.get_line(campfire_id=200, line_id=300)
        c.close()

        assert route.called
        assert line["id"] == 300

    @respx.mock
    def test_update_line(self):
        route = respx.put("https://3.basecampapi.com/12345/chats/200/lines/300").mock(return_value=httpx.Response(204))

        c = Client(access_token="test-token")
        result = c.for_account("12345").campfires.update_line(campfire_id=200, line_id=300, content="Edited!")
        c.close()

        assert result is None
        assert route.called
        body = json.loads(route.calls.last.request.content)
        assert body == {"content": "Edited!"}

    @respx.mock
    def test_update_line_validation_error(self):
        respx.put("https://3.basecampapi.com/12345/chats/200/lines/300").mock(
            return_value=httpx.Response(422, json={"error": "Unprocessable"})
        )

        c = Client(access_token="test-token")
        with pytest.raises(ValidationError):
            c.for_account("12345").campfires.update_line(campfire_id=200, line_id=300, content="Edited!")
        c.close()

    @respx.mock
    def test_delete_line(self):
        route = respx.delete("https://3.basecampapi.com/12345/chats/200/lines/300").mock(
            return_value=httpx.Response(204)
        )

        c = Client(access_token="test-token")
        result = c.for_account("12345").campfires.delete_line(campfire_id=200, line_id=300)
        c.close()

        assert result is None
        assert route.called


class TestAsyncCampfireLines:
    @pytest.mark.asyncio
    @respx.mock
    async def test_update_line(self):
        route = respx.put("https://3.basecampapi.com/12345/chats/200/lines/300").mock(return_value=httpx.Response(204))

        c = AsyncClient(access_token="test-token")
        result = await c.for_account("12345").campfires.update_line(campfire_id=200, line_id=300, content="Edited!")
        await c.close()

        assert result is None
        assert route.called
        body = json.loads(route.calls.last.request.content)
        assert body == {"content": "Edited!"}

    @pytest.mark.asyncio
    @respx.mock
    async def test_update_line_validation_error(self):
        respx.put("https://3.basecampapi.com/12345/chats/200/lines/300").mock(
            return_value=httpx.Response(422, json={"error": "Unprocessable"})
        )

        c = AsyncClient(access_token="test-token")
        with pytest.raises(ValidationError):
            await c.for_account("12345").campfires.update_line(campfire_id=200, line_id=300, content="Edited!")
        await c.close()
