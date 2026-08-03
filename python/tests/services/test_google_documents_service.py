"""Tests for the GoogleDocumentsService.

The create path is the load-bearing assertion: bc3 draws google_documents under
``resources :vaults`` only inside the bucket scope, so create is
``/buckets/{bucket_id}/vaults/{vault_id}/google_documents.json`` while get and
update are flat and unscoped (and carry no ``.json`` suffix, like the other
generated single-resource routes). respx only answers the exact URL, so a wrong
spelling fails here the same way it would 404 in production.
"""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"


def _google_document() -> dict:
    return {
        "id": 2002,
        "status": "active",
        "title": "Roadmap (draft)",
        "type": "GoogleDocument",
        "url": "https://docs.google.com/document/d/abcd1234/edit",
        "app_url": "https://3.basecamp.com/12345/buckets/2085958500/google_documents/2002",
        "description": '<div dir="auto">Quarterly roadmap</div>',
        "description_attachments": [],
        "document_type": "doc",
    }


def _google_documents():
    return Client(access_token="test-token").for_account("12345").google_documents


class TestGoogleDocuments:
    @respx.mock
    def test_get_returns_the_external_link_as_url(self):
        respx.get(f"{BASE}/google_documents/2002").mock(return_value=httpx.Response(200, json=_google_document()))

        result = _google_documents().get_google_document(google_document_id=2002)

        assert result["document_type"] == "doc"
        assert result["url"] == "https://docs.google.com/document/d/abcd1234/edit"
        assert result["app_url"].startswith("https://3.basecamp.com")

    @respx.mock
    def test_get_raises_not_found(self):
        respx.get(f"{BASE}/google_documents/9999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _google_documents().get_google_document(google_document_id=9999)

    @respx.mock
    def test_create_posts_to_the_bucket_scoped_vault_nested_path(self):
        route = respx.post(f"{BASE}/buckets/2085958500/vaults/3001/google_documents.json").mock(
            return_value=httpx.Response(201, json=_google_document())
        )

        result = _google_documents().create_google_document(
            bucket_id=2085958500,
            vault_id=3001,
            url="https://docs.google.com/document/d/abcd1234/edit",
            document_type="doc",
            title="Roadmap",
        )

        assert result["id"] == 2002
        assert "document_type" in route.calls.last.request.content.decode()

    @respx.mock
    def test_create_surfaces_the_document_type_rejection(self):
        """bc3 rejects an unrecognized document_type in a before_action.

        The Rails enum would raise rather than add a validation error, so the
        controller renders the wrapped field-keyed 422 from a literal hash.
        """
        respx.post(f"{BASE}/buckets/2085958500/vaults/3001/google_documents.json").mock(
            return_value=httpx.Response(422, json={"errors": {"document_type": ["is not a valid document type"]}})
        )

        with pytest.raises(ValidationError) as exc:
            _google_documents().create_google_document(
                bucket_id=2085958500,
                vault_id=3001,
                url="https://docs.google.com/document/d/abcd1234/edit",
                document_type="bogus",
            )

        assert exc.value.field_errors == {"document_type": ["is not a valid document type"]}

    @respx.mock
    def test_update_puts_to_the_flat_path(self):
        route = respx.put(f"{BASE}/google_documents/2002").mock(
            return_value=httpx.Response(200, json={**_google_document(), "title": "Roadmap (revised)"})
        )

        result = _google_documents().update_google_document(
            google_document_id=2002,
            url="https://docs.google.com/document/d/abcd1234/edit",
            document_type="doc",
            title="Roadmap (revised)",
        )

        assert result["title"] == "Roadmap (revised)"
        assert "document_type" in route.calls.last.request.content.decode()

    @respx.mock
    def test_update_raises_not_found(self):
        respx.put(f"{BASE}/google_documents/9999").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError):
            _google_documents().update_google_document(
                google_document_id=9999,
                url="https://docs.google.com/d/x",
                document_type="doc",
            )


class TestAsyncGoogleDocuments:
    """The async account client must expose the same service."""

    @respx.mock
    @pytest.mark.asyncio
    async def test_async_google_documents_is_wired(self):
        respx.get(f"{BASE}/google_documents/2002").mock(return_value=httpx.Response(200, json=_google_document()))

        account = AsyncClient(access_token="test-token").for_account("12345")
        google_document = await account.google_documents.get_google_document(google_document_id=2002)

        assert google_document["id"] == 2002
