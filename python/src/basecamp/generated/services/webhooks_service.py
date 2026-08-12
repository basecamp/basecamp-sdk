# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class WebhooksService(BaseService):
    def list(self, *, bucket_id: int, max_items: int | None = None) -> ListResult:
        """List all webhooks for a project.

        Args:
            bucket_id: The bucket id.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="webhooks", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/webhooks.json",
            max_items=max_items,
            operation="ListWebhooks",
        )

    def create(
        self, *, bucket_id: int, payload_url: str, types: list[str], active: bool | None = None
    ) -> dict[str, Any]:
        """Create a new webhook for a project.

        Args:
            bucket_id: The bucket id.
            payload_url: The payload url.
            types: The types.
            active: The active.
        """
        return self._request(
            OperationInfo(service="webhooks", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/webhooks.json",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="CreateWebhook",
        )

    def get(self, *, webhook_id: int) -> dict[str, Any]:
        """Get a single webhook by id.

        Args:
            webhook_id: The webhook id.
        """
        return self._request(
            OperationInfo(service="webhooks", operation="get", is_mutation=False, resource_id=webhook_id),
            "GET",
            f"/webhooks/{webhook_id}",
            operation="GetWebhook",
        )

    def update(
        self,
        *,
        webhook_id: int,
        payload_url: str | None = None,
        types: list[str] | None = None,
        active: bool | None = None,
    ) -> dict[str, Any]:
        """Update an existing webhook.

        Args:
            webhook_id: The webhook id.
            payload_url: The payload url.
            types: The types.
            active: The active.
        """
        return self._request(
            OperationInfo(service="webhooks", operation="update", is_mutation=True, resource_id=webhook_id),
            "PUT",
            f"/webhooks/{webhook_id}",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="UpdateWebhook",
        )

    def delete(self, *, webhook_id: int) -> None:
        """Delete a webhook.

        Args:
            webhook_id: The webhook id.
        """
        self._request_void(
            OperationInfo(service="webhooks", operation="delete", is_mutation=True, resource_id=webhook_id),
            "DELETE",
            f"/webhooks/{webhook_id}",
            operation="DeleteWebhook",
        )


class AsyncWebhooksService(AsyncBaseService):
    async def list(self, *, bucket_id: int, max_items: int | None = None) -> ListResult:
        """List all webhooks for a project.

        Args:
            bucket_id: The bucket id.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="webhooks", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/webhooks.json",
            max_items=max_items,
            operation="ListWebhooks",
        )

    async def create(
        self, *, bucket_id: int, payload_url: str, types: list[str], active: bool | None = None
    ) -> dict[str, Any]:
        """Create a new webhook for a project.

        Args:
            bucket_id: The bucket id.
            payload_url: The payload url.
            types: The types.
            active: The active.
        """
        return await self._request(
            OperationInfo(service="webhooks", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/webhooks.json",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="CreateWebhook",
        )

    async def get(self, *, webhook_id: int) -> dict[str, Any]:
        """Get a single webhook by id.

        Args:
            webhook_id: The webhook id.
        """
        return await self._request(
            OperationInfo(service="webhooks", operation="get", is_mutation=False, resource_id=webhook_id),
            "GET",
            f"/webhooks/{webhook_id}",
            operation="GetWebhook",
        )

    async def update(
        self,
        *,
        webhook_id: int,
        payload_url: str | None = None,
        types: list[str] | None = None,
        active: bool | None = None,
    ) -> dict[str, Any]:
        """Update an existing webhook.

        Args:
            webhook_id: The webhook id.
            payload_url: The payload url.
            types: The types.
            active: The active.
        """
        return await self._request(
            OperationInfo(service="webhooks", operation="update", is_mutation=True, resource_id=webhook_id),
            "PUT",
            f"/webhooks/{webhook_id}",
            json_body=self._compact(payload_url=payload_url, types=types, active=active),
            operation="UpdateWebhook",
        )

    async def delete(self, *, webhook_id: int) -> None:
        """Delete a webhook.

        Args:
            webhook_id: The webhook id.
        """
        await self._request_void(
            OperationInfo(service="webhooks", operation="delete", is_mutation=True, resource_id=webhook_id),
            "DELETE",
            f"/webhooks/{webhook_id}",
            operation="DeleteWebhook",
        )
