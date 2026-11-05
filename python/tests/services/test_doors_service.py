"""Tests for the DoorsService (external links).

Bodies are bc3's documented examples (doc/api/sections/external_links.md),
shared under spec/fixtures/doors/ and guarded by ``make check-fixture-coverage``.
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
FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures" / "doors"
DOOR = f"{BASE}/dock/doors/1069480290"


def _fixture(name: str):
    return json.loads((FIXTURES / name).read_text())


def _doors():
    return Client(access_token="test-token").for_account("12345").doors


def _async_doors():
    return AsyncClient(access_token="test-token").for_account("12345").doors


class TestList:
    @respx.mock
    def test_lists_a_projects_external_links(self):
        respx.get(f"{BASE}/buckets/2085958504/dock/doors.json").mock(
            return_value=httpx.Response(200, json=_fixture("list.json"))
        )

        result = _doors().list(bucket_id=2085958504)

        assert len(result) == 1
        assert result[0]["type"] == "Door"
        assert result[0]["position"] == 8

    @respx.mock
    def test_forbidden(self):
        respx.get(f"{BASE}/buckets/1/dock/doors.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        with pytest.raises(ForbiddenError):
            _doors().list(bucket_id=1)


class TestGet:
    @respx.mock
    def test_returns_the_full_shape(self):
        respx.get(DOOR).mock(return_value=httpx.Response(200, json=_fixture("get.json")))

        door = _doors().get(door_id=1069480290)

        assert door["url"] == "https://www.figma.com/file/abc123/Design-system"
        assert door["service"]["code"] == "figma"
        assert door["description"] == "<div>Shared Figma workspace</div>"

    @respx.mock
    def test_not_found(self):
        respx.get(f"{BASE}/dock/doors/9999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _doors().get(door_id=9999)


class TestCreate:
    @respx.mock
    def test_wraps_the_door_and_puts_visibility_beside_it(self):
        request = _fixture("create-request.json")
        route = respx.post(f"{BASE}/buckets/2085958504/dock/doors.json").mock(
            return_value=httpx.Response(201, json=_fixture("get.json"))
        )

        door = _doors().create(
            bucket_id=2085958504,
            door=request["door"],
            visible_to_clients=request["visible_to_clients"],
        )

        assert door["id"] == 1069480290
        assert json.loads(route.calls.last.request.content) == request

    @respx.mock
    def test_omits_visibility_when_not_given(self):
        route = respx.post(f"{BASE}/buckets/1/dock/doors.json").mock(
            return_value=httpx.Response(201, json=_fixture("get.json"))
        )

        _doors().create(bucket_id=1, door={"service": "other", "url": "https://example.com"})

        assert json.loads(route.calls.last.request.content) == {
            "door": {"service": "other", "url": "https://example.com"}
        }

    @respx.mock
    def test_invalid_url(self):
        respx.post(f"{BASE}/buckets/1/dock/doors.json").mock(
            return_value=httpx.Response(422, json={"errors": {"url": ["is invalid"]}})
        )

        with pytest.raises(ValidationError):
            _doors().create(bucket_id=1, door={"service": "other", "url": "javascript:alert(1)"})


class TestUpdate:
    @respx.mock
    def test_sends_the_partial_change(self):
        request = _fixture("update-request.json")
        route = respx.put(DOOR).mock(return_value=httpx.Response(200, json=_fixture("get.json")))

        _doors().update(door_id=1069480290, door=request["door"], visible_to_clients=request["visible_to_clients"])

        assert json.loads(route.calls.last.request.content) == request

    @respx.mock
    def test_visibility_alone(self):
        route = respx.put(DOOR).mock(return_value=httpx.Response(200, json=_fixture("get.json")))

        _doors().update(door_id=1069480290, visible_to_clients=True)

        assert json.loads(route.calls.last.request.content) == {"visible_to_clients": True}


class TestDelete:
    @respx.mock
    def test_deletes(self):
        route = respx.delete(DOOR).mock(return_value=httpx.Response(204))

        assert _doors().delete(door_id=1069480290) is None
        assert route.called

    @respx.mock
    def test_not_found(self):
        respx.delete(f"{BASE}/dock/doors/9999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _doors().delete(door_id=9999)


class TestAsync:
    @respx.mock
    @pytest.mark.asyncio
    async def test_get(self):
        respx.get(DOOR).mock(return_value=httpx.Response(200, json=_fixture("get.json")))

        door = await _async_doors().get(door_id=1069480290)

        assert door["id"] == 1069480290
