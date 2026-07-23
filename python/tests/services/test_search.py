"""Tests for the generated search service: array-filter wire encoding + metadata."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client

SEARCH_URL = "https://3.basecampapi.com/12345/search.json"
METADATA_URL = "https://3.basecampapi.com/12345/searches/metadata.json"

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
