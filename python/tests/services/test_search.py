"""Tests for the generated search service: array-filter wire encoding, the four
special-cased result branches, and metadata."""

from __future__ import annotations

import json
import pathlib

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client

SEARCH_URL = "https://3.basecampapi.com/12345/search.json"
METADATA_URL = "https://3.basecampapi.com/12345/searches/metadata.json"

# The shared, coverage-guarded body: eight hits covering the generic recording
# envelope and all four branches `api_search_result_template_path` special-cases
# (file attachments, chat lines, kanban lists, gauge needles). Read from disk
# rather than restated inline so this cannot drift from the copy the other five
# SDKs and the conformance runners assert against; `make check-fixture-coverage`
# validates it against the generated SearchResult schema.
_FIXTURES = pathlib.Path(__file__).resolve().parents[3] / "spec/fixtures/search"
_RESULTS = json.loads((_FIXTURES / "results.json").read_text(encoding="utf-8"))

_ATTACHMENT_HIT = next(hit for hit in _RESULTS if "type" not in hit)
_UPLOAD_LINE_HIT = next(hit for hit in _RESULTS if hit.get("type") == "Chat::Lines::Upload")
_KANBAN_HIT = next(hit for hit in _RESULTS if hit.get("type") == "Kanban::Column")
_NEEDLE_HIT = next(hit for hit in _RESULTS if hit.get("type") == "Gauge::Needle")


def _assert_attachment_hit(results: list) -> None:
    """The file-attachment branch (searches/_attachment.json.jbuilder): the one
    branch that writes its own projection instead of decorating the recording
    envelope, so it omits id/title/type/url/app_url entirely and carries the ten
    file keys instead. The absence of those five IS the discriminator."""
    assert len(results) == 1
    hit = results[0]
    for key in ("id", "title", "type", "url", "app_url"):
        assert key not in hit
    assert hit["filename"] == "leto-hero.jpg"
    assert hit["content_type"] == "image/jpeg"
    assert hit["byte_size"] == 512000
    assert hit["previewable"] is True
    # Float-spelled dimensions survive the untyped decode as-is.
    assert hit["width"] == 1920
    assert hit["height"] == 1080
    assert "/previews/" in hit["preview_url"]
    assert "/thumbnails/" in hit["thumbnail_url"]
    assert "/download/" in hit["download_url"]
    assert "/download/" in hit["app_download_url"]
    # Present-and-null on every branch, this one included: the show template
    # nil-overwrites both after rendering the recording's own partial.
    assert "content" in hit and hit["content"] is None
    assert "description" in hit and hit["description"] is None
    assert hit["parent"]["type"] == "Message"


def _assert_upload_line_hit(results: list) -> None:
    """A chat upload line's `attachments` is a BESPOKE six-key aggregate the
    line builds inline — title, url, filename, content_type, byte_size,
    download_url. It is not a RichTextAttachment: no id, no sgid, no preview
    keys. That is why SearchResultAttachment models the union of both variants
    with only the four keys both always emit required."""
    hit = results[0]
    assert hit["type"] == "Chat::Lines::Upload"
    # Chat lines pass `boostable`, so the envelope emits the boost pair.
    assert hit["boosts_count"] == 1
    assert "/boosts.json" in hit["boosts_url"]

    attachment = hit["attachments"][0]
    assert set(attachment) == {
        "title",
        "url",
        "filename",
        "content_type",
        "byte_size",
        "download_url",
    }
    assert attachment["title"] == "leto-benchmarks.pdf"
    assert attachment["content_type"] == "application/pdf"
    assert attachment["byte_size"] == 1048576


def _assert_kanban_hit(results: list) -> None:
    """A kanban (card table) list layers the list partial's keys over the
    recording envelope. `color` is emitted unconditionally with a null value
    when unset, so present-and-null is the normal case, not a malformed one."""
    hit = results[0]
    assert hit["type"] == "Kanban::Column"
    assert hit["cards_count"] == 4
    assert hit["comment_count"] == 1
    assert "/cards.json" in hit["cards_url"]
    assert "color" in hit and hit["color"] is None
    # Envelope keys the list branch reaches: it is subscribable and positioned.
    assert "/subscription.json" in hit["subscription_url"]
    assert hit["position"] == 2
    assert [p["name"] for p in hit["subscribers"]] == ["Victor Cooper"]
    # on_hold is a whole nested list, not a flag.
    assert hit["on_hold"]["cards_count"] == 0
    assert "/cards.json" in hit["on_hold"]["cards_url"]


