# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class DocumentsService(BaseService):
    def get(self, *, document_id: int) -> dict[str, Any]:
        """Get a single document by id.

        Args:
            document_id: The document id.
        """
        return self._request(
            OperationInfo(service="documents", operation="get", is_mutation=False, resource_id=document_id),
            "GET",
            f"/documents/{document_id}",
            operation="GetDocument",
        )

    def replace(self, *, document_id: int, title: str | None = None, content: str | None = None) -> dict[str, Any]:
        """Replace a document with a new complete representation.
        The request body is the document's full writable state: any writable field
        omitted from the request is cleared server-side. Omitting content clears it;
        omitting title clears it too, and the document then reads back as
        "Untitled" (Document#title falls back when blank).
        Neither field is required. BC3 builds a brand-new Document from the
        permitted params and swaps the recordable wholesale, and neither attribute
        carries a presence validation — so an omission is a 200 that clears, not a
        422. What BC3 does require is the wrapping document object, which Rails
        synthesizes from a flat body, so a request naming neither field is a 400.
        Publishing a draft (status: "active") is not modeled: the SDK sends only
        title and content, and BC3 rejects a status-only update for the same
        reason it 400s an empty body.
        Subscribers are the one exception to omission-clears. A drafted document
        keeps its current subscribers when the request addresses neither
        subscriptions nor notify, so a full-representation PUT that mentions
        neither is safe on a draft.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current document and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            document_id: The document id.
            title: The title.
            content: The content.
        """
        return self._request(
            OperationInfo(service="documents", operation="replace", is_mutation=True, resource_id=document_id),
            "PUT",
            f"/documents/{document_id}",
            json_body=self._compact(title=title, content=content),
            operation="ReplaceDocument",
        )

    def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List documents in a vault.

        Args:
            vault_id: The vault id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="documents", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/documents.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListDocuments",
        )

    def create(
        self,
        *,
        vault_id: int,
        title: str,
        content: str | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new document in a vault.

        Args:
            vault_id: The vault id.
            title: The title.
            content: The content.
            status: active|drafted
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return self._request(
            OperationInfo(service="documents", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/documents.json",
            json_body=self._compact(
                title=title,
                content=content,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateDocument",
        )


class AsyncDocumentsService(AsyncBaseService):
    async def get(self, *, document_id: int) -> dict[str, Any]:
        """Get a single document by id.

        Args:
            document_id: The document id.
        """
        return await self._request(
            OperationInfo(service="documents", operation="get", is_mutation=False, resource_id=document_id),
            "GET",
            f"/documents/{document_id}",
            operation="GetDocument",
        )

    async def replace(
        self, *, document_id: int, title: str | None = None, content: str | None = None
    ) -> dict[str, Any]:
        """Replace a document with a new complete representation.
        The request body is the document's full writable state: any writable field
        omitted from the request is cleared server-side. Omitting content clears it;
        omitting title clears it too, and the document then reads back as
        "Untitled" (Document#title falls back when blank).
        Neither field is required. BC3 builds a brand-new Document from the
        permitted params and swaps the recordable wholesale, and neither attribute
        carries a presence validation — so an omission is a 200 that clears, not a
        422. What BC3 does require is the wrapping document object, which Rails
        synthesizes from a flat body, so a request naming neither field is a 400.
        Publishing a draft (status: "active") is not modeled: the SDK sends only
        title and content, and BC3 rejects a status-only update for the same
        reason it 400s an empty body.
        Subscribers are the one exception to omission-clears. A drafted document
        keeps its current subscribers when the request addresses neither
        subscriptions nor notify, so a full-representation PUT that mentions
        neither is safe on a draft.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current document and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            document_id: The document id.
            title: The title.
            content: The content.
        """
        return await self._request(
            OperationInfo(service="documents", operation="replace", is_mutation=True, resource_id=document_id),
            "PUT",
            f"/documents/{document_id}",
            json_body=self._compact(title=title, content=content),
            operation="ReplaceDocument",
        )

    async def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List documents in a vault.

        Args:
            vault_id: The vault id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="documents", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/documents.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListDocuments",
        )

    async def create(
        self,
        *,
        vault_id: int,
        title: str,
        content: str | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new document in a vault.

        Args:
            vault_id: The vault id.
            title: The title.
            content: The content.
            status: active|drafted
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return await self._request(
            OperationInfo(service="documents", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/documents.json",
            json_body=self._compact(
                title=title,
                content=content,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateDocument",
        )
