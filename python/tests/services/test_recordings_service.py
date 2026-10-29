from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient
from basecamp.client import Client
from basecamp.errors import BasecampError, ForbiddenError, NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"
RECORDING = {"id": 456, "type": "Message", "title": "Launch", "status": "active"}


def make_account():
    client = Client(access_token="test-token")
    return client, client.for_account("12345")


FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures" / "recordings"


def load_fixture(name):
    return json.loads((FIXTURES / name).read_text())


class TestRecordingGet:
    @respx.mock
    def test_get_reads_a_recording_without_its_type(self):
        respx.get(f"{BASE}/recordings/1069479351").mock(
            return_value=httpx.Response(200, json=load_fixture("get.json"))
        )

        client, account = make_account()
        recording = account.recordings.get(recording_id=1069479351)
        client.close()

        assert recording["type"] == "Message"
        assert recording["parent"]["type"] == "Message::Board"
        assert recording["bucket"]["id"] == 2085958499

    @respx.mock
    def test_get_dock_tool_has_no_parent(self):
        respx.get(f"{BASE}/recordings/1069479338").mock(
            return_value=httpx.Response(200, json=load_fixture("get_tool.json"))
        )

        client, account = make_account()
        recording = account.recordings.get(recording_id=1069479338)
        client.close()

        assert recording["type"] == "Message::Board"
        assert "parent" not in recording

    @respx.mock
    def test_get_surfaces_not_found(self):
        respx.get(f"{BASE}/recordings/9999").mock(return_value=httpx.Response(404))

        client, account = make_account()
        with pytest.raises(NotFoundError):
            account.recordings.get(recording_id=9999)
        client.close()

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_get(self):
        respx.get(f"{BASE}/recordings/1069479351").mock(
            return_value=httpx.Response(200, json=load_fixture("get.json"))
        )

        client = AsyncClient(access_token="test-token")
        recording = await client.for_account("12345").recordings.get(recording_id=1069479351)
        await client.close()

        assert recording["type"] == "Message"


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


class TestRecordingMoveToVault:
    @respx.mock
    def test_posts_the_vault_and_the_position_only_when_given(self):
        route = respx.post(f"{BASE}/recordings/456/filing.json").mock(return_value=httpx.Response(204))

        client, account = make_account()
        assert account.recordings.move_to_vault(recording_id=456, parent_id=789) is None
        assert account.recordings.move_to_vault(recording_id=456, parent_id=789, position=2) is None
        client.close()

        bodies = [json.loads(call.request.content) for call in route.calls]
        assert bodies == [{"parent_id": 789}, {"parent_id": 789, "position": 2}]

    @respx.mock
    def test_surfaces_the_refusal_reason(self):
        respx.post(f"{BASE}/recordings/456/filing.json").mock(
            return_value=httpx.Response(422, json={"error": "Parent must not be self or descendant"})
        )

        client, account = make_account()
        with pytest.raises(ValidationError) as excinfo:
            account.recordings.move_to_vault(recording_id=456, parent_id=789)
        client.close()

        assert excinfo.value.http_status == 422
        assert "Parent must not be self or descendant" in str(excinfo.value)

    @respx.mock
    def test_surfaces_a_bodiless_refusal(self):
        respx.post(f"{BASE}/recordings/456/filing.json").mock(return_value=httpx.Response(403))

        client, account = make_account()
        with pytest.raises(ForbiddenError) as excinfo:
            account.recordings.move_to_vault(recording_id=456, parent_id=789)
        client.close()

        assert excinfo.value.http_status == 403


class TestAsyncRecordingMoveToVault:
    @pytest.mark.asyncio
    @respx.mock
    async def test_posts_the_vault_and_the_position_only_when_given(self):
        route = respx.post(f"{BASE}/recordings/456/filing.json").mock(return_value=httpx.Response(204))

        client = AsyncClient(access_token="test-token")
        recordings = client.for_account("12345").recordings
        assert await recordings.move_to_vault(recording_id=456, parent_id=789) is None
        assert await recordings.move_to_vault(recording_id=456, parent_id=789, position=2) is None
        await client.close()

        bodies = [json.loads(call.request.content) for call in route.calls]
        assert bodies == [{"parent_id": 789}, {"parent_id": 789, "position": 2}]

    @pytest.mark.asyncio
    @respx.mock
    async def test_surfaces_a_bodiless_refusal(self):
        respx.post(f"{BASE}/recordings/456/filing.json").mock(return_value=httpx.Response(422))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ValidationError):
            await client.for_account("12345").recordings.move_to_vault(recording_id=456, parent_id=789)
        await client.close()