def _assert_needle_hit(results: list) -> None:
    """A gauge needle is both commentable and boostable, so it carries BOTH
    count pairs — plus the branch partial's own singular `comment_count`, a
    distinct key from the envelope's `comments_count`. Its `attachments` is the
    OTHER variant: the rich-text one, with id and sgid populated."""
    hit = results[0]
    assert hit["type"] == "Gauge::Needle"
    assert hit["comments_count"] == 2
    assert hit["comment_count"] == 2
    assert hit["boosts_count"] == 3
    assert hit["color"] == "green"
    assert hit["position"] == 72
    # description is nil-overwritten, but its companion attachment array is not.
    assert "description" in hit and hit["description"] is None
    assert len(hit["description_attachments"]) == 1

    attachment = hit["attachments"][0]
    assert attachment["id"] == 1069479631
    assert attachment["sgid"].endswith("--srchndl1")
    assert attachment["width"] == 1024
    assert attachment["previewable"] is True


_METADATA = {
    "recording_search_types": [
        {"key": None, "value": "Everything"},
        {"key": "Message", "value": "Messages"},
    ],
    "file_search_types": [
        {"key": None, "value": "All files"},
        {"key": "Image", "value": "Images"},
    ],
    "default_creator_label": "Anyone",
    "default_bucket_label": "All projects",
    "default_circle_label": "All pings",
    "default_file_type_label": "All files",
    "default_type_label": "Everything",
}


def _assert_bracketed_array_wire(request: httpx.Request) -> None:
    """Array filters must serialize as bracketed repeated keys — the only form
    Rails' permit(bucket_ids: []) accepts. Assert on the decoded params."""
    params = request.url.params
    assert params.get_list("bucket_ids[]") == ["1", "2"]
    assert params.get_list("type_names[]") == ["Message", "Todo"]
    assert params.get_list("creator_ids[]") == ["7"]
    # The bare and double-bracketed forms must be absent.
    assert "bucket_ids" not in params
    assert "bucket_ids[][]" not in params
    assert params.get("q") == "hello"


def _full_surface_kwargs() -> dict:
    return {
        "q": "hello",
        "bucket_ids": [1, 2],
        "type_names": ["Message"],
        "creator_ids": [7],
        "file_type": "Image",
        "exclude_chat": True,
        "since": "last_30_days",
        "sort": "recency",
        "type": "Message",
        "bucket_id": 9,
        "creator_id": 3,
    }


def _assert_full_surface_wire(request: httpx.Request) -> None:
    """Every filter param — arrays, scalars, and deprecated singulars — lands
    on the wire with the right key/value."""
    p = request.url.params
    assert p.get_list("bucket_ids[]") == ["1", "2"]
    assert p.get_list("type_names[]") == ["Message"]
    assert p.get_list("creator_ids[]") == ["7"]
    assert p.get("q") == "hello"
    assert p.get("file_type") == "Image"
    assert p.get("exclude_chat") == "true"
    assert p.get("since") == "last_30_days"
    assert p.get("sort") == "recency"
    assert p.get("type") == "Message"
    assert p.get("bucket_id") == "9"
    assert p.get("creator_id") == "3"


