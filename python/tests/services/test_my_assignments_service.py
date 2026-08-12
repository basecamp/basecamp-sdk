"""Tests for the MyAssignmentsService Up Next write operations."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import BasecampError, ForbiddenError, NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"


def _my_assignments():
    return Client(access_token="test-token").for_account("12345").my_assignments


class TestGetMyAssignments:
    @respx.mock
    def test_decodes_assignees_with_full_person_minimal_projection(self):
        # bc3's people/_person_minimal partial renders id, name and avatar_url
        # unconditionally, so an assignee always carries all three keys (#659).
        respx.get(f"{BASE}/my/assignments.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "priorities": [
                        {
                            "id": 1,
                            "content": "Priority task",
                            "assignees": [
                                {
                                    "id": 1049715914,
                                    "name": "Victor Cooper",
                                    "avatar_url": "https://example.com/avatar",
                                }
                            ],
                        }
                    ],
                    "non_priorities": [],
                },
            )
        )

        result = _my_assignments().get_my_assignments()

        assignee = result["priorities"][0]["assignees"][0]
        assert assignee["id"] == 1049715914
        assert assignee["name"] == "Victor Cooper"
        assert assignee["avatar_url"] == "https://example.com/avatar"


class TestPrioritizeAssignment:
    @respx.mock
    def test_posts_the_recording_id(self):
        route = respx.post(f"{BASE}/my/priorities.json").mock(return_value=httpx.Response(204))

        assert _my_assignments().prioritize_assignment(id=1069479801) is None
        assert json.loads(route.calls[-1].request.content) == {"id": 1069479801}

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.post(f"{BASE}/my/priorities.json").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _my_assignments().prioritize_assignment(id=999)


class TestDeprioritizeAssignment:
    @respx.mock
    def test_deletes_the_exact_recording(self):
        route = respx.delete(f"{BASE}/my/priorities/1069479801").mock(return_value=httpx.Response(204))

        assert _my_assignments().deprioritize_assignment(recording_id=1069479801) is None
        assert route.called

    @respx.mock
    def test_403_surfaces_as_forbidden(self):
        respx.delete(f"{BASE}/my/priorities/1069479801").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        with pytest.raises(ForbiddenError):
            _my_assignments().deprioritize_assignment(recording_id=1069479801)


class TestReorderUpNext:
    @respx.mock
    def test_posts_source_and_position(self):
        route = respx.post(f"{BASE}/my/priority_moves.json").mock(return_value=httpx.Response(204))

        assert _my_assignments().reorder_up_next(source_id=1069479801, position=1) is None
        assert json.loads(route.calls[-1].request.content) == {"source_id": 1069479801, "position": 1}

    @respx.mock
    def test_typed_400_surfaces(self):
        respx.post(f"{BASE}/my/priority_moves.json").mock(
            return_value=httpx.Response(400, json={"error": "Position must be an integer."})
        )

        with pytest.raises(BasecampError):
            _my_assignments().reorder_up_next(source_id=1069479801, position=2)

    @respx.mock
    def test_typed_422_surfaces_as_validation_error(self):
        respx.post(f"{BASE}/my/priority_moves.json").mock(
            return_value=httpx.Response(422, json={"error": "Position must be between 1 and 3."})
        )

        with pytest.raises(ValidationError):
            _my_assignments().reorder_up_next(source_id=1069479801, position=99)

    @respx.mock
    def test_bare_bodyless_404_surfaces_as_not_found(self):
        respx.post(f"{BASE}/my/priority_moves.json").mock(return_value=httpx.Response(404))

        with pytest.raises(NotFoundError):
            _my_assignments().reorder_up_next(source_id=999, position=1)