PENDING_MOVE = {
    "id": 4,
    "status": "pending",
    "type": "Move",
    "source_recording_id": 456,
    "url": f"{BASE}/filings/4.json",
}
COMPLETED_MOVE = {
    **PENDING_MOVE,
    "status": "completed",
    "destination_recording": {
        "id": 1069480270,
        "type": "Kanban::Card",
        "url": f"{BASE}/buckets/2085958505/card_tables/cards/1069480270.json",
        "app_url": "https://3.basecamp.com/12345/buckets/2085958505/card_tables/cards/1069480270",
    },
}


class TestRecordingMovesAndCopies:
    @respx.mock
    def test_move_posts_the_destination_and_returns_the_filing(self):
        route = respx.post(f"{BASE}/recordings/456/moves.json").mock(
            return_value=httpx.Response(201, json=PENDING_MOVE)
        )

        client, account = make_account()
        filing = account.recordings.move(recording_id=456, filing_destination_id=789)
        client.close()

        assert json.loads(route.calls[0].request.content) == {"filing_destination_id": 789}
        assert filing == PENDING_MOVE

    @respx.mock
    def test_move_surfaces_a_refusal(self):
        respx.post(f"{BASE}/recordings/456/moves.json").mock(return_value=httpx.Response(403))

        client, account = make_account()
        with pytest.raises(ForbiddenError):
            account.recordings.move(recording_id=456, filing_destination_id=789)
        client.close()

    @respx.mock
    def test_copy_posts_the_destination_and_include_comments(self):
        route = respx.post(f"{BASE}/recordings/456/copies.json").mock(
            return_value=httpx.Response(201, json={**PENDING_MOVE, "type": "Copy"})
        )

        client, account = make_account()
        filing = account.recordings.copy(recording_id=456, destination_project_id=2085958505, include_comments=True)
        client.close()

        assert json.loads(route.calls[0].request.content) == {
            "destination_project_id": 2085958505,
            "include_comments": True,
        }
        assert filing["type"] == "Copy"

    @respx.mock
    def test_copy_is_sent_once_on_a_503(self):
        route = respx.post(f"{BASE}/recordings/456/copies.json").mock(return_value=httpx.Response(503))

        client, account = make_account()
        with pytest.raises(BasecampError):
            account.recordings.copy(recording_id=456, filing_destination_id=789)
        client.close()

        assert route.call_count == 1

    @respx.mock
    def test_get_filing_returns_the_recording_it_made(self):
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(200, json=COMPLETED_MOVE))

        client, account = make_account()
        filing = account.recordings.get_filing(filing_id=4)
        client.close()

        assert filing["destination_recording"]["id"] == 1069480270

    @respx.mock
    def test_get_filing_surfaces_not_found(self):
        respx.get(f"{BASE}/filings/4").mock(
            return_value=httpx.Response(404, json={"status": 404, "error": "Not Found"})
        )

        client, account = make_account()
        with pytest.raises(NotFoundError):
            account.recordings.get_filing(filing_id=4)
        client.close()


class TestAsyncRecordingMovesAndCopies:
    @pytest.mark.asyncio
    @respx.mock
    async def test_move_and_poll(self):
        respx.post(f"{BASE}/recordings/456/moves.json").mock(return_value=httpx.Response(201, json=PENDING_MOVE))
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(200, json=COMPLETED_MOVE))

        client = AsyncClient(access_token="test-token")
        recordings = client.for_account("12345").recordings
        filing = await recordings.move(recording_id=456, filing_destination_id=789)
        completed = await recordings.get_filing(filing_id=filing["id"])
        await client.close()

        assert completed["destination_recording"]["id"] == 1069480270

    @pytest.mark.asyncio
    @respx.mock
    async def test_copy_is_sent_once_on_a_503(self):
        route = respx.post(f"{BASE}/recordings/456/copies.json").mock(return_value=httpx.Response(503))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(BasecampError):
            await client.for_account("12345").recordings.copy(recording_id=456, filing_destination_id=789)
        await client.close()

        assert route.call_count == 1
