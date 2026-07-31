"""Tests for the heterogeneous /files.json "everything" feed."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import NotFoundError

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


class TestEverythingTaskFilters:
    """assignee_ids[]/due filters on the everything to-do/card family (#12442)."""

    @respx.mock
    def test_grouped_listing_serializes_bracketed_assignee_ids_and_due(self):
        def respond(request: httpx.Request) -> httpx.Response:
            params = request.url.params
            # Bracketed repeated keys — the only form Rails' permit accepts.
            assert params.get_list("assignee_ids[]") == ["11", "22"]
            assert "assignee_ids" not in params
            assert params.get("due") == "overdue"
            return httpx.Response(200, json=[])

        route = respx.get("https://3.basecampapi.com/12345/todos/open.json").mock(side_effect=respond)

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_open_todos(assignee_ids=[11, 22], due="overdue")

        assert route.called
        assert len(result) == 0

    @respx.mock
    def test_overdue_feed_accepts_the_filters(self):
        def respond(request: httpx.Request) -> httpx.Response:
            params = request.url.params
            assert params.get_list("assignee_ids[]") == ["7"]
            assert params.get("due") == "with"
            return httpx.Response(200, json=[])

        route = respx.get("https://3.basecampapi.com/12345/cards/overdue.json").mock(side_effect=respond)

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_overdue_cards(assignee_ids=[7], due="with")

        assert route.called
        assert len(result) == 0


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
        # No X-Total-Count header on this unpaginated feed: total_count must fall
        # back to the array length (not the parser's 0 sentinel).
        assert result.meta.total_count == 2

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

    @respx.mock
    def test_overdue_todos_does_not_follow_link_header(self):
        # The overdue feeds are complete, unpaginated arrays. Even if the server
        # advertises a next page via a Link header, the SDK must issue exactly one
        # request and return only that response — matching the other SDKs' plain
        # full-array decode (no Link-following).
        route = respx.get("https://3.basecampapi.com/12345/todos/overdue.json").mock(
            return_value=httpx.Response(
                200,
                json=_OVERDUE_TODOS_FEED,
                headers={"Link": '<https://3.basecampapi.com/12345/todos/overdue.json?page=2>; rel="next"'},
            )
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_overdue_todos()

        assert route.call_count == 1
        assert len(result) == 2


# Every everything operation must surface a canonical 4xx as a typed error.
_FLAT_ERROR_CASES = [
    ("messages.json", lambda acct: acct.everything.get_everything_messages()),
    ("comments.json", lambda acct: acct.everything.get_everything_comments()),
    ("checkins.json", lambda acct: acct.everything.get_everything_checkins()),
    ("forwards.json", lambda acct: acct.everything.get_everything_forwards()),
    ("files.json", lambda acct: acct.everything.get_everything_files(kind=None, people_ids=None)),
    ("todos/overdue.json", lambda acct: acct.everything.get_everything_overdue_todos()),
    ("cards/overdue.json", lambda acct: acct.everything.get_everything_overdue_cards()),
]

_BUCKET_ERROR_CASES = [
    ("todos/open.json", lambda acct: acct.everything.get_everything_open_todos()),
    ("todos/completed.json", lambda acct: acct.everything.get_everything_completed_todos()),
    ("todos/unassigned.json", lambda acct: acct.everything.get_everything_unassigned_todos()),
    ("todos/no_due_date.json", lambda acct: acct.everything.get_everything_no_due_date_todos()),
    ("cards/open.json", lambda acct: acct.everything.get_everything_open_cards()),
    ("cards/completed.json", lambda acct: acct.everything.get_everything_completed_cards()),
    ("cards/unassigned.json", lambda acct: acct.everything.get_everything_unassigned_cards()),
    ("cards/no_due_date.json", lambda acct: acct.everything.get_everything_no_due_date_cards()),
    ("cards/not_now.json", lambda acct: acct.everything.get_everything_not_now_cards()),
]


class TestEverythingErrorPropagation:
    @pytest.mark.parametrize("path, call", _FLAT_ERROR_CASES + _BUCKET_ERROR_CASES)
    @respx.mock
    def test_operation_propagates_not_found(self, path, call):
        respx.get(f"https://3.basecampapi.com/12345/{path}").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(NotFoundError):
            call(account)


# Bucket-grouped feeds are paginated arrays of `{bucket, todos|cards}` groups.
# Each todo/card is a full recording carrying a `steps` array (a to-do's steps
# or a card's step checklist).
def _todo_group(bucket_id, bucket_name, todos):
    # Real API invariant: each child recording's `bucket` is the group's bucket,
    # so propagate it rather than leaving the _todo default (which weakens the
    # test by letting a group and its children disagree).
    bucket = {"id": bucket_id, "name": bucket_name, "type": "Project"}
    for todo in todos:
        todo["bucket"] = bucket
    return {"bucket": bucket, "todos": todos}


def _card_group(bucket_id, bucket_name, cards):
    bucket = {"id": bucket_id, "name": bucket_name, "type": "Project"}
    for card in cards:
        card["bucket"] = bucket
    return {"bucket": bucket, "cards": cards}


def _todo(todo_id, title, steps):
    return {
        "id": todo_id,
        "type": "Todo",
        "status": "active",
        "visible_to_clients": False,
        "title": title,
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
        "steps": steps,
    }


def _card(card_id, title, steps):
    return {
        "id": card_id,
        "type": "Kanban::Card",
        "status": "active",
        "visible_to_clients": False,
        "title": title,
        "bucket": {"id": 2, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1, "name": "Victor Cooper"},
        "steps": steps,
    }


_OPEN_TODOS_FEED = [
    _todo_group(
        2,
        "The Leto Laptop",
        [
            _todo(2000, "Wire up auth", [{"id": 20000, "title": "Add login form", "completed": False}]),
            _todo(2001, "Write docs", [{"id": 20010, "title": "Draft README", "completed": True}]),
        ],
    ),
    _todo_group(3, "Honcho Design", [_todo(2002, "Design mockups", [])]),
]

_COMPLETED_TODOS_FEED = [
    _todo_group(
        2,
        "The Leto Laptop",
        [_todo(2100, "Ship v1", [{"id": 21000, "title": "Tag release", "completed": True}])],
    ),
]

_UNASSIGNED_TODOS_FEED = [
    _todo_group(
        3,
        "Honcho Design",
        [_todo(2200, "Triage backlog", [{"id": 22000, "title": "Label issues", "completed": False}])],
    ),
]

_NO_DUE_DATE_TODOS_FEED = [
    _todo_group(
        2,
        "The Leto Laptop",
        [_todo(2300, "Someday refactor", [{"id": 23000, "title": "Sketch plan", "completed": False}])],
    ),
]

_OPEN_CARDS_FEED = [
    _card_group(
        2,
        "The Leto Laptop",
        [
            _card(3000, "Build pipeline", [{"id": 30000, "title": "Add CI", "completed": False}]),
            _card(3001, "Add metrics", [{"id": 30010, "title": "Wire Prometheus", "completed": True}]),
        ],
    ),
    _card_group(3, "Honcho Design", [_card(3002, "Draft brief", [])]),
]

_COMPLETED_CARDS_FEED = [
    _card_group(
        2,
        "The Leto Laptop",
        [_card(3100, "Launch site", [{"id": 31000, "title": "Deploy", "completed": True}])],
    ),
]

_UNASSIGNED_CARDS_FEED = [
    _card_group(
        3,
        "Honcho Design",
        [_card(3200, "Pick a name", [{"id": 32000, "title": "Brainstorm", "completed": False}])],
    ),
]

_NO_DUE_DATE_CARDS_FEED = [
    _card_group(
        2,
        "The Leto Laptop",
        [_card(3300, "Backlog idea", [{"id": 33000, "title": "Research", "completed": False}])],
    ),
]

_NOT_NOW_CARDS_FEED = [
    _card_group(
        3,
        "Honcho Design",
        [_card(3400, "Deferred feature", [{"id": 34000, "title": "Revisit Q3", "completed": False}])],
    ),
]


class TestEverythingBucketGroupedTodoFeeds:
    @respx.mock
    def test_open_todos_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/todos/open.json").mock(
            return_value=httpx.Response(200, json=_OPEN_TODOS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_open_todos()

        assert route.called
        assert len(result) == 2
        group = result[0]
        assert group["bucket"] == {"id": 2, "name": "The Leto Laptop", "type": "Project"}
        assert len(group["todos"]) == 2
        assert group["todos"][0]["id"] == 2000
        assert group["todos"][0]["type"] == "Todo"
        assert group["todos"][0]["steps"][0]["title"] == "Add login form"

    @respx.mock
    def test_completed_todos_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/todos/completed.json").mock(
            return_value=httpx.Response(200, json=_COMPLETED_TODOS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_completed_todos()

        assert route.called
        assert len(result) == 1
        group = result[0]
        assert group["bucket"]["id"] == 2
        assert group["todos"][0]["id"] == 2100
        assert group["todos"][0]["steps"][0]["completed"] is True

    @respx.mock
    def test_unassigned_todos_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/todos/unassigned.json").mock(
            return_value=httpx.Response(200, json=_UNASSIGNED_TODOS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_unassigned_todos()

        assert route.called
        assert len(result) == 1
        group = result[0]
        assert group["bucket"]["name"] == "Honcho Design"
        assert group["todos"][0]["id"] == 2200
        assert group["todos"][0]["steps"][0]["title"] == "Label issues"

    @respx.mock
    def test_no_due_date_todos_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/todos/no_due_date.json").mock(
            return_value=httpx.Response(200, json=_NO_DUE_DATE_TODOS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_no_due_date_todos()

        assert route.called
        assert len(result) == 1
        group = result[0]
        assert group["bucket"]["type"] == "Project"
        assert group["todos"][0]["id"] == 2300
        assert group["todos"][0]["steps"][0]["title"] == "Sketch plan"


class TestEverythingBucketGroupedCardFeeds:
    @respx.mock
    def test_open_cards_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/cards/open.json").mock(
            return_value=httpx.Response(200, json=_OPEN_CARDS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_open_cards()

        assert route.called
        assert len(result) == 2
        group = result[0]
        assert group["bucket"] == {"id": 2, "name": "The Leto Laptop", "type": "Project"}
        assert len(group["cards"]) == 2
        assert group["cards"][0]["id"] == 3000
        assert group["cards"][0]["type"] == "Kanban::Card"
        assert group["cards"][0]["steps"][0]["title"] == "Add CI"

    @respx.mock
    def test_completed_cards_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/cards/completed.json").mock(
            return_value=httpx.Response(200, json=_COMPLETED_CARDS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_completed_cards()

        assert route.called
        assert len(result) == 1
        group = result[0]
        assert group["bucket"]["id"] == 2
        assert group["cards"][0]["id"] == 3100
        assert group["cards"][0]["steps"][0]["completed"] is True

    @respx.mock
    def test_unassigned_cards_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/cards/unassigned.json").mock(
            return_value=httpx.Response(200, json=_UNASSIGNED_CARDS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_unassigned_cards()

        assert route.called
        assert len(result) == 1
        group = result[0]
        assert group["bucket"]["name"] == "Honcho Design"
        assert group["cards"][0]["id"] == 3200
        assert group["cards"][0]["steps"][0]["title"] == "Brainstorm"

    @respx.mock
    def test_no_due_date_cards_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/cards/no_due_date.json").mock(
            return_value=httpx.Response(200, json=_NO_DUE_DATE_CARDS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_no_due_date_cards()

        assert route.called
        assert len(result) == 1
        group = result[0]
        assert group["bucket"]["type"] == "Project"
        assert group["cards"][0]["id"] == 3300
        assert group["cards"][0]["steps"][0]["title"] == "Research"

    @respx.mock
    def test_not_now_cards_groups_by_bucket_with_steps(self):
        route = respx.get("https://3.basecampapi.com/12345/cards/not_now.json").mock(
            return_value=httpx.Response(200, json=_NOT_NOW_CARDS_FEED)
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.everything.get_everything_not_now_cards()

        assert route.called
        assert len(result) == 1
        group = result[0]
        assert group["bucket"]["name"] == "Honcho Design"
        assert group["cards"][0]["id"] == 3400
        assert group["cards"][0]["steps"][0]["title"] == "Revisit Q3"
