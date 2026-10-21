from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.config import Config
from basecamp.errors import NotFoundError, RateLimitError, ValidationError

BASE = "https://3.basecampapi.com/12345"
FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures" / "embeds"
YOUTUBE_URL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"


def _fixture(name: str):
    return json.loads((FIXTURES / name).read_text())


SLASHED_SGID = _fixture("pending.json")["sgid"]


def _embeds(**config):
    return Client(access_token="test-token", config=Config(**config)).for_account("12345").embeds


def _async_embeds():
    return AsyncClient(access_token="test-token").for_account("12345").embeds


class TestCreate:
    @respx.mock
    def test_sends_the_url(self):
        route = respx.post(f"{BASE}/embeds.json").mock(return_value=httpx.Response(200, json=_fixture("pending.json")))

        embed = _embeds().create(url=YOUTUBE_URL)

        assert json.loads(route.calls.last.request.content) == {"url": YOUTUBE_URL}
        assert embed["status"] == "pending"
        assert embed["sgid"] == SLASHED_SGID
        assert "content_type" not in embed

    @respx.mock
    def test_returns_an_existing_active_embed(self):
        respx.post(f"{BASE}/embeds.json").mock(return_value=httpx.Response(200, json=_fixture("active.json")))

        embed = _embeds().create(url=YOUTUBE_URL)

        assert embed["status"] == "active"
        assert embed["content_type"] == "embed/youtube-video"
        assert embed["image_url"] is None

    @respx.mock
    def test_422_for_a_url_no_provider_embeds(self):
        message = "Basecamp can't embed this URL. Link to it instead."
        respx.post(f"{BASE}/embeds.json").mock(return_value=httpx.Response(422, json={"error": message}))

        with pytest.raises(ValidationError) as raised:
            _embeds().create(url="https://example.com/page")

        assert raised.value.http_status == 422
        assert str(raised.value) == message

    @respx.mock
    def test_429_surfaces_on_the_first_attempt(self):
        route = respx.post(f"{BASE}/embeds.json").mock(
            return_value=httpx.Response(429, json={"error": "Too many new embeds. Try again later."})
        )

        with pytest.raises(RateLimitError) as raised:
            _embeds().create(url=YOUTUBE_URL)

        assert raised.value.http_status == 429
        assert route.call_count == 1


class TestGet:
    @respx.mock
    def test_escapes_the_sgid_as_one_path_segment(self):
        route = respx.get(url__regex=r".*/embeds/.*").mock(
            return_value=httpx.Response(200, json=_fixture("active.json"))
        )

        embed = _embeds().get(sgid=SLASHED_SGID)

        assert embed["status"] == "active"
        assert route.calls.last.request.url.raw_path.decode() == (
            "/12345/embeds/" + SLASHED_SGID.replace("/", "%2F").replace("=", "%3D")
        )

    @respx.mock
    def test_404_for_an_unknown_sgid(self):
        respx.get(f"{BASE}/embeds/missing").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _embeds().get(sgid="missing")


@pytest.mark.asyncio
class TestAsync:
    @respx.mock
    async def test_create_and_get(self):
        respx.post(f"{BASE}/embeds.json").mock(return_value=httpx.Response(200, json=_fixture("pending.json")))
        route = respx.get(url__regex=r".*/embeds/.*").mock(
            return_value=httpx.Response(200, json=_fixture("active.json"))
        )
        embeds = _async_embeds()

        created = await embeds.create(url=YOUTUBE_URL)
        fetched = await embeds.get(sgid=created["sgid"])

        assert fetched["status"] == "active"
        assert "%2F" in route.calls.last.request.url.raw_path.decode()
