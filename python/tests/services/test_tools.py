"""Tests for generated tools service routes."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client


def _tool(tool_id: int = 800, *, title: str = "Message Board") -> dict:
    return {
        "id": tool_id,
        "name": "message_board",
        "title": title,
        "enabled": True,
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z",
    }


class TestSyncTools:
    @respx.mock
    def test_create_posts_to_bucket_scoped_dock_tools_path(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool(title="Message Board (Copy)"))
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.tools.create(
            bucket_id=456,
            tool_type="Message::Board",
            title="Message Board (Copy)",
        )

        assert route.called
        request = route.calls[0].request
        assert request.method == "POST"
        assert json.loads(request.content) == {
            "tool_type": "Message::Board",
            "title": "Message Board (Copy)",
        }
        assert result["id"] == 800

    @respx.mock
    def test_create_omits_title_when_not_provided(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool())
        )

        account = Client(access_token="test-token").for_account("12345")
        account.tools.create(bucket_id=456, tool_type="Message::Board")

        assert route.called
        assert json.loads(route.calls[0].request.content) == {"tool_type": "Message::Board"}


class TestAsyncTools:
    @pytest.mark.asyncio
    @respx.mock
    async def test_create_posts_to_bucket_scoped_dock_tools_path(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool(title="Message Board (Copy)"))
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.tools.create(
            bucket_id=456,
            tool_type="Message::Board",
            title="Message Board (Copy)",
        )

        assert route.called
        request = route.calls[0].request
        assert request.method == "POST"
        assert json.loads(request.content) == {
            "tool_type": "Message::Board",
            "title": "Message Board (Copy)",
        }
        assert result["id"] == 800
