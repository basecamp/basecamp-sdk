"""Tests for the CloudFilesService.

The create path is the load-bearing assertion: bc3 draws cloud_files under
``resources :vaults`` only inside the bucket scope, so create is
``/buckets/{bucket_id}/vaults/{vault_id}/cloud_files.json`` while get and update
are flat and unscoped (and carry no ``.json`` suffix, like the other generated
single-resource routes). respx only answers the exact URL, so a wrong spelling
fails here the same way it would 404 in production.
"""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"


def _cloud_file() -> dict:
    return {
        "id": 1001,
        "status": "active",
        "title": "Brand book draft",
        "type": "CloudFile",
        # The EXTERNAL link, not this record's API url — the cloud_files
        # jbuilder renders the recording partial and then overwrites :url with
        # the recordable's.
        "url": "https://www.dropbox.com/s/abcd1234/brand-draft.pdf",
        "app_url": "https://3.basecamp.com/12345/buckets/2085958500/cloud_files/1001",
        "description": '<div dir="auto">Working draft</div>',
        "description_attachments": [],
        "service": {
            "name": "Dropbox",
            "example_url": "https://www.dropbox.com/s/abcd1234/example.pdf",
            "code": "dropbox",
            "valid_patterns": ["(.*?\\.)?dropbox\\.com(\\/.*)?"],
            "supporting_text": "a file or folder on Dropbox",
        },
    }


def _cloud_files():
    return Client(access_token="test-token").for_account("12345").cloud_files


class TestCloudFiles:
    @respx.mock
    def test_get_returns_the_external_link_as_url(self):
        respx.get(f"{BASE}/cloud_files/1001").mock(return_value=httpx.Response(200, json=_cloud_file()))

        result = _cloud_files().get_cloud_file(cloud_file_id=1001)

        assert result["title"] == "Brand book draft"
        assert result["url"] == "https://www.dropbox.com/s/abcd1234/brand-draft.pdf"
        assert result["app_url"].startswith("https://3.basecamp.com")
        assert result["service"]["code"] == "dropbox"

    @respx.mock
    def test_get_raises_not_found(self):
        respx.get(f"{BASE}/cloud_files/9999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _cloud_files().get_cloud_file(cloud_file_id=9999)

    @respx.mock
    def test_create_posts_to_the_bucket_scoped_vault_nested_path(self):
        route = respx.post(f"{BASE}/buckets/2085958500/vaults/3001/cloud_files.json").mock(
            return_value=httpx.Response(201, json=_cloud_file())
        )

        result = _cloud_files().create_cloud_file(
            bucket_id=2085958500,
            vault_id=3001,
            url="https://www.dropbox.com/s/abcd1234/brand.zip",
            service="dropbox",
            title="Brand assets",
        )

        assert result["id"] == 1001
        body = route.calls.last.request.content.decode()
        assert '"service":"dropbox"' in body.replace(" ", "")
        assert "brand.zip" in body

    @respx.mock
    def test_create_surfaces_the_field_keyed_422(self):
        respx.post(f"{BASE}/buckets/2085958500/vaults/3001/cloud_files.json").mock(
            return_value=httpx.Response(422, json={"errors": {"url": ["is not a valid Dropbox url"]}})
        )

        with pytest.raises(ValidationError) as exc:
            _cloud_files().create_cloud_file(
                bucket_id=2085958500,
                vault_id=3001,
                url="https://example.com/nope",
                service="dropbox",
            )

        assert exc.value.field_errors == {"url": ["is not a valid Dropbox url"]}

    @respx.mock
    def test_update_puts_to_the_flat_path(self):
        route = respx.put(f"{BASE}/cloud_files/1001").mock(
            return_value=httpx.Response(200, json={**_cloud_file(), "title": "Brand assets v2"})
        )

        result = _cloud_files().update_cloud_file(
            cloud_file_id=1001,
            url="https://www.dropbox.com/s/abcd1234/brand-v2.zip",
            service="dropbox",
            title="Brand assets v2",
        )

        assert result["title"] == "Brand assets v2"
        assert "brand-v2.zip" in route.calls.last.request.content.decode()

    @respx.mock
    def test_update_raises_not_found(self):
        respx.put(f"{BASE}/cloud_files/9999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _cloud_files().update_cloud_file(
                cloud_file_id=9999,
                url="https://www.dropbox.com/s/a/b.pdf",
                service="dropbox",
            )


class TestAsyncCloudFiles:
    """The async account client must expose the same service."""

    @respx.mock
    @pytest.mark.asyncio
    async def test_async_cloud_files_is_wired(self):
        respx.get(f"{BASE}/cloud_files/1001").mock(return_value=httpx.Response(200, json=_cloud_file()))

        account = AsyncClient(access_token="test-token").for_account("12345")
        cloud_file = await account.cloud_files.get_cloud_file(cloud_file_id=1001)

        assert cloud_file["id"] == 1001
