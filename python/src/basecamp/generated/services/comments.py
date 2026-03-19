# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CommentsService(BaseService):
    def get(self, *, comment_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="comments", operation="get", is_mutation=False, resource_id=comment_id),
            "GET",
            f"/comments/{comment_id}",
        )

    def update(self, *, comment_id: int | str, content: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="comments", operation="update", is_mutation=True, resource_id=comment_id),
            "PUT",
            f"/comments/{comment_id}",
            json_body=self._compact(content=content),
            operation="UpdateComment",
        )

    def list(self, *, recording_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="comments", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/comments.json",
        )

    def create(self, *, recording_id: int | str, content: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="comments", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/comments.json",
            json_body=self._compact(content=content),
            operation="CreateComment",
        )


class AsyncCommentsService(AsyncBaseService):
    async def get(self, *, comment_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="comments", operation="get", is_mutation=False, resource_id=comment_id),
            "GET",
            f"/comments/{comment_id}",
        )

    async def update(self, *, comment_id: int | str, content: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="comments", operation="update", is_mutation=True, resource_id=comment_id),
            "PUT",
            f"/comments/{comment_id}",
            json_body=self._compact(content=content),
            operation="UpdateComment",
        )

    async def list(self, *, recording_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="comments", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/comments.json",
        )

    async def create(self, *, recording_id: int | str, content: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="comments", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/comments.json",
            json_body=self._compact(content=content),
            operation="CreateComment",
        )
