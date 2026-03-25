# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class DocumentsService(BaseService):
    def get(self, *, document_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="documents", operation="get", is_mutation=False, resource_id=document_id),
            "GET",
            f"/documents/{document_id}",
        )

    def update(self, *, document_id: int, title: str | None = None, content: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="documents", operation="update", is_mutation=True, resource_id=document_id),
            "PUT",
            f"/documents/{document_id}",
            json_body=self._compact(title=title, content=content),
            operation="UpdateDocument",
        )

    def list(self, *, vault_id: int) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="documents", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/documents.json",
        )

    def create(
        self,
        *,
        vault_id: int,
        title: str,
        content: str | None = None,
        status: str | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="documents", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/documents.json",
            json_body=self._compact(title=title, content=content, status=status, subscriptions=subscriptions),
            operation="CreateDocument",
        )


class AsyncDocumentsService(AsyncBaseService):
    async def get(self, *, document_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="documents", operation="get", is_mutation=False, resource_id=document_id),
            "GET",
            f"/documents/{document_id}",
        )

    async def update(self, *, document_id: int, title: str | None = None, content: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="documents", operation="update", is_mutation=True, resource_id=document_id),
            "PUT",
            f"/documents/{document_id}",
            json_body=self._compact(title=title, content=content),
            operation="UpdateDocument",
        )

    async def list(self, *, vault_id: int) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="documents", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/documents.json",
        )

    async def create(
        self,
        *,
        vault_id: int,
        title: str,
        content: str | None = None,
        status: str | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="documents", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/documents.json",
            json_body=self._compact(title=title, content=content, status=status, subscriptions=subscriptions),
            operation="CreateDocument",
        )
