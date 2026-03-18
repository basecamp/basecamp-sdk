# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ClientRepliesService(BaseService):
    def list(self, *, recording_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="clientreplies", operation="list", is_mutation=False, resource_id=recording_id),
            f"/client/recordings/{recording_id}/replies.json",
        )

    def get(self, *, recording_id: int | str, reply_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="clientreplies", operation="get", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/client/recordings/{recording_id}/replies/{reply_id}",
        )


class AsyncClientRepliesService(AsyncBaseService):
    async def list(self, *, recording_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="clientreplies", operation="list", is_mutation=False, resource_id=recording_id),
            f"/client/recordings/{recording_id}/replies.json",
        )

    async def get(self, *, recording_id: int | str, reply_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="clientreplies", operation="get", is_mutation=False, resource_id=reply_id),
            "GET",
            f"/client/recordings/{recording_id}/replies/{reply_id}",
        )
