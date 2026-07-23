"""Tests for card-table wormhole operations (sync + async)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ForbiddenError, NotFoundError, ValidationError

CREATE_URL = "https://3.basecampapi.com/12345/buckets/2085958499/card_tables/1069479345/wormholes.json"
WORMHOLE_URL = "https://3.basecampapi.com/12345/buckets/2085958499/card_tables/wormholes/1069479400"
MISSING_URL = "https://3.basecampapi.com/12345/buckets/2085958499/card_tables/wormholes/999"


def _wormhole(wormhole_id: int = 1069479400, linked: bool = True) -> dict:
    return {
        "id": wormhole_id,
        "status": "active",
        "visible_to_clients": False,
        "title": "Design → Marketing backlog",
        "type": "Kanban::Wormhole",
        "color": "#f5d76e",
        "linked": linked,
        "destination_url": (
            "https://3.basecampapi.com/12345/buckets/2085958500/card_tables/columns/1069479500.json" if linked else None
        ),
    }


class TestSyncWormholes:
    @respx.mock
    def test_create(self):
        route = respx.post(CREATE_URL).mock(return_value=httpx.Response(201, json=_wormhole(99)))

        c = Client(access_token="test-token")
        wormhole = c.for_account("12345").wormholes.create(
            bucket_id=2085958499, card_table_id=1069479345, destination_recording_id=1069479500
        )
        c.close()

        assert route.called
        assert wormhole["id"] == 99
        assert wormhole["linked"] is True
        assert wormhole["destination_url"] is not None
        body = json.loads(route.calls.last.request.content)
        assert body == {"destination_recording_id": 1069479500}

    @respx.mock
    def test_create_validation_error_at_limit(self):
        respx.post(CREATE_URL).mock(return_value=httpx.Response(422, json={"error": "Limit reached"}))

        c = Client(access_token="test-token")
        with pytest.raises(ValidationError):
            c.for_account("12345").wormholes.create(
                bucket_id=2085958499, card_table_id=1069479345, destination_recording_id=1069479500
            )
        c.close()

    @respx.mock
    def test_create_not_found_destination(self):
        respx.post(CREATE_URL).mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        c = Client(access_token="test-token")
        with pytest.raises(NotFoundError):
            c.for_account("12345").wormholes.create(
                bucket_id=2085958499, card_table_id=1069479345, destination_recording_id=999
            )
        c.close()

    @respx.mock
    def test_update(self):
        route = respx.put(WORMHOLE_URL).mock(return_value=httpx.Response(200, json=_wormhole()))

        c = Client(access_token="test-token")
        wormhole = c.for_account("12345").wormholes.update(
            bucket_id=2085958499, wormhole_id=1069479400, destination_recording_id=1069479501
        )
        c.close()

        assert route.called
        assert wormhole["id"] == 1069479400
        body = json.loads(route.calls.last.request.content)
        assert body == {"destination_recording_id": 1069479501}

    @respx.mock
    def test_update_not_found(self):
        respx.put(MISSING_URL).mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        c = Client(access_token="test-token")
        with pytest.raises(NotFoundError):
            c.for_account("12345").wormholes.update(bucket_id=2085958499, wormhole_id=999, destination_recording_id=1)
        c.close()

    @respx.mock
    def test_delete(self):
        route = respx.delete(WORMHOLE_URL).mock(return_value=httpx.Response(204))

        c = Client(access_token="test-token")
        result = c.for_account("12345").wormholes.delete(bucket_id=2085958499, wormhole_id=1069479400)
        c.close()

        assert result is None
        assert route.called

    @respx.mock
    def test_delete_forbidden(self):
        respx.delete(WORMHOLE_URL).mock(return_value=httpx.Response(403, json={"error": "Forbidden"}))

        c = Client(access_token="test-token")
        with pytest.raises(ForbiddenError):
            c.for_account("12345").wormholes.delete(bucket_id=2085958499, wormhole_id=1069479400)
        c.close()

    @respx.mock
    def test_delete_not_found(self):
        respx.delete(MISSING_URL).mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        c = Client(access_token="test-token")
        with pytest.raises(NotFoundError):
            c.for_account("12345").wormholes.delete(bucket_id=2085958499, wormhole_id=999)
        c.close()


class TestAsyncWormholes:
    @pytest.mark.asyncio
    @respx.mock
    async def test_create(self):
        route = respx.post(CREATE_URL).mock(return_value=httpx.Response(201, json=_wormhole(99)))

        c = AsyncClient(access_token="test-token")
        wormhole = await c.for_account("12345").wormholes.create(
            bucket_id=2085958499, card_table_id=1069479345, destination_recording_id=1069479500
        )
        await c.close()

        assert route.called
        assert wormhole["id"] == 99
        assert wormhole["linked"] is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_validation_error_at_limit(self):
        respx.post(CREATE_URL).mock(return_value=httpx.Response(422, json={"error": "Limit reached"}))

        c = AsyncClient(access_token="test-token")
        with pytest.raises(ValidationError):
            await c.for_account("12345").wormholes.create(
                bucket_id=2085958499, card_table_id=1069479345, destination_recording_id=1069479500
            )
        await c.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_not_found_destination(self):
        respx.post(CREATE_URL).mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        c = AsyncClient(access_token="test-token")
        with pytest.raises(NotFoundError):
            await c.for_account("12345").wormholes.create(
                bucket_id=2085958499, card_table_id=1069479345, destination_recording_id=999
            )
        await c.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_update(self):
        route = respx.put(WORMHOLE_URL).mock(return_value=httpx.Response(200, json=_wormhole()))

        c = AsyncClient(access_token="test-token")
        wormhole = await c.for_account("12345").wormholes.update(
            bucket_id=2085958499, wormhole_id=1069479400, destination_recording_id=1069479501
        )
        await c.close()

        assert route.called
        assert wormhole["id"] == 1069479400

    @pytest.mark.asyncio
    @respx.mock
    async def test_update_not_found(self):
        respx.put(MISSING_URL).mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        c = AsyncClient(access_token="test-token")
        with pytest.raises(NotFoundError):
            await c.for_account("12345").wormholes.update(
                bucket_id=2085958499, wormhole_id=999, destination_recording_id=1
            )
        await c.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_delete(self):
        route = respx.delete(WORMHOLE_URL).mock(return_value=httpx.Response(204))

        c = AsyncClient(access_token="test-token")
        result = await c.for_account("12345").wormholes.delete(bucket_id=2085958499, wormhole_id=1069479400)
        await c.close()

        assert result is None
        assert route.called

    @pytest.mark.asyncio
    @respx.mock
    async def test_delete_forbidden(self):
        respx.delete(WORMHOLE_URL).mock(return_value=httpx.Response(403, json={"error": "Forbidden"}))

        c = AsyncClient(access_token="test-token")
        with pytest.raises(ForbiddenError):
            await c.for_account("12345").wormholes.delete(bucket_id=2085958499, wormhole_id=1069479400)
        await c.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_delete_not_found(self):
        respx.delete(MISSING_URL).mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        c = AsyncClient(access_token="test-token")
        with pytest.raises(NotFoundError):
            await c.for_account("12345").wormholes.delete(bucket_id=2085958499, wormhole_id=999)
        await c.close()
