"""Tests for the DraftsService (the current user's unpublished drafts)."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import AuthError

BASE = "https://3.basecampapi.com/12345"


def _draft(draft_id: int, **overrides) -> dict:
    return {
        "id": draft_id,
        "app_url": f"https://3.basecamp.com/12345/buckets/2/documents/{draft_id}",
        "title": "Quarterly plan",
        "type": "document",
        "bucket": {"id": 2, "name": "The Leto Laptop", "app_url": "https://3.basecamp.com/12345/projects/2"},
        "parent": {"id": 500, "title": "Docs & Files", "app_url": "https://3.basecamp.com/12345/buckets/2/vaults/500"},
        "excerpt": "First 300 chars of the body",
        "created_at": "2026-07-01T00:00:00Z",
        "updated_at": "2026-07-02T00:00:00Z",
        "scheduled_posting_at": None,
        **overrides,
    }


def _drafts():
    return Client(access_token="test-token").for_account("12345").drafts


class TestListMyDrafts:
    @respx.mock
    def test_lists_envelopes_with_null_parent_and_schedule(self):
        respx.get(f"{BASE}/my/drafts.json").mock(
            return_value=httpx.Response(
                200,
                json=[
                    _draft(1),
                    _draft(2, parent=None, scheduled_posting_at="2026-08-01T09:00:00Z"),
                ],
            )
        )

        result = _drafts().list_my_drafts()

        assert len(result) == 2
        assert result[0]["parent"]["title"] == "Docs & Files"
        assert result[0]["scheduled_posting_at"] is None
        # Bucket-rooted draft: parent is present-but-null, not absent.
        assert result[1]["parent"] is None
        assert result[1]["scheduled_posting_at"] == "2026-08-01T09:00:00Z"

    @respx.mock
    def test_401_surfaces_as_auth_error(self):
        respx.get(f"{BASE}/my/drafts.json").mock(return_value=httpx.Response(401, json={"error": "Unauthorized"}))

        with pytest.raises(AuthError):
            _drafts().list_my_drafts()
