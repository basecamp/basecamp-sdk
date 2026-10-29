"""Tests for the HillChartsService's progress updates.

Bodies are bc3's documented examples (doc/api/sections/hill_charts.md, BC3
#13620), shared under spec/fixtures/hill_charts/ and guarded by
``make check-fixture-coverage``.
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"
FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures" / "hill_charts"
VERSIONS = f"{BASE}/todosets/1069479829/hills/versions.json"


def _fixture(name: str):
    return json.loads((FIXTURES / name).read_text())


def _hill_charts():
    return Client(access_token="test-token").for_account("12345").hill_charts


def _async_hill_charts():
    return AsyncClient(access_token="test-token").for_account("12345").hill_charts


class TestGet:
    @respx.mock
    def test_carries_versions_url(self):
        respx.get(f"{BASE}/todosets/1069479829/hill.json").mock(
            return_value=httpx.Response(200, json=_fixture("get.json"))
        )

        chart = _hill_charts().get(todoset_id=1069479829)

        assert chart["versions_url"] == (
            "https://3.basecampapi.com/195539477/buckets/2085958505/todosets/1069479829/hills/versions.json"
        )


class TestListVersions:
    @respx.mock
    def test_lists_progress_updates(self):
        respx.get(VERSIONS).mock(
            return_value=httpx.Response(200, json=_fixture("versions.json"), headers={"X-Total-Count": "1"})
        )

        result = _hill_charts().list_versions(todoset_id=1069479829)

        assert len(result) == 1
        assert result.meta.total_count == 1
        version = result[0]
        assert version["id"] == 1069480194
        assert version["type"] == "Hill::Version"
        assert "Research is wrapping up." in version["description"]
        assert version["description_attachments"] == []
        assert version["dots"] == [
            {
                "id": 1069479863,
                "label": "Background and research",
                "color": "blue",
                "position": 40,
                "url": "https://3.basecampapi.com/195539477/buckets/2085958505/todolists/1069479863.json",
                "app_url": "https://3.basecamp.com/195539477/buckets/2085958505/todolists/1069479863",
            }
        ]

    @respx.mock
    def test_follows_the_link_header(self):
        first = f"{BASE}/todosets/1/hills/versions.json"
        respx.get(first, params={"page": "2"}).mock(return_value=httpx.Response(200, json=_fixture("versions.json")))
        respx.get(first).mock(
            return_value=httpx.Response(
                200, json=_fixture("versions.json"), headers={"Link": f'<{first}?page=2>; rel="next"'}
            )
        )

        assert len(_hill_charts().list_versions(todoset_id=1)) == 2

    @respx.mock
    def test_404_is_not_found(self):
        respx.get(f"{BASE}/todosets/999/hills/versions.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _hill_charts().list_versions(todoset_id=999)

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_lists_progress_updates(self):
        respx.get(VERSIONS).mock(return_value=httpx.Response(200, json=_fixture("versions.json")))

        result = await _async_hill_charts().list_versions(todoset_id=1069479829)

        assert [v["id"] for v in result] == [1069480194]


class TestGetVersion:
    @respx.mock
    def test_gets_one_progress_update(self):
        respx.get(f"{BASE}/todosets/1069479829/hills/versions/1069480194").mock(
            return_value=httpx.Response(200, json=_fixture("version.json"))
        )

        version = _hill_charts().get_version(todoset_id=1069479829, version_id=1069480194)

        assert version["id"] == 1069480194
        assert version["creator"]["name"] == "Victor Cooper"
        assert version["dots"][0]["position"] == 40

    @respx.mock
    def test_a_dot_whose_todolist_is_gone(self):
        respx.get(f"{BASE}/todosets/1069479829/hills/versions/1069480194").mock(
            return_value=httpx.Response(200, json=_fixture("version_todolist_gone.json"))
        )

        gone = _hill_charts().get_version(todoset_id=1069479829, version_id=1069480194)["dots"][1]

        assert gone == {"id": None, "label": "Old research list", "color": "green", "position": 100}

    @respx.mock
    def test_404_is_not_found(self):
        respx.get(f"{BASE}/todosets/1/hills/versions/999").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _hill_charts().get_version(todoset_id=1, version_id=999)

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_404_is_not_found(self):
        respx.get(f"{BASE}/todosets/1/hills/versions/999").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            await _async_hill_charts().get_version(todoset_id=1, version_id=999)


class TestCreateVersion:
    @respx.mock
    def test_posts_the_documented_request(self):
        request = _fixture("create-version-request.json")
        route = respx.post(VERSIONS).mock(return_value=httpx.Response(201, json=_fixture("create-version.json")))

        version = _hill_charts().create_version(todoset_id=1069479829, **request)

        assert json.loads(route.calls.last.request.content) == request
        assert version["id"] == 1069480194
        assert version["description"] == '<div dir="auto">Research is done. Building starts Monday.</div>'
        assert version["dots"][0]["position"] == 55

    @respx.mock
    def test_nothing_given_posts_an_empty_object(self):
        route = respx.post(VERSIONS).mock(return_value=httpx.Response(201, json=_fixture("create-version.json")))

        _hill_charts().create_version(todoset_id=1069479829)

        assert json.loads(route.calls.last.request.content) == {}

    @respx.mock
    def test_422_is_validation_with_the_servers_message(self):
        respx.post(VERSIONS).mock(
            return_value=httpx.Response(422, json={"error": "Not tracked on the hill chart: 1069479511"})
        )

        with pytest.raises(ValidationError, match="Not tracked on the hill chart: 1069479511"):
            _hill_charts().create_version(todoset_id=1069479829, dots=[{"id": 1069479511, "position": 50}])

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_posts_the_documented_request(self):
        request = _fixture("create-version-request.json")
        route = respx.post(VERSIONS).mock(return_value=httpx.Response(201, json=_fixture("create-version.json")))

        version = await _async_hill_charts().create_version(todoset_id=1069479829, **request)

        assert json.loads(route.calls.last.request.content) == request
        assert version["dots"][0]["position"] == 55
