# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class BookmarksService(BaseService):
    def list_my_bookmarks(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List the current user's bookmarks, most recently bookmarked first (paginated).
        A bookmark is a personal link between the current user and a single recording,
        visible only to its creator; each entry wraps the shared recording projection.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="bookmarks", operation="list_my_bookmarks", is_mutation=False),
            "/my/bookmarks.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListMyBookmarks",
        )

    def get_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        """Report whether the current user has bookmarked the recording.

        Args:
            recording_id: The recording id.
        """
        return self._request(
            OperationInfo(service="bookmarks", operation="get_bookmark", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}/bookmark.json",
            operation="GetBookmark",
        )

    def create_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        """Bookmark a recording for the current user.
        Idempotent: re-bookmarking returns the existing bookmark, never a duplicate.

        Args:
            recording_id: The recording id.
        """
        return self._request(
            OperationInfo(service="bookmarks", operation="create_bookmark", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/bookmark.json",
            operation="CreateBookmark",
        )

    def delete_bookmark(self, *, recording_id: int) -> None:
        """Remove the current user's bookmark from a recording (returns 204 No Content).
        Idempotent: deleting an absent bookmark also returns 204.

        Args:
            recording_id: The recording id.
        """
        self._request_void(
            OperationInfo(service="bookmarks", operation="delete_bookmark", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/bookmark.json",
            operation="DeleteBookmark",
        )


class AsyncBookmarksService(AsyncBaseService):
    async def list_my_bookmarks(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List the current user's bookmarks, most recently bookmarked first (paginated).
        A bookmark is a personal link between the current user and a single recording,
        visible only to its creator; each entry wraps the shared recording projection.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="bookmarks", operation="list_my_bookmarks", is_mutation=False),
            "/my/bookmarks.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListMyBookmarks",
        )

    async def get_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        """Report whether the current user has bookmarked the recording.

        Args:
            recording_id: The recording id.
        """
        return await self._request(
            OperationInfo(service="bookmarks", operation="get_bookmark", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}/bookmark.json",
            operation="GetBookmark",
        )

    async def create_bookmark(self, *, recording_id: int) -> dict[str, Any]:
        """Bookmark a recording for the current user.
        Idempotent: re-bookmarking returns the existing bookmark, never a duplicate.

        Args:
            recording_id: The recording id.
        """
        return await self._request(
            OperationInfo(service="bookmarks", operation="create_bookmark", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/bookmark.json",
            operation="CreateBookmark",
        )

    async def delete_bookmark(self, *, recording_id: int) -> None:
        """Remove the current user's bookmark from a recording (returns 204 No Content).
        Idempotent: deleting an absent bookmark also returns 204.

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(service="bookmarks", operation="delete_bookmark", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/bookmark.json",
            operation="DeleteBookmark",
        )
