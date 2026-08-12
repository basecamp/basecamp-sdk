# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ForwardsService(BaseService):
    def get(self, *, forward_id: int) -> dict[str, Any]:
        """Get a forward by ID.

        Args:
            forward_id: The forward id.
        """
        return self._request(
            OperationInfo(service="forwards", operation="get", is_mutation=False, resource_id=forward_id),
            "GET",
            f"/inbox_forwards/{forward_id}",
            operation="GetForward",
        )

    def list_replies(self, *, forward_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all replies to a forward.

        Args:
            forward_id: The forward id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="forwards", operation="list_replies", is_mutation=False, resource_id=forward_id),
            f"/inbox_forwards/{forward_id}/replies.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListForwardReplies",
        )

    def get_reply(self, *, forward_id: int, reply_id: int) -> dict[str, Any]:
        """Get a forward reply by ID.

        Args:
            forward_id: The forward id.
            reply_id: The reply id.
        """
        return self._request(
            OperationInfo(service="forwards", operation="get_reply", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/inbox_forwards/{forward_id}/replies/{reply_id}",
            operation="GetForwardReply",
        )

    def get_inbox(self, *, inbox_id: int) -> dict[str, Any]:
        """Get an inbox by ID.

        Args:
            inbox_id: The inbox id.
        """
        return self._request(
            OperationInfo(service="forwards", operation="get_inbox", is_mutation=False, resource_id=inbox_id),
            "GET",
            f"/inboxes/{inbox_id}",
            operation="GetInbox",
        )

    def list(
        self,
        *,
        inbox_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List all forwards in an inbox.

        Args:
            inbox_id: The inbox id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="forwards", operation="list", is_mutation=False, resource_id=inbox_id),
            f"/inboxes/{inbox_id}/inbox_forwards.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListForwards",
        )


class AsyncForwardsService(AsyncBaseService):
    async def get(self, *, forward_id: int) -> dict[str, Any]:
        """Get a forward by ID.

        Args:
            forward_id: The forward id.
        """
        return await self._request(
            OperationInfo(service="forwards", operation="get", is_mutation=False, resource_id=forward_id),
            "GET",
            f"/inbox_forwards/{forward_id}",
            operation="GetForward",
        )

    async def list_replies(
        self, *, forward_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List all replies to a forward.

        Args:
            forward_id: The forward id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="forwards", operation="list_replies", is_mutation=False, resource_id=forward_id),
            f"/inbox_forwards/{forward_id}/replies.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListForwardReplies",
        )

    async def get_reply(self, *, forward_id: int, reply_id: int) -> dict[str, Any]:
        """Get a forward reply by ID.

        Args:
            forward_id: The forward id.
            reply_id: The reply id.
        """
        return await self._request(
            OperationInfo(service="forwards", operation="get_reply", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/inbox_forwards/{forward_id}/replies/{reply_id}",
            operation="GetForwardReply",
        )

    async def get_inbox(self, *, inbox_id: int) -> dict[str, Any]:
        """Get an inbox by ID.

        Args:
            inbox_id: The inbox id.
        """
        return await self._request(
            OperationInfo(service="forwards", operation="get_inbox", is_mutation=False, resource_id=inbox_id),
            "GET",
            f"/inboxes/{inbox_id}",
            operation="GetInbox",
        )

    async def list(
        self,
        *,
        inbox_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List all forwards in an inbox.

        Args:
            inbox_id: The inbox id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="forwards", operation="list", is_mutation=False, resource_id=inbox_id),
            f"/inboxes/{inbox_id}/inbox_forwards.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListForwards",
        )
