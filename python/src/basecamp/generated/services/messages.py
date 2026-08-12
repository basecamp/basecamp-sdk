# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MessagesService(BaseService):
    def list(
        self,
        *,
        board_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List messages on a message board.

        Args:
            board_id: The board id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="messages", operation="list", is_mutation=False, resource_id=board_id),
            f"/message_boards/{board_id}/messages.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListMessages",
        )

    def create(
        self,
        *,
        board_id: int,
        subject: str,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new message on a message board.

        Args:
            board_id: The board id.
            subject: The subject.
            content: The content.
            status: active|drafted
            category_id: The category id.
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return self._request(
            OperationInfo(service="messages", operation="create", is_mutation=True, resource_id=board_id),
            "POST",
            f"/message_boards/{board_id}/messages.json",
            json_body=self._compact(
                subject=subject,
                content=content,
                status=status,
                category_id=category_id,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateMessage",
        )

    def get(self, *, message_id: int) -> dict[str, Any]:
        """Get a single message by id.

        Args:
            message_id: The message id.
        """
        return self._request(
            OperationInfo(service="messages", operation="get", is_mutation=False, resource_id=message_id),
            "GET",
            f"/messages/{message_id}",
            operation="GetMessage",
        )

    def update(
        self,
        *,
        message_id: int,
        subject: str | None = None,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
    ) -> dict[str, Any]:
        """Update an existing message.

        Args:
            message_id: The message id.
            subject: The subject.
            content: The content.
            status: active|drafted
            category_id: The category id.
        """
        return self._request(
            OperationInfo(service="messages", operation="update", is_mutation=True, resource_id=message_id),
            "PUT",
            f"/messages/{message_id}",
            json_body=self._compact(subject=subject, content=content, status=status, category_id=category_id),
            operation="UpdateMessage",
        )

    def pin(self, *, message_id: int) -> None:
        """Pin a message to the top of the message board.

        Args:
            message_id: The message id.
        """
        self._request_void(
            OperationInfo(service="messages", operation="pin", is_mutation=True, resource_id=message_id),
            "POST",
            f"/recordings/{message_id}/pin.json",
            operation="PinMessage",
        )

    def unpin(self, *, message_id: int) -> None:
        """Unpin a message from the message board.

        Args:
            message_id: The message id.
        """
        self._request_void(
            OperationInfo(service="messages", operation="unpin", is_mutation=True, resource_id=message_id),
            "DELETE",
            f"/recordings/{message_id}/pin.json",
            operation="UnpinMessage",
        )


class AsyncMessagesService(AsyncBaseService):
    async def list(
        self,
        *,
        board_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List messages on a message board.

        Args:
            board_id: The board id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="messages", operation="list", is_mutation=False, resource_id=board_id),
            f"/message_boards/{board_id}/messages.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListMessages",
        )

    async def create(
        self,
        *,
        board_id: int,
        subject: str,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new message on a message board.

        Args:
            board_id: The board id.
            subject: The subject.
            content: The content.
            status: active|drafted
            category_id: The category id.
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return await self._request(
            OperationInfo(service="messages", operation="create", is_mutation=True, resource_id=board_id),
            "POST",
            f"/message_boards/{board_id}/messages.json",
            json_body=self._compact(
                subject=subject,
                content=content,
                status=status,
                category_id=category_id,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateMessage",
        )

    async def get(self, *, message_id: int) -> dict[str, Any]:
        """Get a single message by id.

        Args:
            message_id: The message id.
        """
        return await self._request(
            OperationInfo(service="messages", operation="get", is_mutation=False, resource_id=message_id),
            "GET",
            f"/messages/{message_id}",
            operation="GetMessage",
        )

    async def update(
        self,
        *,
        message_id: int,
        subject: str | None = None,
        content: str | None = None,
        status: str | None = None,
        category_id: int | None = None,
    ) -> dict[str, Any]:
        """Update an existing message.

        Args:
            message_id: The message id.
            subject: The subject.
            content: The content.
            status: active|drafted
            category_id: The category id.
        """
        return await self._request(
            OperationInfo(service="messages", operation="update", is_mutation=True, resource_id=message_id),
            "PUT",
            f"/messages/{message_id}",
            json_body=self._compact(subject=subject, content=content, status=status, category_id=category_id),
            operation="UpdateMessage",
        )

    async def pin(self, *, message_id: int) -> None:
        """Pin a message to the top of the message board.

        Args:
            message_id: The message id.
        """
        await self._request_void(
            OperationInfo(service="messages", operation="pin", is_mutation=True, resource_id=message_id),
            "POST",
            f"/recordings/{message_id}/pin.json",
            operation="PinMessage",
        )

    async def unpin(self, *, message_id: int) -> None:
        """Unpin a message from the message board.

        Args:
            message_id: The message id.
        """
        await self._request_void(
            OperationInfo(service="messages", operation="unpin", is_mutation=True, resource_id=message_id),
            "DELETE",
            f"/recordings/{message_id}/pin.json",
            operation="UnpinMessage",
        )
