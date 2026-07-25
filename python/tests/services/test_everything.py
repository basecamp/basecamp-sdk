"""Tests for the heterogeneous /files.json "everything" feed."""

from __future__ import annotations

import httpx
import respx

from basecamp import Client

# Canonical heterogeneous feed: a full Upload recording, a Basecamp Document
# recording, and a rich-text Attachment envelope in a single non-empty array.
_FILES_FEED = [
    {
        "id": 900,
        "type": "Upload",
        "status": "active",
        "visible_to_clients": False,
        "title": "logo.png",
        "inherits_status": True,
        "filename": "logo.png",
        "content_type": "image/png",
        "byte_size": 1281,
        "width": 1024.0,
        "height": 768.0,
        "url": "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
        "app_url": "https://3.basecamp.com/1/buckets/2/uploads/900",
        "download_url": "https://3.basecampapi.com/1/buckets/2/uploads/900/download/logo.png",
        "app_download_url": "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
    },
    {
        "id": 901,
        "type": "Document",
        "status": "active",
        "visible_to_clients": False,
        "title": "Spec",
        "inherits_status": True,
        "content_type": "text/html",
        "url": "https://3.basecampapi.com/1/buckets/2/documents/901.json",
        "app_url": "https://3.basecamp.com/1/buckets/2/documents/901",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
    },
    {
        "id": 902,
        "type": "Attachment",
        "attachable_sgid": "sgid-902",
        "filename": "chart.avif",
        "content_type": "image/avif",
        "byte_size": 4096,
        "width": None,
        "height": None,
        "download_url": "https://storage.3.basecamp.com/1/blobs/902/download/chart.avif",
        "parent": {"id": 800, "title": "A message", "type": "Message"},
    },
]


