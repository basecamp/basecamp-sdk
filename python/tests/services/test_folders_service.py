"""Tests for the FoldersService (home-screen folders; wire type "Stack")."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import AuthError, NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"


def _folder(folder_id: int, name: str = "Client work") -> dict:
    """The list shape: base fields, no ``projects``. ``type`` is "Stack"."""
    return {
        "id": folder_id,
        "name": name,
        "type": "Stack",
        "created_at": "2026-07-27T10:16:49.312Z",
        "updated_at": "2026-07-27T10:16:49.325Z",
        "bucket_ids": [201, 202],
        "is_emoji_only_name": False,
        "star_url": f"{BASE}/buckets/{folder_id}/stars.json",
        "gauges_url": None,
        "color": None,
        "image_url": None,
        "url": f"{BASE}/stacks/{folder_id}.json",
    }


def _project(project_id: int, name: str) -> dict:
    return {
        "id": project_id,
        "status": "active",
        "created_at": "2026-06-01T00:00:00Z",
        "updated_at": "2026-06-02T00:00:00Z",
        "name": name,
        "description": "",
        "purpose": "topic",
        "clients_enabled": False,
        "bookmark_url": f"{BASE}/my/bookmarks/abc.json",
        "url": f"{BASE}/projects/{project_id}.json",
        "app_url": f"https://3.basecamp.com/12345/projects/{project_id}",
    }


def _folder_with_projects(folder_id: int, name: str = "Client work") -> dict:
    """The get/create/update shape: the base folder plus expanded projects."""
    return {
        **_folder(folder_id, name),
        "projects": [_project(201, "Refresh"), _project(202, "Nike promotion")],
    }


def _folders():
    return Client(access_token="test-token").for_account("12345").folders


class TestListFolders:
    @respx.mock
    def test_lists_a_bare_array_without_projects(self):
        respx.get(f"{BASE}/stacks.json").mock(
            return_value=httpx.Response(200, json=[_folder(1), _folder(2, "Personal")])
        )

        result = _folders().list_folders()

        assert len(result) == 2
        assert result[0]["id"] == 1
        assert result[0]["type"] == "Stack"
        assert result[0]["bucket_ids"] == [201, 202]
        assert "projects" not in result[0]

    @respx.mock
    def test_decodes_the_always_present_nullable_fields(self):
        respx.get(f"{BASE}/stacks.json").mock(return_value=httpx.Response(200, json=[_folder(1)]))

        folder = _folders().list_folders()[0]

        assert "gauges_url" in folder
        assert folder["gauges_url"] is None
        assert folder["color"] is None
        assert folder["image_url"] is None

    @respx.mock
    def test_401_surfaces_as_auth_error(self):
        respx.get(f"{BASE}/stacks.json").mock(return_value=httpx.Response(401, json={"error": "Unauthorized"}))

        with pytest.raises(AuthError):
            _folders().list_folders()


class TestGetFolder:
    @respx.mock
    def test_expands_its_projects(self):
        respx.get(f"{BASE}/stacks/1").mock(return_value=httpx.Response(200, json=_folder_with_projects(1)))

        folder = _folders().get_folder(folder_id=1)

        assert len(folder["projects"]) == 2
        assert folder["projects"][0]["name"] == "Refresh"
        assert folder["bucket_ids"] == [p["id"] for p in folder["projects"]]

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/stacks/999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _folders().get_folder(folder_id=999)


class TestCreateFolder:
    @respx.mock
    def test_sends_project_ids_and_returns_the_expanded_folder(self):
        route = respx.post(f"{BASE}/stacks.json").mock(return_value=httpx.Response(201, json=_folder_with_projects(7)))

        folder = _folders().create_folder(name="Client work", project_ids=[201, 202])

        assert route.calls.last.request.read() == b'{"name":"Client work","project_ids":[201,202]}'
        assert folder["id"] == 7
        assert len(folder["projects"]) == 2

    @respx.mock
    def test_unreachable_project_id_is_a_zero_write_404(self):
        respx.post(f"{BASE}/stacks.json").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _folders().create_folder(name="Mixed", project_ids=[201, 999999999])


class TestUpdateFolder:
    @respx.mock
    def test_renames_and_returns_projects(self):
        respx.put(f"{BASE}/stacks/1").mock(
            return_value=httpx.Response(200, json=_folder_with_projects(1, "Active client work"))
        )

        folder = _folders().update_folder(folder_id=1, name="Active client work")

        assert folder["name"] == "Active client work"
        assert len(folder["projects"]) == 2

    @respx.mock
    def test_blank_name_surfaces_as_validation_error(self):
        respx.put(f"{BASE}/stacks/1").mock(
            return_value=httpx.Response(422, json={"errors": {"name": ["can't be blank"]}})
        )

        with pytest.raises(ValidationError):
            _folders().update_folder(folder_id=1, name="   ")


class TestDeleteFolder:
    @respx.mock
    def test_deletes_with_204(self):
        route = respx.delete(f"{BASE}/stacks/1").mock(return_value=httpx.Response(204))

        assert _folders().delete_folder(folder_id=1) is None
        assert route.called

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.delete(f"{BASE}/stacks/999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _folders().delete_folder(folder_id=999)
