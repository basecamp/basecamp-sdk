"""Tests for generated templates service routes."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import (
    ForbiddenError,
    NotFoundError,
    PeopleConfirmationRequiredError,
    ValidationError,
)


def _construction() -> dict:
    return {
        "id": 900,
        "status": "completed",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z",
    }


def _templatification() -> dict:
    return {
        "id": 7,
        "status": "pending",
        "source_recording_id": 2,
        "url": "https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/7.json",
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
    def test_create_project_sends_start_date_under_project_envelope(self):
        route = respx.post("https://3.basecampapi.com/12345/templates/2085958507/project_constructions.json").mock(
            return_value=httpx.Response(201, json=_construction())
        )

        account = Client(access_token="test-token").for_account("12345")
        account.templates.create_project(
            template_id=2085958507,
            project={
                "name": "Marketing Campaign",
                "description": "For Client: Xyz Corp Conference",
                "start_date": "2026-09-01",
            },
        )

        body = json.loads(route.calls[0].request.content)
        assert body == {
            "project": {
                "name": "Marketing Campaign",
                "description": "For Client: Xyz Corp Conference",
                "start_date": "2026-09-01",
            }
        }
        assert "start_date" not in body

    @respx.mock
    def test_get_library_todolists(self):
        route = respx.get("https://3.basecampapi.com/12345/template_library/todolists.json").mock(
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
        result = account.templates.get_library_todolists()

        assert route.called
        assert result["bucket"]["type"] == "TemplateLibrary"
        assert result["todolists"][0]["name"] == "Project kickoff"

    @respx.mock
    def test_get_library_card_tables(self):
        route = respx.get("https://3.basecampapi.com/12345/template_library/card_tables.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "bucket": {"id": 1, "name": "To-do List Templates", "type": "TemplateLibrary"},
                    "kanban_boardset": {
                        "id": 2,
                        "title": "Card Table Templates",
                        "type": "Kanban::Boardset",
                    },
                    "card_tables": [{"id": 3, "title": "Client onboarding", "type": "Kanban::Board"}],
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.get_library_card_tables()

        assert route.called
        assert result["kanban_boardset"]["id"] == 2
        assert result["card_tables"][0]["title"] == "Client onboarding"

    @respx.mock
    def test_get_library_card_tables_without_a_container(self):
        respx.get("https://3.basecampapi.com/12345/template_library/card_tables.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "bucket": {"id": 1, "name": "To-do List Templates", "type": "TemplateLibrary"},
                    "kanban_boardset": None,
                    "card_tables": [],
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.get_library_card_tables()

        assert result["kanban_boardset"] is None
        assert result["card_tables"] == []

    @respx.mock
    def test_create_library_card_table(self):
        route = respx.post("https://3.basecampapi.com/12345/template_library/card_tables.json").mock(
            return_value=httpx.Response(
                201,
                json={
                    "id": 3,
                    "title": "Client onboarding",
                    "type": "Kanban::Board",
                    "parent": {
                        "id": 2,
                        "title": "Card Table Templates",
                        "type": "Kanban::Boardset",
                    },
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.create_library_card_table(name="Client onboarding")

        assert json.loads(route.calls[0].request.content) == {"name": "Client onboarding"}
        assert result["title"] == "Client onboarding"
        assert result["parent"]["id"] == 2

    @respx.mock
    def test_create_library_todolist(self):
        route = respx.post("https://3.basecampapi.com/12345/template_library/todolists.json").mock(
            return_value=httpx.Response(
                201,
                json={
                    "id": 3,
                    "name": "Project kickoff",
                    "type": "Todolist",
                    "description": "<div>Everything to open a project</div>",
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.create_library_todolist(
            name="Project kickoff",
            description="<div>Everything to open a project</div>",
        )

        assert json.loads(route.calls[0].request.content) == {
            "name": "Project kickoff",
            "description": "<div>Everything to open a project</div>",
        }
        assert result["id"] == 3
        assert result["name"] == "Project kickoff"

    @respx.mock
    def test_create_library_todolist_validation_error(self):
        respx.post("https://3.basecampapi.com/12345/template_library/todolists.json").mock(
            return_value=httpx.Response(422, json={"error": "Name can't be blank"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(ValidationError) as excinfo:
            account.templates.create_library_todolist(name="Project kickoff")

        assert excinfo.value.http_status == 422

    @respx.mock
    def test_get_library_todolists_forbidden(self):
        respx.get("https://3.basecampapi.com/12345/template_library/todolists.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(ForbiddenError) as excinfo:
            account.templates.get_library_todolists()

        assert excinfo.value.http_status == 403

    @respx.mock
    def test_get_library_card_tables_forbidden(self):
        respx.get("https://3.basecampapi.com/12345/template_library/card_tables.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(ForbiddenError) as excinfo:
            account.templates.get_library_card_tables()

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

    @respx.mock
    def test_create_templatification_sends_no_optional_keys_when_unset(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications.json").mock(
            return_value=httpx.Response(201, json=_templatification())
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.create_templatification(bucket_id=1, recording_id=2)

        body = json.loads(route.calls[0].request.content or b"{}")
        assert body == {}
        assert "template_name" not in body
        assert "copy_comments" not in body
        assert "copy_assignments" not in body
        assert "move_cards_to_triage" not in body
        assert result["status"] == "pending"
        assert result["source_recording_id"] == 2
        assert "destination_parent_id" not in result

    @respx.mock
    def test_create_templatification_sends_the_optional_save_settings(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications.json").mock(
            return_value=httpx.Response(201, json=_templatification())
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.create_templatification(
            bucket_id=1,
            recording_id=2,
            template_name="Client onboarding",
            copy_comments=True,
            copy_assignments=False,
            move_cards_to_triage=True,
        )

        assert json.loads(route.calls[0].request.content) == {
            "template_name": "Client onboarding",
            "copy_comments": True,
            "copy_assignments": False,
            "move_cards_to_triage": True,
        }
        assert result["id"] == 7

    @respx.mock
    def test_create_templatification_forbidden(self):
        respx.post("https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(ForbiddenError) as excinfo:
            account.templates.create_templatification(bucket_id=1, recording_id=2)

        assert excinfo.value.http_status == 403

    @respx.mock
    def test_get_completed_templatification_with_destination_todolist(self):
        respx.get("https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/7").mock(
            return_value=httpx.Response(
                200,
                json={
                    **_templatification(),
                    "status": "completed",
                    "destination_todolist": {"id": 10, "name": "Project kickoff"},
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.get_templatification(bucket_id=1, recording_id=2, templatification_id=7)

        assert result["status"] == "completed"
        assert result["destination_todolist"]["id"] == 10
        assert "destination_card_table" not in result
        assert "destination_parent_id" not in result

    @respx.mock
    def test_get_completed_templatification_with_destination_card_table(self):
        respx.get("https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/8").mock(
            return_value=httpx.Response(
                200,
                json={
                    **_templatification(),
                    "id": 8,
                    "status": "completed",
                    "destination_card_table": {
                        "id": 11,
                        "title": "Client onboarding",
                        "type": "Kanban::Board",
                    },
                },
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.templates.get_templatification(bucket_id=1, recording_id=2, templatification_id=8)

        assert result["destination_card_table"]["id"] == 11
        assert "destination_todolist" not in result

    @respx.mock
    def test_get_templatification_not_found(self):
        respx.get("https://3.basecampapi.com/12345/buckets/1/recordings/2/templatifications/404").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(NotFoundError) as excinfo:
            account.templates.get_templatification(bucket_id=1, recording_id=2, templatification_id=404)

        assert excinfo.value.http_status == 404


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
