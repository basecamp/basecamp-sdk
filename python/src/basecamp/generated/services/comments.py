# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CommentsService(BaseService):
    def get(self, *, comment_id: int) -> dict[str, Any]:
        """Get a single comment by id.

        Args:
            comment_id: The comment id.
        """
        return self._request(
            OperationInfo(service="comments", operation="get", is_mutation=False, resource_id=comment_id),
            "GET",
            f"/comments/{comment_id}",
            operation="GetComment",
        )

    def update(self, *, comment_id: int, content: str) -> dict[str, Any]:
        """Update an existing comment.

        Args:
            comment_id: The comment id.
            content: The content.
        """
        return self._request(
            OperationInfo(service="comments", operation="update", is_mutation=True, resource_id=comment_id),
            "PUT",
            f"/comments/{comment_id}",
            json_body=self._compact(content=content),
            operation="UpdateComment",
        )

    def list(self, *, recording_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List comments on a recording.

        Args:
            recording_id: The recording id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="comments", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/comments.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListComments",
        )

    def create(self, *, recording_id: int, content: str) -> dict[str, Any]:
        """Create a new comment on a recording.

        Args:
            recording_id: The recording id.
            content: The content.
        """
        return self._request(
            OperationInfo(service="comments", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/comments.json",
            json_body=self._compact(content=content),
            operation="CreateComment",
        )


class AsyncCommentsService(AsyncBaseService):
    async def get(self, *, comment_id: int) -> dict[str, Any]:
        """Get a single comment by id.

        Args:
            comment_id: The comment id.
        """
        return await self._request(
            OperationInfo(service="comments", operation="get", is_mutation=False, resource_id=comment_id),
            "GET",
            f"/comments/{comment_id}",
            operation="GetComment",
        )

    async def update(self, *, comment_id: int, content: str) -> dict[str, Any]:
        """Update an existing comment.

        Args:
            comment_id: The comment id.
            content: The content.
        """
        return await self._request(
            OperationInfo(service="comments", operation="update", is_mutation=True, resource_id=comment_id),
            "PUT",
            f"/comments/{comment_id}",
            json_body=self._compact(content=content),
            operation="UpdateComment",
        )

    async def list(self, *, recording_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List comments on a recording.

        Args:
            recording_id: The recording id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="comments", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/comments.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListComments",
        )

    async def create(self, *, recording_id: int, content: str) -> dict[str, Any]:
        """Create a new comment on a recording.

        Args:
            recording_id: The recording id.
            content: The content.
        """
        return await self._request(
            OperationInfo(service="comments", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/comments.json",
            json_body=self._compact(content=content),
            operation="CreateComment",
        )
