# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class UploadsService(BaseService):
    def get(self, *, upload_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="uploads", operation="get", is_mutation=False, resource_id=upload_id),
            "GET",
            f"/uploads/{upload_id}",
        )

    def update(self, *, upload_id: int, description: str | None = None, base_name: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="uploads", operation="update", is_mutation=True, resource_id=upload_id),
            "PUT",
            f"/uploads/{upload_id}",
            json_body=self._compact(description=description, base_name=base_name),
            operation="UpdateUpload",
        )

    def list_versions(self, *, upload_id: int) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="uploads", operation="list_versions", is_mutation=False, resource_id=upload_id),
            f"/uploads/{upload_id}/versions.json",
        )

    def list(self, *, vault_id: int) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="uploads", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/uploads.json",
        )

    def create(
        self,
        *,
        vault_id: int,
        attachable_sgid: str,
        description: str | None = None,
        base_name: str | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="uploads", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/uploads.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                description=description,
                base_name=base_name,
                subscriptions=subscriptions,
            ),
            operation="CreateUpload",
        )


class AsyncUploadsService(AsyncBaseService):
    async def get(self, *, upload_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="uploads", operation="get", is_mutation=False, resource_id=upload_id),
            "GET",
            f"/uploads/{upload_id}",
        )

    async def update(
        self, *, upload_id: int, description: str | None = None, base_name: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="uploads", operation="update", is_mutation=True, resource_id=upload_id),
            "PUT",
            f"/uploads/{upload_id}",
            json_body=self._compact(description=description, base_name=base_name),
            operation="UpdateUpload",
        )

    async def list_versions(self, *, upload_id: int) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="uploads", operation="list_versions", is_mutation=False, resource_id=upload_id),
            f"/uploads/{upload_id}/versions.json",
        )

    async def list(self, *, vault_id: int) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="uploads", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/uploads.json",
        )

    async def create(
        self,
        *,
        vault_id: int,
        attachable_sgid: str,
        description: str | None = None,
        base_name: str | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="uploads", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/uploads.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                description=description,
                base_name=base_name,
                subscriptions=subscriptions,
            ),
            operation="CreateUpload",
        )
