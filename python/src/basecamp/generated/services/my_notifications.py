# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MyNotificationsService(BaseService):
    def get_my_notifications(self, *, page: int | None = None, limit_bubble_ups: bool | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="mynotifications", operation="get_my_notifications", is_mutation=False),
            "GET",
            "/my/readings.json",
            params=self._compact(page=page, limit_bubble_ups=limit_bubble_ups),
            operation="GetMyNotifications",
        )

    def get_bubble_ups(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="mynotifications", operation="get_bubble_ups", is_mutation=False),
            "/my/readings/bubble_ups.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetBubbleUps",
        )

    def mark_as_read(self, *, readables: list[str]) -> None:
        self._request_void(
            OperationInfo(service="mynotifications", operation="mark_as_read", is_mutation=True),
            "PUT",
            "/my/unreads.json",
            json_body=self._compact(readables=readables),
            operation="MarkAsRead",
        )


class AsyncMyNotificationsService(AsyncBaseService):
    async def get_my_notifications(
        self, *, page: int | None = None, limit_bubble_ups: bool | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="mynotifications", operation="get_my_notifications", is_mutation=False),
            "GET",
            "/my/readings.json",
            params=self._compact(page=page, limit_bubble_ups=limit_bubble_ups),
            operation="GetMyNotifications",
        )

    async def get_bubble_ups(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="mynotifications", operation="get_bubble_ups", is_mutation=False),
            "/my/readings/bubble_ups.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetBubbleUps",
        )

    async def mark_as_read(self, *, readables: list[str]) -> None:
        await self._request_void(
            OperationInfo(service="mynotifications", operation="mark_as_read", is_mutation=True),
            "PUT",
            "/my/unreads.json",
            json_body=self._compact(readables=readables),
            operation="MarkAsRead",
        )
