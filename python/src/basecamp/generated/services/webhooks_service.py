# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class WebhooksService(BaseService):
    def list(self, *, bucket_id: int) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="webhooks", operation="list", is_mutation=False, resource_id=bucket_id),
            f"/buckets/{bucket_id}/webhooks.json",
        )

    def create(self, *, bucket_id: int, payload_url: str, types: list, active: bool | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="webhooks", operation="create", is_mutation=True, resource_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/webhooks.json",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="CreateWebhook",
        )

    def get(self, *, webhook_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="webhooks", operation="get", is_mutation=False, resource_id=webhook_id),
            "GET",
            f"/webhooks/{webhook_id}",
        )

    def update(
        self, *, webhook_id: int, payload_url: str | None = None, types: list | None = None, active: bool | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="webhooks", operation="update", is_mutation=True, resource_id=webhook_id),
            "PUT",
            f"/webhooks/{webhook_id}",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="UpdateWebhook",
        )

    def delete(self, *, webhook_id: int) -> None:
        self._request_void(
            OperationInfo(service="webhooks", operation="delete", is_mutation=True, resource_id=webhook_id),
            "DELETE",
            f"/webhooks/{webhook_id}",
            operation="DeleteWebhook",
        )


class AsyncWebhooksService(AsyncBaseService):
    async def list(self, *, bucket_id: int) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="webhooks", operation="list", is_mutation=False, resource_id=bucket_id),
            f"/buckets/{bucket_id}/webhooks.json",
        )

    async def create(
        self, *, bucket_id: int, payload_url: str, types: list, active: bool | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="webhooks", operation="create", is_mutation=True, resource_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/webhooks.json",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="CreateWebhook",
        )

    async def get(self, *, webhook_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="webhooks", operation="get", is_mutation=False, resource_id=webhook_id),
            "GET",
            f"/webhooks/{webhook_id}",
        )

    async def update(
        self, *, webhook_id: int, payload_url: str | None = None, types: list | None = None, active: bool | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="webhooks", operation="update", is_mutation=True, resource_id=webhook_id),
            "PUT",
            f"/webhooks/{webhook_id}",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="UpdateWebhook",
        )

    async def delete(self, *, webhook_id: int) -> None:
        await self._request_void(
            OperationInfo(service="webhooks", operation="delete", is_mutation=True, resource_id=webhook_id),
            "DELETE",
            f"/webhooks/{webhook_id}",
            operation="DeleteWebhook",
        )
