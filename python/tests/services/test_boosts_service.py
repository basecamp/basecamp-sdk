"""Tests for BoostsService.update_boost, the creator-only content replacement."""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ForbiddenError, NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"
FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures" / "boosts"


def _fixture(name: str):
    return json.loads((FIXTURES / name).read_text())


def _boosts():
    return Client(access_token="test-token").for_account("12345").boosts


def _async_boosts():
    return AsyncClient(access_token="test-token").for_account("12345").boosts


class TestUpdateBoost:
    @respx.mock
    def test_replaces_the_content(self):
        route = respx.put(f"{BASE}/boosts/500").mock(return_value=httpx.Response(200, json=_fixture("get.json")))

        boost = _boosts().update_boost(boost_id=500, content="Done!")

        assert boost["id"] == _fixture("get.json")["id"]
        assert json.loads(route.calls.last.request.content) == {"content": "Done!"}

    @respx.mock
    def test_403_when_the_caller_is_not_the_creator(self):
        respx.put(f"{BASE}/boosts/500").mock(return_value=httpx.Response(403))

        with pytest.raises(ForbiddenError):
            _boosts().update_boost(boost_id=500, content="Done!")

    @respx.mock
    def test_404_for_a_missing_boost(self):
        respx.put(f"{BASE}/boosts/500").mock(return_value=httpx.Response(404))

        with pytest.raises(NotFoundError):
            _boosts().update_boost(boost_id=500, content="Done!")

    @respx.mock
    def test_422_when_the_content_is_rejected(self):
        respx.put(f"{BASE}/boosts/500").mock(return_value=httpx.Response(422))

        with pytest.raises(ValidationError):
            _boosts().update_boost(boost_id=500, content="Done!")


@pytest.mark.asyncio
class TestAsync:
    @respx.mock
    async def test_update_boost(self):
        route = respx.put(f"{BASE}/boosts/500").mock(return_value=httpx.Response(200, json=_fixture("get.json")))

        boost = await _async_boosts().update_boost(boost_id=500, content="Done!")

        assert boost["id"] == _fixture("get.json")["id"]
        assert json.loads(route.calls.last.request.content) == {"content": "Done!"}

    @respx.mock
    async def test_update_boost_403_surfaces_as_forbidden(self):
        respx.put(f"{BASE}/boosts/500").mock(return_value=httpx.Response(403))

        with pytest.raises(ForbiddenError):
            await _async_boosts().update_boost(boost_id=500, content="Done!")
