# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MessageTypesService(BaseService):
    def list(self, *, project_id: int) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="messagetypes", operation="list", is_mutation=False, project_id=project_id),
            f"/buckets/{project_id}/categories.json",
        )

    def create(self, *, project_id: int, name: str, icon: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="messagetypes", operation="create", is_mutation=True, project_id=project_id),
            "POST",
            f"/buckets/{project_id}/categories.json",
            json_body=self._compact(name=name, icon=icon),
            operation="CreateMessageType",
        )

    def get(self, *, project_id: int, type_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="messagetypes", operation="get", is_mutation=False, project_id=project_id, resource_id=type_id
            ),
            "GET",
            f"/buckets/{project_id}/categories/{type_id}",
        )

    def update(
        self, *, project_id: int, type_id: int, name: str | None = None, icon: str | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="messagetypes", operation="update", is_mutation=True, project_id=project_id, resource_id=type_id
            ),
            "PUT",
            f"/buckets/{project_id}/categories/{type_id}",
            json_body=self._compact(name=name, icon=icon),
            operation="UpdateMessageType",
        )

    def delete(self, *, project_id: int, type_id: int) -> None:
        self._request_void(
            OperationInfo(
                service="messagetypes", operation="delete", is_mutation=True, project_id=project_id, resource_id=type_id
            ),
            "DELETE",
            f"/buckets/{project_id}/categories/{type_id}",
            operation="DeleteMessageType",
        )


class AsyncMessageTypesService(AsyncBaseService):
    async def list(self, *, project_id: int) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="messagetypes", operation="list", is_mutation=False, project_id=project_id),
            f"/buckets/{project_id}/categories.json",
        )

    async def create(self, *, project_id: int, name: str, icon: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="messagetypes", operation="create", is_mutation=True, project_id=project_id),
            "POST",
            f"/buckets/{project_id}/categories.json",
            json_body=self._compact(name=name, icon=icon),
            operation="CreateMessageType",
        )

    async def get(self, *, project_id: int, type_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="messagetypes", operation="get", is_mutation=False, project_id=project_id, resource_id=type_id
            ),
            "GET",
            f"/buckets/{project_id}/categories/{type_id}",
        )

    async def update(
        self, *, project_id: int, type_id: int, name: str | None = None, icon: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="messagetypes", operation="update", is_mutation=True, project_id=project_id, resource_id=type_id
            ),
            "PUT",
            f"/buckets/{project_id}/categories/{type_id}",
            json_body=self._compact(name=name, icon=icon),
            operation="UpdateMessageType",
        )

    async def delete(self, *, project_id: int, type_id: int) -> None:
        await self._request_void(
            OperationInfo(
                service="messagetypes", operation="delete", is_mutation=True, project_id=project_id, resource_id=type_id
            ),
            "DELETE",
            f"/buckets/{project_id}/categories/{type_id}",
            operation="DeleteMessageType",
        )