class TestSyncSearch:
    @respx.mock
    def test_encodes_array_filters_as_bracketed_keys(self):
        route = respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[]))

        account = Client(access_token="test-token").for_account("12345")
        account.search.search(
            q="hello",
            bucket_ids=[1, 2],
            type_names=["Message", "Todo"],
            creator_ids=[7],
        )

        assert route.called
        _assert_bracketed_array_wire(route.calls[0].request)

    @respx.mock
    def test_encodes_full_filter_surface(self):
        route = respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[]))

        account = Client(access_token="test-token").for_account("12345")
        account.search.search(**_full_surface_kwargs())

        assert route.called
        _assert_full_surface_wire(route.calls[0].request)

    @respx.mock
    def test_attachment_branch_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_ATTACHMENT_HIT]))

        account = Client(access_token="test-token").for_account("12345")
        results = account.search.search(q="leto hero")

        _assert_attachment_hit(list(results))

    @respx.mock
    def test_chat_upload_line_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_UPLOAD_LINE_HIT]))

        account = Client(access_token="test-token").for_account("12345")
        results = account.search.search(q="benchmarks")

        _assert_upload_line_hit(list(results))

    @respx.mock
    def test_kanban_list_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_KANBAN_HIT]))

        account = Client(access_token="test-token").for_account("12345")
        results = account.search.search(q="in progress")

        _assert_kanban_hit(list(results))

    @respx.mock
    def test_gauge_needle_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_NEEDLE_HIT]))

        account = Client(access_token="test-token").for_account("12345")
        results = account.search.search(q="progress update")

        _assert_needle_hit(list(results))

    @respx.mock
    def test_all_branches_decode_together(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=_RESULTS))

        account = Client(access_token="test-token").for_account("12345")
        results = list(account.search.search(q="Leto"))

        assert len(results) == 8
        # bubble_up_url rides the polymorphic projection: todolists/_todolist is
        # the only partial passing bubbleupable: true, so exactly one hit has it.
        assert [r.get("type") for r in results if "bubble_up_url" in r] == ["Todolist"]

    @respx.mock
    def test_metadata_decodes_filter_options(self):
        respx.get(METADATA_URL).mock(return_value=httpx.Response(200, json=_METADATA))

        account = Client(access_token="test-token").for_account("12345")
        metadata = account.search.metadata()

        assert len(metadata["recording_search_types"]) == 2
        # The default "everything" option carries a null key.
        assert metadata["recording_search_types"][0]["key"] is None
        assert metadata["recording_search_types"][1]["value"] == "Messages"
        assert metadata["file_search_types"][1]["key"] == "Image"
        assert metadata["default_creator_label"] == "Anyone"
        assert metadata["default_type_label"] == "Everything"


class TestAsyncSearch:
    @pytest.mark.asyncio
    @respx.mock
    async def test_encodes_array_filters_as_bracketed_keys(self):
        route = respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[]))

        account = AsyncClient(access_token="test-token").for_account("12345")
        await account.search.search(
            q="hello",
            bucket_ids=[1, 2],
            type_names=["Message", "Todo"],
            creator_ids=[7],
        )

        assert route.called
        _assert_bracketed_array_wire(route.calls[0].request)

    @pytest.mark.asyncio
    @respx.mock
    async def test_encodes_full_filter_surface(self):
        route = respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[]))

        account = AsyncClient(access_token="test-token").for_account("12345")
        await account.search.search(**_full_surface_kwargs())

        assert route.called
        _assert_full_surface_wire(route.calls[0].request)

    @pytest.mark.asyncio
    @respx.mock
    async def test_attachment_branch_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_ATTACHMENT_HIT]))

        account = AsyncClient(access_token="test-token").for_account("12345")
        results = await account.search.search(q="leto hero")

        _assert_attachment_hit(list(results))

    @pytest.mark.asyncio
    @respx.mock
    async def test_chat_upload_line_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_UPLOAD_LINE_HIT]))

        account = AsyncClient(access_token="test-token").for_account("12345")
        results = await account.search.search(q="benchmarks")

        _assert_upload_line_hit(list(results))

    @pytest.mark.asyncio
    @respx.mock
    async def test_kanban_list_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_KANBAN_HIT]))

        account = AsyncClient(access_token="test-token").for_account("12345")
        results = await account.search.search(q="in progress")

        _assert_kanban_hit(list(results))

    @pytest.mark.asyncio
    @respx.mock
    async def test_gauge_needle_hit_decodes(self):
        respx.get(SEARCH_URL).mock(return_value=httpx.Response(200, json=[_NEEDLE_HIT]))

        account = AsyncClient(access_token="test-token").for_account("12345")
        results = await account.search.search(q="progress update")

        _assert_needle_hit(list(results))

    @pytest.mark.asyncio
    @respx.mock
    async def test_metadata_decodes_filter_options(self):
        respx.get(METADATA_URL).mock(return_value=httpx.Response(200, json=_METADATA))

        account = AsyncClient(access_token="test-token").for_account("12345")
        metadata = await account.search.metadata()

        assert len(metadata["recording_search_types"]) == 2
        # The default "everything" option carries a null key.
        assert metadata["recording_search_types"][0]["key"] is None
        assert metadata["recording_search_types"][1]["value"] == "Messages"
        assert metadata["file_search_types"][1]["key"] == "Image"
        assert metadata["default_creator_label"] == "Anyone"
        assert metadata["default_type_label"] == "Everything"