class TestEverythingFiles:
    @respx.mock
    def test_heterogeneous_files_feed_decodes_all_three_variants(self):
        route = respx.get("https://3.basecampapi.com/12345/files.json").mock(
            return_value=httpx.Response(200, json=_FILES_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_files(kind=None, people_ids=None)

        assert route.called
        assert len(result) == 3

        upload = result[0]
        assert upload["type"] == "Upload"
        assert upload["filename"] == "logo.png"
        assert "app_download_url" in upload
        assert upload["width"] == 1024.0

        document = result[1]
        assert document["type"] == "Document"
        assert document["title"] == "Spec"

        attachment = result[2]
        assert attachment["type"] == "Attachment"
        assert attachment["attachable_sgid"] == "sgid-902"
        assert "parent" in attachment
        assert attachment["width"] is None


# A recording root embeds a `bucket` (the project it lives in). Messages,
# comments, check-in entries, and forwards are all recordings.
_MESSAGES_FEED = [
    {
        "id": 1000,
        "type": "Message",
        "status": "active",
        "visible_to_clients": False,
        "title": "Kickoff",
        "content": "<div>Let's go</div>",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
    },
    {
        "id": 1001,
        "type": "Message",
        "status": "active",
        "visible_to_clients": True,
        "title": "Status update",
        "content": "<div>All green</div>",
        "bucket": {"id": 3, "name": "Honcho Design", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
    },
]

_COMMENTS_FEED = [
    {
        "id": 1100,
        "type": "Comment",
        "status": "active",
        "visible_to_clients": False,
        "content": "<div>Nice work</div>",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
    },
    {
        "id": 1101,
        "type": "Comment",
        "status": "active",
        "visible_to_clients": False,
        "content": "<div>Agreed</div>",
        "bucket": {"id": 3, "name": "Honcho Design", "type": "Project"},
        "creator": {"id": 2, "name": "Annie Bryan"},
    },
]

_CHECKINS_FEED = [
    {
        "id": 1200,
        "type": "Question::Answer",
        "status": "active",
        "visible_to_clients": False,
        "content": "<div>Shipped the SDK tests</div>",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
    },
    {
        "id": 1201,
        "type": "Question::Answer",
        "status": "active",
        "visible_to_clients": False,
        "content": "<div>Reviewed PRs</div>",
        "bucket": {"id": 3, "name": "Honcho Design", "type": "Project"},
        "creator": {"id": 2, "name": "Annie Bryan"},
    },
]

_FORWARDS_FEED = [
    {
        "id": 1300,
        "type": "Inbox::Forward",
        "status": "active",
        "visible_to_clients": False,
        "subject": "FW: Invoice",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
    },
    {
        "id": 1301,
        "type": "Inbox::Forward",
        "status": "active",
        "visible_to_clients": False,
        "subject": "FW: Contract",
        "bucket": {"id": 3, "name": "Honcho Design", "type": "Project"},
        "creator": {"id": 2, "name": "Annie Bryan"},
    },
]

# A boost is a tiny reaction wrapping the `recording` it was applied to.
_BOOSTS_FEED = [
    {
        "id": 1400,
        "content": "👍",
        "created_at": "2024-01-01T00:00:00Z",
        "creator": {"id": 1, "name": "Victor Cooper"},
        "recording": {"id": 1000, "type": "Message", "title": "Kickoff"},
    },
    {
        "id": 1401,
        "content": "🔥",
        "created_at": "2024-01-02T00:00:00Z",
        "creator": {"id": 2, "name": "Annie Bryan"},
        "recording": {"id": 1100, "type": "Comment", "title": "Nice work"},
    },
]

# Overdue feeds are unpaginated bare arrays sorted oldest-first by due_on.
_OVERDUE_TODOS_FEED = [
    {
        "id": 1500,
        "type": "Todo",
        "status": "active",
        "visible_to_clients": False,
        "title": "Renew domain",
        "due_on": "2024-01-01",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
    },
    {
        "id": 1501,
        "type": "Todo",
        "status": "active",
        "visible_to_clients": False,
        "title": "File taxes",
        "due_on": "2024-03-15",
        "bucket": {"id": 3, "name": "Honcho Design", "type": "Project"},
    },
]

_OVERDUE_CARDS_FEED = [
    {
        "id": 1600,
        "type": "Kanban::Card",
        "status": "active",
        "visible_to_clients": False,
        "title": "Ship beta",
        "due_on": "2024-02-01",
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
    },
    {
        "id": 1601,
        "type": "Kanban::Card",
        "status": "active",
        "visible_to_clients": False,
        "title": "Cut release",
        "due_on": "2024-04-01",
        "bucket": {"id": 3, "name": "Honcho Design", "type": "Project"},
    },
]


class TestEverythingRecordingFeeds:
    @respx.mock
    def test_messages_feed_embeds_bucket_roots(self):
        route = respx.get("https://3.basecampapi.com/12345/messages.json").mock(
            return_value=httpx.Response(200, json=_MESSAGES_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_messages()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 1000
        assert result[0]["type"] == "Message"
        assert result[0]["bucket"] == {"id": 2, "name": "The Leto Laptop", "type": "Project"}
        assert result[1]["visible_to_clients"] is True

    @respx.mock
    def test_comments_feed_embeds_bucket_roots(self):
        route = respx.get("https://3.basecampapi.com/12345/comments.json").mock(
            return_value=httpx.Response(200, json=_COMMENTS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_comments()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 1100
        assert result[0]["type"] == "Comment"
        assert result[0]["bucket"]["id"] == 2
        assert result[1]["creator"]["name"] == "Annie Bryan"

    @respx.mock
    def test_checkins_feed_embeds_bucket_roots(self):
        route = respx.get("https://3.basecampapi.com/12345/checkins.json").mock(
            return_value=httpx.Response(200, json=_CHECKINS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_checkins()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 1200
        assert result[0]["type"] == "Question::Answer"
        assert result[0]["bucket"]["type"] == "Project"

    @respx.mock
    def test_forwards_feed_embeds_bucket_roots(self):
        route = respx.get("https://3.basecampapi.com/12345/forwards.json").mock(
            return_value=httpx.Response(200, json=_FORWARDS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_forwards()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 1300
        assert result[0]["type"] == "Inbox::Forward"
        assert result[0]["subject"] == "FW: Invoice"
        assert result[0]["bucket"]["id"] == 2

    @respx.mock
    def test_boosts_feed_wraps_nested_recording(self):
        route = respx.get("https://3.basecampapi.com/12345/boosts.json").mock(
            return_value=httpx.Response(200, json=_BOOSTS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_boosts()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 1400
        assert result[0]["content"] == "👍"
        assert result[0]["recording"] == {"id": 1000, "type": "Message", "title": "Kickoff"}
        assert result[1]["recording"]["type"] == "Comment"


class TestEverythingOverdueFeeds:
    @respx.mock
    def test_overdue_todos_returns_oldest_first_bare_array(self):
        route = respx.get("https://3.basecampapi.com/12345/todos/overdue.json").mock(
            return_value=httpx.Response(200, json=_OVERDUE_TODOS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_overdue_todos()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 1500
        assert result[0]["type"] == "Todo"
        assert result[0]["due_on"] == "2024-01-01"
        assert result[0]["due_on"] < result[1]["due_on"]

    @respx.mock
    def test_overdue_cards_returns_oldest_first_bare_array(self):
        route = respx.get("https://3.basecampapi.com/12345/cards/overdue.json").mock(
            return_value=httpx.Response(200, json=_OVERDUE_CARDS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_overdue_cards()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 1600
        assert result[0]["type"] == "Kanban::Card"
        assert result[0]["due_on"] == "2024-02-01"
        assert result[0]["due_on"] < result[1]["due_on"]
