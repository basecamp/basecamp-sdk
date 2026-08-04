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
