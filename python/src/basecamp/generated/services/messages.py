# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MessagesService(BaseService):
    def list(self, *, board_id: int | str, sort: str | None = None, direction: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="messages", operation="list", is_mutation=False, resource_id=board_id),
            f"/message_boards/{board_id}/messages.json",
            params=self._compact(sort=sort, direction=direction),
        )

    def create(
        self,
        *,
        board_id: int | str,
        subject: str,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="messages", operation="create", is_mutation=True, resource_id=board_id),
            "POST",
            f"/message_boards/{board_id}/messages.json",
            json_body=self._compact(
                subject=subject, content=content, status=status, category_id=category_id, subscriptions=subscriptions
            ),
            operation="CreateMessage",
        )

    def get(self, *, message_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="messages", operation="get", is_mutation=False, resource_id=message_id),
            "GET",
            f"/messages/{message_id}",
        )

    def update(
        self,
        *,
        message_id: int | str,
        subject: str | None = None,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="messages", operation="update", is_mutation=True, resource_id=message_id),
            "PUT",
            f"/messages/{message_id}",
            json_body=self._compact(subject=subject, content=content, status=status, category_id=category_id),
            operation="UpdateMessage",
        )

    def pin(self, *, message_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="messages", operation="pin", is_mutation=True, resource_id=message_id),
            "POST",
            f"/recordings/{message_id}/pin.json",
            operation="PinMessage",
        )

    def unpin(self, *, message_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="messages", operation="unpin", is_mutation=True, resource_id=message_id),
            "DELETE",
            f"/recordings/{message_id}/pin.json",
            operation="UnpinMessage",
        )


class AsyncMessagesService(AsyncBaseService):
    async def list(self, *, board_id: int | str, sort: str | None = None, direction: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="messages", operation="list", is_mutation=False, resource_id=board_id),
            f"/message_boards/{board_id}/messages.json",
            params=self._compact(sort=sort, direction=direction),
        )

    async def create(
        self,
        *,
        board_id: int | str,
        subject: str,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="messages", operation="create", is_mutation=True, resource_id=board_id),
            "POST",
            f"/message_boards/{board_id}/messages.json",
            json_body=self._compact(
                subject=subject, content=content, status=status, category_id=category_id, subscriptions=subscriptions
            ),
            operation="CreateMessage",
        )

    async def get(self, *, message_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="messages", operation="get", is_mutation=False, resource_id=message_id),
            "GET",
            f"/messages/{message_id}",
        )

    async def update(
        self,
        *,
        message_id: int | str,
        subject: str | None = None,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="messages", operation="update", is_mutation=True, resource_id=message_id),
            "PUT",
            f"/messages/{message_id}",
            json_body=self._compact(subject=subject, content=content, status=status, category_id=category_id),
            operation="UpdateMessage",
        )

    async def pin(self, *, message_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="messages", operation="pin", is_mutation=True, resource_id=message_id),
            "POST",
            f"/recordings/{message_id}/pin.json",
            operation="PinMessage",
        )

    async def unpin(self, *, message_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="messages", operation="unpin", is_mutation=True, resource_id=message_id),
            "DELETE",
            f"/recordings/{message_id}/pin.json",
            operation="UnpinMessage",
        )
