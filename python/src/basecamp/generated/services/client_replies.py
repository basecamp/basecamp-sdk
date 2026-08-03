# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ClientRepliesService(BaseService):
    def list(
        self, *, bucket_id: int, recording_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(
                service="clientreplies",
                operation="list",
                is_mutation=False,
                project_id=bucket_id,
                resource_id=recording_id,
            ),
            f"/buckets/{bucket_id}/client/recordings/{recording_id}/replies.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListClientReplies",
        )

    def get(self, *, bucket_id: int, recording_id: int, reply_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="clientreplies", operation="get", is_mutation=False, project_id=bucket_id, resource_id=reply_id
            ),
            "GET",
            f"/buckets/{bucket_id}/client/recordings/{recording_id}/replies/{reply_id}",
            operation="GetClientReply",
        )


class AsyncClientRepliesService(AsyncBaseService):
    async def list(
        self, *, bucket_id: int, recording_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(
                service="clientreplies",
                operation="list",
                is_mutation=False,
                project_id=bucket_id,
                resource_id=recording_id,
            ),
            f"/buckets/{bucket_id}/client/recordings/{recording_id}/replies.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListClientReplies",
        )

    async def get(self, *, bucket_id: int, recording_id: int, reply_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="clientreplies", operation="get", is_mutation=False, project_id=bucket_id, resource_id=reply_id
            ),
            "GET",
            f"/buckets/{bucket_id}/client/recordings/{recording_id}/replies/{reply_id}",
            operation="GetClientReply",
        )
