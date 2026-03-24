# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ForwardsService(BaseService):
    def get(self, *, forward_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="forwards", operation="get", is_mutation=False, resource_id=forward_id),
            "GET",
            f"/inbox_forwards/{forward_id}",
        )

    def list_replies(self, *, forward_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="forwards", operation="list_replies", is_mutation=False, resource_id=forward_id),
            f"/inbox_forwards/{forward_id}/replies.json",
        )

    def create_reply(self, *, forward_id: int | str, content: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="forwards", operation="create_reply", is_mutation=True, resource_id=forward_id),
            "POST",
            f"/inbox_forwards/{forward_id}/replies.json",
            json_body=self._compact(content=content),
            operation="CreateForwardReply",
        )

    def get_reply(self, *, forward_id: int | str, reply_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="forwards", operation="get_reply", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/inbox_forwards/{forward_id}/replies/{reply_id}",
        )

    def get_inbox(self, *, inbox_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="forwards", operation="get_inbox", is_mutation=False, resource_id=inbox_id),
            "GET",
            f"/inboxes/{inbox_id}",
        )

    def list(self, *, inbox_id: int | str, sort: str | None = None, direction: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="forwards", operation="list", is_mutation=False, resource_id=inbox_id),
            f"/inboxes/{inbox_id}/forwards.json",
            params=self._compact(sort=sort, direction=direction),
        )


class AsyncForwardsService(AsyncBaseService):
    async def get(self, *, forward_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="forwards", operation="get", is_mutation=False, resource_id=forward_id),
            "GET",
            f"/inbox_forwards/{forward_id}",
        )

    async def list_replies(self, *, forward_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="forwards", operation="list_replies", is_mutation=False, resource_id=forward_id),
            f"/inbox_forwards/{forward_id}/replies.json",
        )

    async def create_reply(self, *, forward_id: int | str, content: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="forwards", operation="create_reply", is_mutation=True, resource_id=forward_id),
            "POST",
            f"/inbox_forwards/{forward_id}/replies.json",
            json_body=self._compact(content=content),
            operation="CreateForwardReply",
        )

    async def get_reply(self, *, forward_id: int | str, reply_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="forwards", operation="get_reply", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/inbox_forwards/{forward_id}/replies/{reply_id}",
        )

    async def get_inbox(self, *, inbox_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="forwards", operation="get_inbox", is_mutation=False, resource_id=inbox_id),
            "GET",
            f"/inboxes/{inbox_id}",
        )

    async def list(self, *, inbox_id: int | str, sort: str | None = None, direction: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="forwards", operation="list", is_mutation=False, resource_id=inbox_id),
            f"/inboxes/{inbox_id}/forwards.json",
            params=self._compact(sort=sort, direction=direction),
        )
