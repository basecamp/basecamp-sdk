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
            operation="GetUpload",
        )

    def update(self, *, upload_id: int, description: str | None = None, base_name: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="uploads", operation="update", is_mutation=True, resource_id=upload_id),
            "PUT",
            f"/uploads/{upload_id}",
            json_body=self._compact(description=description, base_name=base_name),
            operation="UpdateUpload",
        )

    def list_versions(self, *, upload_id: int, max_items: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="uploads", operation="list_versions", is_mutation=False, resource_id=upload_id),
            f"/uploads/{upload_id}/versions.json",
            max_items=max_items,
            operation="ListUploadVersions",
        )

    def create_version(
        self,
        *,
        upload_id: int,
        attachable_sgid: str,
        base_name: str | None = None,
        description: str | None = None,
        notify: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="uploads", operation="create_version", is_mutation=True, resource_id=upload_id),
            "POST",
            f"/uploads/{upload_id}/versions.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                base_name=base_name,
                description=description,
                notify=notify,
                subscriptions=subscriptions,
            ),
            operation="CreateUploadVersion",
        )

    def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="uploads", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/uploads.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListUploads",
        )

    def create(
        self,
        *,
        vault_id: int,
        attachable_sgid: str,
        description: str | None = None,
        base_name: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
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
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateUpload",
        )


class AsyncUploadsService(AsyncBaseService):
    async def get(self, *, upload_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="uploads", operation="get", is_mutation=False, resource_id=upload_id),
            "GET",
            f"/uploads/{upload_id}",
            operation="GetUpload",
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

    async def list_versions(self, *, upload_id: int, max_items: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="uploads", operation="list_versions", is_mutation=False, resource_id=upload_id),
            f"/uploads/{upload_id}/versions.json",
            max_items=max_items,
            operation="ListUploadVersions",
        )

    async def create_version(
        self,
        *,
        upload_id: int,
        attachable_sgid: str,
        base_name: str | None = None,
        description: str | None = None,
        notify: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="uploads", operation="create_version", is_mutation=True, resource_id=upload_id),
            "POST",
            f"/uploads/{upload_id}/versions.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                base_name=base_name,
                description=description,
                notify=notify,
                subscriptions=subscriptions,
            ),
            operation="CreateUploadVersion",
        )

    async def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="uploads", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/uploads.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListUploads",
        )

    async def create(
        self,
        *,
        vault_id: int,
        attachable_sgid: str,
        description: str | None = None,
        base_name: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
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
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateUpload",
        )
