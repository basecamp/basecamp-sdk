"""Tests for message type (category) operations (sync + async).

Message types are bucket-scoped: every operation requires a project id and
hits /buckets/{project_id}/categories(.json). See #368.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import NotFoundError

PROJECT_ID = 89
BASE = "https://3.basecampapi.com/12345"


def _type(type_id: int = 1, name: str = "Announcement", icon: str = "\U0001f4e2") -> dict:
    return {"id": type_id, "name": name, "icon": icon}


class TestSyncMessageTypes:
    @respx.mock
    def test_list(self):
        route = respx.get(f"{BASE}/buckets/{PROJECT_ID}/categories.json").mock(
            return_value=httpx.Response(200, json=[_type(1), _type(2, "Question", "❓")])
        )

        c = Client(access_token="test-token")
        types = c.for_account("12345").message_types.list(project_id=PROJECT_ID)
        c.close()

        assert route.called
        assert len(types) == 2
        assert types[0]["name"] == "Announcement"
        assert types[1]["name"] == "Question"

    @respx.mock
    def test_get(self):
        route = respx.get(f"{BASE}/buckets/{PROJECT_ID}/categories/1").mock(return_value=httpx.Response(200, json=_type()))

        c = Client(access_token="test-token")
        mt = c.for_account("12345").message_types.get(project_id=PROJECT_ID, type_id=1)
        c.close()

        assert route.called
        assert mt["id"] == 1
        assert mt["name"] == "Announcement"

    @respx.mock
    def test_create(self):
        route = respx.post(f"{BASE}/buckets/{PROJECT_ID}/categories.json").mock(
            return_value=httpx.Response(201, json=_type(3, "Update", "\U0001f504"))
        )

        c = Client(access_token="test-token")
        mt = c.for_account("12345").message_types.create(project_id=PROJECT_ID, name="Update", icon="\U0001f504")
        c.close()

        assert route.called
        body = json.loads(route.calls.last.request.content)
        assert body == {"name": "Update", "icon": "\U0001f504"}
        assert mt["id"] == 3

    @respx.mock
    def test_update(self):
        route = respx.put(f"{BASE}/buckets/{PROJECT_ID}/categories/1").mock(
            return_value=httpx.Response(200, json=_type(1, "Important", "\U0001f4e3"))
        )

        c = Client(access_token="test-token")
        mt = c.for_account("12345").message_types.update(project_id=PROJECT_ID, type_id=1, name="Important")
        c.close()

        assert route.called
        assert mt["name"] == "Important"

    @respx.mock
    def test_delete(self):
        route = respx.delete(f"{BASE}/buckets/{PROJECT_ID}/categories/1").mock(return_value=httpx.Response(204))

        c = Client(access_token="test-token")
        result = c.for_account("12345").message_types.delete(project_id=PROJECT_ID, type_id=1)
        c.close()

        assert route.called
        assert result is None

    @respx.mock
    def test_get_not_found(self):
        respx.get(f"{BASE}/buckets/{PROJECT_ID}/categories/999").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        c = Client(access_token="test-token")
        with pytest.raises(NotFoundError):
            c.for_account("12345").message_types.get(project_id=PROJECT_ID, type_id=999)
        c.close()


class TestAsyncMessageTypes:
    @respx.mock
    @pytest.mark.asyncio
    async def test_list(self):
        route = respx.get(f"{BASE}/buckets/{PROJECT_ID}/categories.json").mock(
            return_value=httpx.Response(200, json=[_type(1), _type(2, "Question", "❓")])
        )

        c = AsyncClient(access_token="test-token")
        types = await c.for_account("12345").message_types.list(project_id=PROJECT_ID)
        await c.close()

        assert route.called
        assert len(types) == 2
        assert types[0]["name"] == "Announcement"

    @respx.mock
    @pytest.mark.asyncio
    async def test_create(self):
        route = respx.post(f"{BASE}/buckets/{PROJECT_ID}/categories.json").mock(
            return_value=httpx.Response(201, json=_type(3, "Update", "\U0001f504"))
        )

        c = AsyncClient(access_token="test-token")
        mt = await c.for_account("12345").message_types.create(project_id=PROJECT_ID, name="Update", icon="\U0001f504")
        await c.close()

        assert route.called
        assert mt["id"] == 3
