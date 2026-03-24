# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MyNotificationsService(BaseService):
    def get_my_notifications(self, *, page: int | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="mynotifications", operation="get_my_notifications", is_mutation=False),
            "GET",
            "/my/readings.json",
            params=self._compact(page=page),
        )

    def mark_as_read(self, *, readables: list) -> None:
        self._request_void(
            OperationInfo(service="mynotifications", operation="mark_as_read", is_mutation=True),
            "PUT",
            "/my/unreads.json",
            json_body=self._compact(readables=readables),
            operation="MarkAsRead",
        )


class AsyncMyNotificationsService(AsyncBaseService):
    async def get_my_notifications(self, *, page: int | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="mynotifications", operation="get_my_notifications", is_mutation=False),
            "GET",
            "/my/readings.json",
            params=self._compact(page=page),
        )

    async def mark_as_read(self, *, readables: list) -> None:
        await self._request_void(
            OperationInfo(service="mynotifications", operation="mark_as_read", is_mutation=True),
            "PUT",
            "/my/unreads.json",
            json_body=self._compact(readables=readables),
            operation="MarkAsRead",
        )
