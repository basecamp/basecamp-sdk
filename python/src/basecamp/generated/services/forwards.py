# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ForwardsService(BaseService):
    def get(self, *, forward_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="forwards", operation="get", is_mutation=False, resource_id=forward_id),
            "GET",
            f"/inbox_forwards/{forward_id}",
            operation="GetForward",
        )

    def list_replies(self, *, forward_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="forwards", operation="list_replies", is_mutation=False, resource_id=forward_id),
            f"/inbox_forwards/{forward_id}/replies.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListForwardReplies",
        )

    def get_reply(self, *, forward_id: int, reply_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="forwards", operation="get_reply", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/inbox_forwards/{forward_id}/replies/{reply_id}",
            operation="GetForwardReply",
        )

    def get_inbox(self, *, inbox_id: int) -> dict[str, Any]:
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
        return self._request_paginated(
            OperationInfo(service="forwards", operation="list", is_mutation=False, resource_id=inbox_id),
            f"/inboxes/{inbox_id}/inbox_forwards.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListForwards",
        )


class AsyncForwardsService(AsyncBaseService):
    async def get(self, *, forward_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="forwards", operation="get", is_mutation=False, resource_id=forward_id),
            "GET",
            f"/inbox_forwards/{forward_id}",
            operation="GetForward",
        )

    async def list_replies(
        self, *, forward_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="forwards", operation="list_replies", is_mutation=False, resource_id=forward_id),
            f"/inbox_forwards/{forward_id}/replies.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListForwardReplies",
        )

    async def get_reply(self, *, forward_id: int, reply_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="forwards", operation="get_reply", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/inbox_forwards/{forward_id}/replies/{reply_id}",
            operation="GetForwardReply",
        )

    async def get_inbox(self, *, inbox_id: int) -> dict[str, Any]:
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
        return await self._request_paginated(
            OperationInfo(service="forwards", operation="list", is_mutation=False, resource_id=inbox_id),
            f"/inboxes/{inbox_id}/inbox_forwards.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListForwards",
        )
