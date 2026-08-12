# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MessageTypesService(BaseService):
    def list(self, *, bucket_id: int, max_items: int | None = None) -> ListResult:
        """List message types in a project.

        Args:
            bucket_id: The bucket id.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages.
        """
        return self._request_paginated(
            OperationInfo(service="messagetypes", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/categories.json",
            max_items=max_items,
            operation="ListMessageTypes",
        )

    def create(self, *, bucket_id: int, name: str, icon: str) -> dict[str, Any]:
        """Create a new message type in a project.

        Args:
            bucket_id: The bucket id.
            name: The name.
            icon: The icon.
        """
        return self._request(
            OperationInfo(service="messagetypes", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/categories.json",
            json_body=self._compact(name=name, icon=icon),
            operation="CreateMessageType",
        )

    def get(self, *, bucket_id: int, type_id: int) -> dict[str, Any]:
        """Get a single message type by id.

        Args:
            bucket_id: The bucket id.
            type_id: The type id.
        """
        return self._request(
            OperationInfo(
                service="messagetypes", operation="get", is_mutation=False, project_id=bucket_id, resource_id=type_id
            ),
            "GET",
            f"/buckets/{bucket_id}/categories/{type_id}",
            operation="GetMessageType",
        )

    def update(
        self, *, bucket_id: int, type_id: int, name: str | None = None, icon: str | None = None
    ) -> dict[str, Any]:
        """Update an existing message type.

        Args:
            bucket_id: The bucket id.
            type_id: The type id.
            name: The name.
            icon: The icon.
        """
        return self._request(
            OperationInfo(
                service="messagetypes", operation="update", is_mutation=True, project_id=bucket_id, resource_id=type_id
            ),
            "PUT",
            f"/buckets/{bucket_id}/categories/{type_id}",
            json_body=self._compact(name=name, icon=icon),
            operation="UpdateMessageType",
        )

    def delete(self, *, bucket_id: int, type_id: int) -> None:
        """Delete a message type.

        Args:
            bucket_id: The bucket id.
            type_id: The type id.
        """
        self._request_void(
            OperationInfo(
                service="messagetypes", operation="delete", is_mutation=True, project_id=bucket_id, resource_id=type_id
            ),
            "DELETE",
            f"/buckets/{bucket_id}/categories/{type_id}",
            operation="DeleteMessageType",
        )


class AsyncMessageTypesService(AsyncBaseService):
    async def list(self, *, bucket_id: int, max_items: int | None = None) -> ListResult:
        """List message types in a project.

        Args:
            bucket_id: The bucket id.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages.
        """
        return await self._request_paginated(
            OperationInfo(service="messagetypes", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/categories.json",
            max_items=max_items,
            operation="ListMessageTypes",
        )

    async def create(self, *, bucket_id: int, name: str, icon: str) -> dict[str, Any]:
        """Create a new message type in a project.

        Args:
            bucket_id: The bucket id.
            name: The name.
            icon: The icon.
        """
        return await self._request(
            OperationInfo(service="messagetypes", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/categories.json",
            json_body=self._compact(name=name, icon=icon),
            operation="CreateMessageType",
        )

    async def get(self, *, bucket_id: int, type_id: int) -> dict[str, Any]:
        """Get a single message type by id.

        Args:
            bucket_id: The bucket id.
            type_id: The type id.
        """
        return await self._request(
            OperationInfo(
                service="messagetypes", operation="get", is_mutation=False, project_id=bucket_id, resource_id=type_id
            ),
            "GET",
            f"/buckets/{bucket_id}/categories/{type_id}",
            operation="GetMessageType",
        )

    async def update(
        self, *, bucket_id: int, type_id: int, name: str | None = None, icon: str | None = None
    ) -> dict[str, Any]:
        """Update an existing message type.

        Args:
            bucket_id: The bucket id.
            type_id: The type id.
            name: The name.
            icon: The icon.
        """
        return await self._request(
            OperationInfo(
                service="messagetypes", operation="update", is_mutation=True, project_id=bucket_id, resource_id=type_id
            ),
            "PUT",
            f"/buckets/{bucket_id}/categories/{type_id}",
            json_body=self._compact(name=name, icon=icon),
            operation="UpdateMessageType",
        )

    async def delete(self, *, bucket_id: int, type_id: int) -> None:
        """Delete a message type.

        Args:
            bucket_id: The bucket id.
            type_id: The type id.
        """
        await self._request_void(
            OperationInfo(
                service="messagetypes", operation="delete", is_mutation=True, project_id=bucket_id, resource_id=type_id
            ),
            "DELETE",
            f"/buckets/{bucket_id}/categories/{type_id}",
            operation="DeleteMessageType",
        )
