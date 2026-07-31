"""Tests for the BookmarksService (my bookmarks + per-recording bookmark toggle)."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import AuthError, ForbiddenError, NotFoundError

BASE = "https://3.basecampapi.com/12345"


def _bookmark(bookmark_id: int) -> dict:
    return {
        "id": bookmark_id,
        "created_at": "2026-07-01T00:00:00Z",
        "updated_at": "2026-07-02T00:00:00Z",
        "recording": {
            "id": 900,
            "status": "active",
            "visible_to_clients": False,
            "created_at": "2026-06-01T00:00:00Z",
            "updated_at": "2026-06-02T00:00:00Z",
            "title": "Kickoff notes",
            "inherits_status": True,
            "type": "Document",
            "url": "https://3.basecampapi.com/12345/buckets/2/documents/900.json",
            "app_url": "https://3.basecamp.com/12345/buckets/2/documents/900",
            "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
            "creator": {"id": 1, "name": "Victor Cooper"},
        },
    }


def _bookmarks():
    return Client(access_token="test-token").for_account("12345").bookmarks


class TestListMyBookmarks:
    @respx.mock
    def test_lists_envelopes_with_wrapped_recordings(self):
        respx.get(f"{BASE}/my/bookmarks.json").mock(return_value=httpx.Response(200, json=[_bookmark(1), _bookmark(2)]))

        result = _bookmarks().list_my_bookmarks()

        assert len(result) == 2
        assert result[0]["id"] == 1
        assert result[0]["recording"]["title"] == "Kickoff notes"

    @respx.mock
    def test_401_surfaces_as_auth_error(self):
        respx.get(f"{BASE}/my/bookmarks.json").mock(return_value=httpx.Response(401, json={"error": "Unauthorized"}))

        with pytest.raises(AuthError):
            _bookmarks().list_my_bookmarks()


class TestGetBookmark:
    @respx.mock
    def test_reports_state(self):
        respx.get(f"{BASE}/recordings/900/bookmark.json").mock(
            return_value=httpx.Response(200, json={"bookmarked": True})
        )

        assert _bookmarks().get_bookmark(recording_id=900)["bookmarked"] is True

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/recordings/999/bookmark.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _bookmarks().get_bookmark(recording_id=999)


class TestCreateBookmark:
    @respx.mock
    def test_returns_envelope(self):
        respx.post(f"{BASE}/recordings/900/bookmark.json").mock(return_value=httpx.Response(201, json=_bookmark(7)))

        bookmark = _bookmarks().create_bookmark(recording_id=900)

        assert bookmark["id"] == 7
        assert bookmark["recording"]["id"] == 900

    @respx.mock
    def test_404_surfaces_as_not_found(self):
        respx.post(f"{BASE}/recordings/999/bookmark.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError):
            _bookmarks().create_bookmark(recording_id=999)


class TestDeleteBookmark:
    @respx.mock
    def test_deletes_with_204(self):
        route = respx.delete(f"{BASE}/recordings/900/bookmark.json").mock(return_value=httpx.Response(204))

        assert _bookmarks().delete_bookmark(recording_id=900) is None
        assert route.called

    @respx.mock
    def test_403_surfaces_as_forbidden(self):
        respx.delete(f"{BASE}/recordings/900/bookmark.json").mock(
            return_value=httpx.Response(403, json={"error": "Forbidden"})
        )

        with pytest.raises(ForbiddenError):
            _bookmarks().delete_bookmark(recording_id=900)
