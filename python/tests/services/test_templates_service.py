"""Tests for generated templates service routes."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client


def _construction() -> dict:
    return {
        "id": 900,
        "status": "completed",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z",
    }


class TestSyncTemplates:
    @respx.mock
    def test_create_project_nests_body_under_project_envelope(self):
        route = respx.post("https://3.basecampapi.com/12345/templates/456/project_constructions.json").mock(
            return_value=httpx.Response(201, json=_construction())
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.create_project(
            template_id=456,
            project={"name": "New Project", "description": "From template"},
        )

        assert route.called
        request = route.calls[0].request
        assert request.method == "POST"
        body = json.loads(request.content)
        assert body == {"project": {"name": "New Project", "description": "From template"}}
        assert "name" not in body
        assert result["id"] == 900


class TestAsyncTemplates:
    @pytest.mark.asyncio
    @respx.mock
    async def test_create_project_nests_body_under_project_envelope(self):
        route = respx.post("https://3.basecampapi.com/12345/templates/456/project_constructions.json").mock(
            return_value=httpx.Response(201, json=_construction())
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.templates.create_project(
            template_id=456,
            project={"name": "New Project", "description": "From template"},
        )

        assert route.called
        request = route.calls[0].request
        assert request.method == "POST"
        body = json.loads(request.content)
        assert body == {"project": {"name": "New Project", "description": "From template"}}
        assert "name" not in body
        assert result["id"] == 900
