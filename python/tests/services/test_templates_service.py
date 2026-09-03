"""Tests for generated templates service routes."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ForbiddenError, NotFoundError, PeopleConfirmationRequiredError


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

    @respx.mock
    def test_get_library(self):
        route = respx.get("https://3.basecampapi.com/12345/template_library.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "bucket": {"id": 1, "name": "To-do List Templates", "type": "TemplateLibrary"},
                    "todoset": {"id": 2, "title": "To-do List Templates", "type": "Todoset"},
                    "todolists": [{"id": 3, "name": "Project kickoff"}],
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.get_library()

        assert route.called
        assert result["bucket"]["type"] == "TemplateLibrary"
        assert result["todolists"][0]["name"] == "Project kickoff"

    @respx.mock
    def test_get_library_forbidden(self):
        respx.get("https://3.basecampapi.com/12345/template_library.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(ForbiddenError) as excinfo:
            account.templates.get_library()

        assert excinfo.value.http_status == 403

    @respx.mock
    def test_create_library_copy(self):
        route = respx.post("https://3.basecampapi.com/12345/template_library/copies.json").mock(
            return_value=httpx.Response(
                201,
                json={
                    "id": 5,
                    "status": "pending",
                    "source_recording_id": 3,
                    "destination_parent_id": 9,
                    "url": "https://3.basecampapi.com/12345/template_library/copies/5.json",
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.create_library_copy(
            template_recording_id=3,
            destination_parent_id=9,
            adding_people_confirmed=True,
        )

        body = json.loads(route.calls[0].request.content)
        assert body == {
            "template_recording_id": 3,
            "destination_parent_id": 9,
            "adding_people_confirmed": True,
        }
        assert result["status"] == "pending"
        assert "destination_todolist" not in result

    @respx.mock
    def test_get_completed_library_copy(self):
        route = respx.get("https://3.basecampapi.com/12345/template_library/copies/5").mock(
            return_value=httpx.Response(
                200,
                json={
                    "id": 5,
                    "status": "completed",
                    "source_recording_id": 3,
                    "destination_parent_id": 9,
                    "url": "https://3.basecampapi.com/12345/template_library/copies/5.json",
                    "destination_todolist": {"id": 10, "name": "Project kickoff"},
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.get_library_copy(copy_id=5)

        assert route.called
        assert result["status"] == "completed"
        assert result["destination_todolist"]["id"] == 10

    @respx.mock
    def test_get_library_copy_not_found(self):
        respx.get("https://3.basecampapi.com/12345/template_library/copies/404").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(NotFoundError) as excinfo:
            account.templates.get_library_copy(copy_id=404)

        assert excinfo.value.http_status == 404

    @respx.mock
    def test_create_library_copy_requires_people_confirmation(self):
        respx.post("https://3.basecampapi.com/12345/template_library/copies.json").mock(
            return_value=httpx.Response(
                422,
                json={
                    "error": "Adding people requires confirmation",
                    "people": [{"id": 4, "name": "Victor", "avatar_url": "https://example.test/avatar.png"}],
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(PeopleConfirmationRequiredError) as excinfo:
            account.templates.create_library_copy(template_recording_id=3, destination_parent_id=9)

        assert excinfo.value.http_status == 422
        assert str(excinfo.value) == "Adding people requires confirmation"
        assert excinfo.value.people == [{"id": 4, "name": "Victor", "avatar_url": "https://example.test/avatar.png"}]


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
