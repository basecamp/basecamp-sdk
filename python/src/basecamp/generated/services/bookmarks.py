# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class BookmarksService(BaseService):
    def list_my_bookmarks(self, *, page: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="bookmarks", operation="list_my_bookmarks", is_mutation=False),
            "/my/bookmarks.json",
            params=self._compact(page=page),
            operation="ListMyBookmarks",
        )

    def get_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="bookmarks", operation="get_bookmark", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}/bookmark.json",
            operation="GetBookmark",
        )

    def create_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="bookmarks", operation="create_bookmark", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/bookmark.json",
            operation="CreateBookmark",
        )

    def delete_bookmark(self, *, recording_id: int) -> None:
        self._request_void(
            OperationInfo(service="bookmarks", operation="delete_bookmark", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/bookmark.json",
            operation="DeleteBookmark",
        )


class AsyncBookmarksService(AsyncBaseService):
    async def list_my_bookmarks(self, *, page: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="bookmarks", operation="list_my_bookmarks", is_mutation=False),
            "/my/bookmarks.json",
            params=self._compact(page=page),
            operation="ListMyBookmarks",
        )

    async def get_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="bookmarks", operation="get_bookmark", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}/bookmark.json",
            operation="GetBookmark",
        )

    async def create_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="bookmarks", operation="create_bookmark", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/bookmark.json",
            operation="CreateBookmark",
        )

    async def delete_bookmark(self, *, recording_id: int) -> None:
        await self._request_void(
            OperationInfo(service="bookmarks", operation="delete_bookmark", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/bookmark.json",
            operation="DeleteBookmark",
        )
