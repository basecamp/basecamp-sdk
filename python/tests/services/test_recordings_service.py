from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient
from basecamp.client import Client
from basecamp.errors import ForbiddenError, ValidationError

BASE = "https://3.basecampapi.com/12345"
RECORDING = {"id": 456, "type": "Message", "title": "Launch", "status": "active"}


def make_account():
    client = Client(access_token="test-token")
    return client, client.for_account("12345")


class TestRecordingSpotlights:
    @respx.mock
    def test_spotlight_returns_recording_on_canonical_flat_route(self):
        route = respx.post(f"{BASE}/recordings/456/spotlight.json").mock(
            return_value=httpx.Response(201, json=RECORDING)
        )

        client, account = make_account()
        result = account.recordings.spotlight(recording_id=456)
        client.close()

        assert result == RECORDING
        assert route.call_count == 1

    @respx.mock
    def test_spotlight_surfaces_ineligible_recording(self):
        respx.post(f"{BASE}/recordings/456/spotlight.json").mock(
            return_value=httpx.Response(422, json={"errors": ["Recording cannot be spotlighted"]})
        )

        client, account = make_account()
        with pytest.raises(ValidationError) as excinfo:
            account.recordings.spotlight(recording_id=456)
        client.close()

        assert excinfo.value.http_status == 422

    @respx.mock
    def test_unspotlight_uses_canonical_flat_route(self):
        route = respx.delete(f"{BASE}/recordings/456/spotlight.json").mock(return_value=httpx.Response(204))

        client, account = make_account()
        assert account.recordings.unspotlight(recording_id=456) is None
        client.close()

        assert route.call_count == 1

    @respx.mock
    def test_unspotlight_surfaces_permission_error(self):
        respx.delete(f"{BASE}/recordings/456/spotlight.json").mock(return_value=httpx.Response(403))

        client, account = make_account()
        with pytest.raises(ForbiddenError) as excinfo:
            account.recordings.unspotlight(recording_id=456)
        client.close()

        assert excinfo.value.http_status == 403


class TestAsyncRecordingSpotlights:
    @pytest.mark.asyncio
    @respx.mock
    async def test_spotlight_returns_recording_on_canonical_flat_route(self):
        route = respx.post(f"{BASE}/recordings/456/spotlight.json").mock(
            return_value=httpx.Response(201, json=RECORDING)
        )

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").recordings.spotlight(recording_id=456)
        await client.close()

        assert result == RECORDING
        assert route.call_count == 1

    @pytest.mark.asyncio
    @respx.mock
    async def test_spotlight_surfaces_ineligible_recording(self):
        respx.post(f"{BASE}/recordings/456/spotlight.json").mock(
            return_value=httpx.Response(422, json={"errors": ["Recording cannot be spotlighted"]})
        )

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ValidationError):
            await client.for_account("12345").recordings.spotlight(recording_id=456)
        await client.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_unspotlight_uses_canonical_flat_route(self):
        route = respx.delete(f"{BASE}/recordings/456/spotlight.json").mock(return_value=httpx.Response(204))

        client = AsyncClient(access_token="test-token")
        assert await client.for_account("12345").recordings.unspotlight(recording_id=456) is None
        await client.close()

        assert route.call_count == 1

    @pytest.mark.asyncio
    @respx.mock
    async def test_unspotlight_surfaces_permission_error(self):
        respx.delete(f"{BASE}/recordings/456/spotlight.json").mock(return_value=httpx.Response(403))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ForbiddenError):
            await client.for_account("12345").recordings.unspotlight(recording_id=456)
        await client.close()
