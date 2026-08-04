# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CloudFilesService(BaseService):
    def create_cloud_file(
        self,
        *,
        bucket_id: int,
        vault_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="cloudfiles",
                operation="create_cloud_file",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=vault_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/vaults/{vault_id}/cloud_files.json",
            json_body=self._compact(
                url=url,
                service=service,
                title=title,
                description=description,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateCloudFile",
        )

    def get_cloud_file(self, *, cloud_file_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="cloudfiles", operation="get_cloud_file", is_mutation=False, resource_id=cloud_file_id
            ),
            "GET",
            f"/cloud_files/{cloud_file_id}",
            operation="GetCloudFile",
        )

    def update_cloud_file(
        self,
        *,
        cloud_file_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="cloudfiles", operation="update_cloud_file", is_mutation=True, resource_id=cloud_file_id
            ),
            "PUT",
            f"/cloud_files/{cloud_file_id}",
            json_body=self._compact(
                url=url, service=service, title=title, description=description, subscriptions=subscriptions
            ),
            operation="UpdateCloudFile",
        )


class AsyncCloudFilesService(AsyncBaseService):
    async def create_cloud_file(
        self,
        *,
        bucket_id: int,
        vault_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="cloudfiles",
                operation="create_cloud_file",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=vault_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/vaults/{vault_id}/cloud_files.json",
            json_body=self._compact(
                url=url,
                service=service,
                title=title,
                description=description,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateCloudFile",
        )

    async def get_cloud_file(self, *, cloud_file_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="cloudfiles", operation="get_cloud_file", is_mutation=False, resource_id=cloud_file_id
            ),
            "GET",
            f"/cloud_files/{cloud_file_id}",
            operation="GetCloudFile",
        )

    async def update_cloud_file(
        self,
        *,
        cloud_file_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="cloudfiles", operation="update_cloud_file", is_mutation=True, resource_id=cloud_file_id
            ),
            "PUT",
            f"/cloud_files/{cloud_file_id}",
            json_body=self._compact(
                url=url, service=service, title=title, description=description, subscriptions=subscriptions
            ),
            operation="UpdateCloudFile",
        )
