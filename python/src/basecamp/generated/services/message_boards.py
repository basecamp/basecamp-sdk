# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MessageBoardsService(BaseService):
    def get(self, *, board_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="messageboards", operation="get", is_mutation=False, resource_id=board_id),
            "GET",
            f"/message_boards/{board_id}",
        )


class AsyncMessageBoardsService(AsyncBaseService):
    async def get(self, *, board_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="messageboards", operation="get", is_mutation=False, resource_id=board_id),
            "GET",
            f"/message_boards/{board_id}",
        )
