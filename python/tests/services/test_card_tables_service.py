"""Tests for card table decode, including the wormholes[] array (sync + async)."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client

CARD_TABLE_URL = "https://3.basecampapi.com/12345/card_tables/1069479345"


def _card_table() -> dict:
    return {
        "id": 1069479345,
        "status": "active",
        "title": "Development Board",
        "type": "Kanban::Board",
        "lists": [{"id": 100, "title": "To Do"}],
        "wormholes": [
            {
                "id": 1069479400,
                "title": "Design → Marketing backlog",
                "linked": True,
                "color": "#f5d76e",
                "destination_url": "https://3.basecampapi.com/12345/buckets/2085958500/card_tables/columns/1069479500.json",
            },
            {
                "id": 1069479401,
                "title": "Broken teleport",
                "linked": False,
                "color": None,
                "destination_url": None,
            },
        ],
    }


class TestSyncCardTables:
    @respx.mock
    def test_get_decodes_linked_and_unlinked_wormholes(self):
        respx.get(CARD_TABLE_URL).mock(return_value=httpx.Response(200, json=_card_table()))

        c = Client(access_token="test-token")
        table = c.for_account("12345").card_tables.get(card_table_id=1069479345)
        c.close()

        assert len(table["wormholes"]) == 2
        assert table["wormholes"][0]["linked"] is True
        assert table["wormholes"][0]["destination_url"] is not None
        assert table["wormholes"][1]["linked"] is False
        assert table["wormholes"][1]["destination_url"] is None


class TestAsyncCardTables:
    @pytest.mark.asyncio
    @respx.mock
    async def test_get_decodes_linked_and_unlinked_wormholes(self):
        respx.get(CARD_TABLE_URL).mock(return_value=httpx.Response(200, json=_card_table()))

        c = AsyncClient(access_token="test-token")
        table = await c.for_account("12345").card_tables.get(card_table_id=1069479345)
        await c.close()

        assert len(table["wormholes"]) == 2
        assert table["wormholes"][0]["linked"] is True
        assert table["wormholes"][0]["destination_url"] is not None
        assert table["wormholes"][1]["linked"] is False
        assert table["wormholes"][1]["destination_url"] is None
