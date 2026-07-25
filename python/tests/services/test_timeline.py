"""Tests for generated timeline service routes."""

from __future__ import annotations

import httpx
import respx

from basecamp import Client


def _timeline_events() -> list[dict]:
    return [
        {
            "id": 1,
            "created_at": "2024-03-15T10:30:00Z",
            "kind": "chat_transcript_rollup",
            "avatars_sample": [
                "https://3.basecampapi.com/1/people/aaa/avatar",
                "https://3.basecampapi.com/1/people/bbb/avatar",
            ],
        },
        {
            "id": 2,
            "created_at": "2024-03-15T10:31:00Z",
            "kind": "schedule_entry_created",
            "avatars_sample": [],
            "data": {
                "all_day": True,
                "starts_at": "2025-10-30",
                "ends_at": "2025-10-30",
            },
        },
        {
            "id": 3,
            "created_at": "2024-03-15T10:32:00Z",
            "kind": "upload_created",
            "avatars_sample": [],
            "attachments": [
                {
                    "id": 900,
                    "type": "Upload",
                    "status": "active",
                    "visible_to_clients": False,
                    "title": "Diagram",
                    "filename": "diagram.png",
                    "content_type": "image/png",
                    "byte_size": 20480,
                    "width": 1024.0,
                    "height": 768.0,
                    "url": "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
                    "app_url": "https://3.basecamp.com/1/buckets/2/uploads/900",
                    "download_url": "https://3.basecampapi.com/1/buckets/2/uploads/900/download/diagram.png",
                    "app_download_url": "https://3.basecamp.com/1/buckets/2/uploads/900/download",
                }
            ],
        },
        {
            "id": 4,
            "created_at": "2024-03-15T10:33:00Z",
            "kind": "comment_created",
            "avatars_sample": [],
            "attachments": [
                {
                    "id": 500,
                    "attachable_sgid": "sgid-attachable-500",
                    "sgid": "sgid-500",
                    "status_url": "https://3.basecampapi.com/1/attachments/sgid-500/status.json",
                    "caption": "See attached",
                    "filename": "notes.pdf",
                    "content_type": "application/pdf",
                    "byte_size": 4096,
                    "key": "blobkey500",
                    "width": None,
                    "height": None,
                    "previewable": True,
                    "download_url": "https://3.basecampapi.com/1/blobs/blobkey500/download/notes.pdf",
                    "preview_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/full",
                    "thumbnail_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/card",
                }
            ],
        },
    ]


class TestSyncTimeline:
    @respx.mock
    def test_get_project_timeline_decodes_additive_event_fields(self):
        route = respx.get("https://3.basecampapi.com/12345/projects/456/timeline.json").mock(
            return_value=httpx.Response(200, json=_timeline_events())
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.timeline.get_project_timeline(project_id=456)

        assert route.called
        assert len(result) == 4

        # avatars_sample: non-empty sample of avatar URLs.
        assert len(result[0]["avatars_sample"]) == 2

        # data: schedule-entry timing with all-day, date-only starts_at/ends_at.
        assert result[1]["data"]["all_day"] is True
        assert result[1]["data"]["starts_at"] == "2025-10-30"
        assert result[1]["data"]["ends_at"] == "2025-10-30"

        # attachments variant 1: full Upload recording with per-variant fields.
        upload = result[2]["attachments"][0]
        assert upload["type"] == "Upload"
        assert upload["filename"] == "diagram.png"
        assert upload["app_download_url"] == "https://3.basecamp.com/1/buckets/2/uploads/900/download"
        assert upload["width"] == 1024.0

        # attachments variant 2: rich-text attachment/blob partial with per-variant fields.
        blob = result[3]["attachments"][0]
        assert blob["attachable_sgid"] == "sgid-attachable-500"
        assert blob["caption"] == "See attached"
        assert blob["key"] == "blobkey500"
        assert blob["previewable"] is True
        assert blob["width"] is None
