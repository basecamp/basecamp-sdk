# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class GoogleDocumentsService(BaseService):
    def create_google_document(
        self,
        *,
        bucket_id: int,
        vault_id: int,
        url: str,
        document_type: str,
        title: str | None = None,
        description: str | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new Google document in a vault.
        An unrecognized `document_type` is rejected before validation with the
        field-keyed 422 {"errors": {"document_type": ["is not a valid document
        type"]}} — the enum would otherwise raise rather than add an error.
        Omitting `title` is allowed and reads back as "Untitled".

        Args:
            bucket_id: The bucket id.
            vault_id: The vault id.
            url: The url.
            document_type: One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
                unrecognized value is rejected up front with a field-keyed 422 ({"errors":
                {"document_type": ["is not a valid document type"]}}) rather than reaching
                validation.
            title: The title.
            description: The description.
            status: active|drafted — defaults to drafted
            subscriptions: The subscriptions.
            visible_to_clients: Whether the document is visible to the project's clients. Applies
                only when creating directly in the tool's vault — an item created inside a folder
                inherits the folder's visibility and ignores this. A client caller always creates
                client-visible records regardless of what is sent.
        """
        return self._request(
            OperationInfo(
                service="googledocuments",
                operation="create_google_document",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=vault_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/vaults/{vault_id}/google_documents.json",
            json_body=self._compact(
                url=url,
                document_type=document_type,
                title=title,
                description=description,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateGoogleDocument",
        )

    def get_google_document(self, *, google_document_id: int) -> dict[str, Any]:
        """Get a single Google document by id.

        Args:
            google_document_id: The google document id.
        """
        return self._request(
            OperationInfo(
                service="googledocuments",
                operation="get_google_document",
                is_mutation=False,
                resource_id=google_document_id,
            ),
            "GET",
            f"/google_documents/{google_document_id}",
            operation="GetGoogleDocument",
        )

    def update_google_document(
        self,
        *,
        google_document_id: int,
        url: str,
        document_type: str,
        title: str | None = None,
        description: str | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Replace a Google document with a new complete representation.
        BC3 builds a brand-new GoogleDocument from the permitted params and swaps
        the recordable wholesale, so the request body is the full writable state:
        omitting `title` clears it (and the document then reads back as "Untitled"),
        and omitting `description` clears it. `url` and `document_type` are required
        here — `document_type` because an absent or unrecognized value is a 422, and
        `url` because it carries a presence validation.
        Subscribers are the exception to omission-clears: a drafted document keeps
        its current subscribers when the request addresses neither `subscriptions`
        nor `notify`. The creator is always on the list.
        The legacy bucket-scoped path
        PUT /buckets/{bucketId}/google_documents/{id}.json is also accepted by BC3;
        this flat spelling is the documented one.

        Args:
            google_document_id: The google document id.
            url: The url.
            document_type: One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
                unrecognized value is rejected up front with a field-keyed 422 ({"errors":
                {"document_type": ["is not a valid document type"]}}) rather than reaching
                validation.
            title: The title.
            description: The description.
            status: active|drafted
            subscriptions: The subscriptions.
        """
        return self._request(
            OperationInfo(
                service="googledocuments",
                operation="update_google_document",
                is_mutation=True,
                resource_id=google_document_id,
            ),
            "PUT",
            f"/google_documents/{google_document_id}",
            json_body=self._compact(
                url=url,
                document_type=document_type,
                title=title,
                description=description,
                status=status,
                subscriptions=subscriptions,
            ),
            operation="UpdateGoogleDocument",
        )


class AsyncGoogleDocumentsService(AsyncBaseService):
    async def create_google_document(
        self,
        *,
        bucket_id: int,
        vault_id: int,
        url: str,
        document_type: str,
        title: str | None = None,
        description: str | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new Google document in a vault.
        An unrecognized `document_type` is rejected before validation with the
        field-keyed 422 {"errors": {"document_type": ["is not a valid document
        type"]}} — the enum would otherwise raise rather than add an error.
        Omitting `title` is allowed and reads back as "Untitled".

        Args:
            bucket_id: The bucket id.
            vault_id: The vault id.
            url: The url.
            document_type: One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
                unrecognized value is rejected up front with a field-keyed 422 ({"errors":
                {"document_type": ["is not a valid document type"]}}) rather than reaching
                validation.
            title: The title.
            description: The description.
            status: active|drafted — defaults to drafted
            subscriptions: The subscriptions.
            visible_to_clients: Whether the document is visible to the project's clients. Applies
                only when creating directly in the tool's vault — an item created inside a folder
                inherits the folder's visibility and ignores this. A client caller always creates
                client-visible records regardless of what is sent.
        """
        return await self._request(
            OperationInfo(
                service="googledocuments",
                operation="create_google_document",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=vault_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/vaults/{vault_id}/google_documents.json",
            json_body=self._compact(
                url=url,
                document_type=document_type,
                title=title,
                description=description,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateGoogleDocument",
        )

    async def get_google_document(self, *, google_document_id: int) -> dict[str, Any]:
        """Get a single Google document by id.

        Args:
            google_document_id: The google document id.
        """
        return await self._request(
            OperationInfo(
                service="googledocuments",
                operation="get_google_document",
                is_mutation=False,
                resource_id=google_document_id,
            ),
            "GET",
            f"/google_documents/{google_document_id}",
            operation="GetGoogleDocument",
        )

    async def update_google_document(
        self,
        *,
        google_document_id: int,
        url: str,
        document_type: str,
        title: str | None = None,
        description: str | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Replace a Google document with a new complete representation.
        BC3 builds a brand-new GoogleDocument from the permitted params and swaps
        the recordable wholesale, so the request body is the full writable state:
        omitting `title` clears it (and the document then reads back as "Untitled"),
        and omitting `description` clears it. `url` and `document_type` are required
        here — `document_type` because an absent or unrecognized value is a 422, and
        `url` because it carries a presence validation.
        Subscribers are the exception to omission-clears: a drafted document keeps
        its current subscribers when the request addresses neither `subscriptions`
        nor `notify`. The creator is always on the list.
        The legacy bucket-scoped path
        PUT /buckets/{bucketId}/google_documents/{id}.json is also accepted by BC3;
        this flat spelling is the documented one.

        Args:
            google_document_id: The google document id.
            url: The url.
            document_type: One of "doc", "sheet", "slide", "other". Backed by a Rails enum, so an
                unrecognized value is rejected up front with a field-keyed 422 ({"errors":
                {"document_type": ["is not a valid document type"]}}) rather than reaching
                validation.
            title: The title.
            description: The description.
            status: active|drafted
            subscriptions: The subscriptions.
        """
        return await self._request(
            OperationInfo(
                service="googledocuments",
                operation="update_google_document",
                is_mutation=True,
                resource_id=google_document_id,
            ),
            "PUT",
            f"/google_documents/{google_document_id}",
            json_body=self._compact(
                url=url,
                document_type=document_type,
                title=title,
                description=description,
                status=status,
                subscriptions=subscriptions,
            ),
            operation="UpdateGoogleDocument",
        )
