# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MyNotificationsService(BaseService):
    def get_my_notifications(self, *, page: int | None = None, limit_bubble_ups: bool | None = None) -> dict[str, Any]:
        """Get the current user's notification inbox (the "Hey!" menu).
        Notifications are grouped into unreads, reads, bubble-ups, and
        scheduled bubble-ups (`memories` remains as an always-empty
        placeholder on BC5). Reads are paginated (50 per page). Unreads are
        capped at 100. Bubble-ups are capped per `limit_bubble_ups`.

        Args:
            page: Page number for paginating through read items. Defaults to 1. This operation is
                not auto-paginated in any SDK, so a page is returned as asked for and later pages
                are not followed.
            limit_bubble_ups: Set to true to cap `bubble_ups` at 2 current bubble-ups and omit the
                `scheduled_bubble_ups` key entirely. Defaults to false. Use the dedicated bubble-ups
                endpoint (GetBubbleUps) to page through all current and scheduled bubble-ups.
        """
        return self._request(
            OperationInfo(service="mynotifications", operation="get_my_notifications", is_mutation=False),
            "GET",
            "/my/readings.json",
            params=self._compact(page=page, limit_bubble_ups=limit_bubble_ups),
            operation="GetMyNotifications",
        )

    def get_bubble_ups(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get the current user's current and scheduled bubble-ups (paginated, 50 per page).
        Current bubble-ups are returned first, ordered by most recently bubbled up;
        scheduled bubble-ups follow, ordered by scheduled bubble-up time. Each item
        uses the same notification object shape as GetMyNotifications.

        Args:
            page: Page number. Defaults to 1. A positive value selects exactly that page, not a
                starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="mynotifications", operation="get_bubble_ups", is_mutation=False),
            "/my/readings/bubble_ups.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetBubbleUps",
        )

    def mark_as_read(self, *, readables: list[str]) -> None:
        """Mark specified items as read.

        Args:
            readables: Array of readable_sgid values identifying the items to mark as read
        """
        self._request_void(
            OperationInfo(service="mynotifications", operation="mark_as_read", is_mutation=True),
            "PUT",
            "/my/unreads.json",
            json_body=self._compact(readables=readables),
            operation="MarkAsRead",
        )


class AsyncMyNotificationsService(AsyncBaseService):
    async def get_my_notifications(
        self, *, page: int | None = None, limit_bubble_ups: bool | None = None
    ) -> dict[str, Any]:
        """Get the current user's notification inbox (the "Hey!" menu).
        Notifications are grouped into unreads, reads, bubble-ups, and
        scheduled bubble-ups (`memories` remains as an always-empty
        placeholder on BC5). Reads are paginated (50 per page). Unreads are
        capped at 100. Bubble-ups are capped per `limit_bubble_ups`.

        Args:
            page: Page number for paginating through read items. Defaults to 1. This operation is
                not auto-paginated in any SDK, so a page is returned as asked for and later pages
                are not followed.
            limit_bubble_ups: Set to true to cap `bubble_ups` at 2 current bubble-ups and omit the
                `scheduled_bubble_ups` key entirely. Defaults to false. Use the dedicated bubble-ups
                endpoint (GetBubbleUps) to page through all current and scheduled bubble-ups.
        """
        return await self._request(
            OperationInfo(service="mynotifications", operation="get_my_notifications", is_mutation=False),
            "GET",
            "/my/readings.json",
            params=self._compact(page=page, limit_bubble_ups=limit_bubble_ups),
            operation="GetMyNotifications",
        )

    async def get_bubble_ups(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get the current user's current and scheduled bubble-ups (paginated, 50 per page).
        Current bubble-ups are returned first, ordered by most recently bubbled up;
        scheduled bubble-ups follow, ordered by scheduled bubble-up time. Each item
        uses the same notification object shape as GetMyNotifications.

        Args:
            page: Page number. Defaults to 1. A positive value selects exactly that page, not a
                starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="mynotifications", operation="get_bubble_ups", is_mutation=False),
            "/my/readings/bubble_ups.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetBubbleUps",
        )

    async def mark_as_read(self, *, readables: list[str]) -> None:
        """Mark specified items as read.

        Args:
            readables: Array of readable_sgid values identifying the items to mark as read
        """
        await self._request_void(
            OperationInfo(service="mynotifications", operation="mark_as_read", is_mutation=True),
            "PUT",
            "/my/unreads.json",
            json_body=self._compact(readables=readables),
            operation="MarkAsRead",
        )
