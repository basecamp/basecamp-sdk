"""Tests for the PingsService.

A Ping is a private conversation held in its own bucket (bucket type and wire
path "Circle") with one Campfire. A Ping's id is its bucket's id, so the
``bucket.id`` a ping's chat lines carry is the id ``get`` takes.
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ForbiddenError, NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"
FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures" / "pings"

DISABLED = {"error": "Pings have been disabled"}


def _fixture(name: str):
    return json.loads((FIXTURES / name).read_text())


def _pings():
    return Client(access_token="test-token").for_account("12345").pings


def _async_pings():
    return AsyncClient(access_token="test-token").for_account("12345").pings


class TestList:
    @respx.mock
    def test_lists_the_callers_pings(self):
        respx.get(f"{BASE}/circles.json").mock(return_value=httpx.Response(200, json=_fixture("list.json")))

        result = _pings().list()

        assert len(result) == 2
        assert result[0]["id"] == 2085958600
        assert result[0]["named"] is True
        assert result[0]["room_url"].endswith("/buckets/2085958600/chats/1069479600.json")
        assert result[0]["links"]["lines"].endswith("/chats/1069479600/lines.json")
        assert [p["name"] for p in result[0]["people"]] == ["Annie Bryan", "Cheryl Walters"]
        assert result.meta.total_count == 2

    @respx.mock
    def test_an_empty_array_is_an_empty_list(self):
        respx.get(f"{BASE}/circles.json").mock(return_value=httpx.Response(200, json=[]))

        assert list(_pings().list()) == []

    @respx.mock
    def test_403_when_the_caller_cannot_ping(self):
        respx.get(f"{BASE}/circles.json").mock(return_value=httpx.Response(403, json=DISABLED))

        with pytest.raises(ForbiddenError):
            _pings().list()


class TestGet:
    @respx.mock
    def test_returns_the_ping(self):
        route = respx.get(f"{BASE}/circles/2085958600").mock(
            return_value=httpx.Response(200, json=_fixture("get.json"))
        )

        ping = _pings().get(ping_id=2085958600)

        assert route.called
        assert ping["id"] == 2085958600
        assert ping["title"] == "Launch crew"
        assert ping["room_url"] == "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json"

    @respx.mock
    def test_404_for_a_bucket_that_is_not_a_ping(self):
        respx.get(f"{BASE}/circles/999").mock(return_value=httpx.Response(404, json={"error": "Record not found"}))

        with pytest.raises(NotFoundError):
            _pings().get(ping_id=999)

    @respx.mock
    def test_403_when_the_caller_cannot_ping(self):
        respx.get(f"{BASE}/circles/2085958600").mock(return_value=httpx.Response(403, json=DISABLED))

        with pytest.raises(ForbiddenError) as raised:
            _pings().get(ping_id=2085958600)

        assert raised.value.http_status == 403


class TestCreate:
    @respx.mock
    def test_sends_the_people_alone_by_default(self):
        route = respx.post(f"{BASE}/circles.json").mock(return_value=httpx.Response(201, json=_fixture("get.json")))

        ping = _pings().create(user_ids=[1049715915, 1049715916])

        assert ping["id"] == 2085958600
        assert json.loads(route.calls.last.request.content) == {"user_ids": [1049715915, 1049715916]}

    @respx.mock
    def test_sends_new_group_when_set(self):
        route = respx.post(f"{BASE}/circles.json").mock(return_value=httpx.Response(201, json=_fixture("get.json")))

        _pings().create(user_ids=[1049715915, 1049715916], new_group=True)

        assert json.loads(route.calls.last.request.content) == {
            "user_ids": [1049715915, 1049715916],
            "new_group": True,
        }

    @respx.mock
    def test_403_when_the_caller_cannot_ping(self):
        respx.post(f"{BASE}/circles.json").mock(return_value=httpx.Response(403, json=DISABLED))

        with pytest.raises(ForbiddenError):
            _pings().create(user_ids=[1049715915])

    @respx.mock
    def test_422_when_nobody_in_the_ping_can_ping(self):
        respx.post(f"{BASE}/circles.json").mock(return_value=httpx.Response(422))

        with pytest.raises(ValidationError):
            _pings().create(user_ids=[1049715915])


@pytest.mark.asyncio
class TestAsync:
    @respx.mock
    async def test_list(self):
        respx.get(f"{BASE}/circles.json").mock(return_value=httpx.Response(200, json=_fixture("list.json")))

        result = await _async_pings().list()

        assert [c["id"] for c in result] == [2085958600, 2085958601]

    @respx.mock
    async def test_get_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/circles/999").mock(return_value=httpx.Response(404, json={"error": "Record not found"}))

        with pytest.raises(NotFoundError):
            await _async_pings().get(ping_id=999)

    @respx.mock
    async def test_create_sends_new_group_only_when_set(self):
        route = respx.post(f"{BASE}/circles.json").mock(return_value=httpx.Response(201, json=_fixture("get.json")))
        pings = _async_pings()

        await pings.create(user_ids=[1])
        await pings.create(user_ids=[1, 2], new_group=True)

        sent = [json.loads(call.request.content) for call in route.calls]
        assert sent == [{"user_ids": [1]}, {"user_ids": [1, 2], "new_group": True}]
