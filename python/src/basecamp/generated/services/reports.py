# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ReportsService(BaseService):
    def progress(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get account-wide activity feed (progress report).

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="reports", operation="progress", is_mutation=False),
            "/reports/progress.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetProgressReport",
        )

    def upcoming(self, *, window_starts_on: str, window_ends_on: str) -> dict[str, Any]:
        """Get upcoming schedule entries and assignable items within a date window.
        This endpoint is preserved as the canonical API path on BC5;
        the BC5 `/calendar` web view is HTML-only.

        The three arrays carry the report's own reduced projections —
        UpcomingScheduleEntry and UpcomingAssignable — not the shared ScheduleEntry
        and Todo/Card shapes. BC3 renders this report through
        `app/views/api/schedules/calendar/_entry.json.jbuilder` and
        `_assignable.json.jbuilder`, which emit a narrower key set than the
        per-resource partials, plus keys those partials never emit. See
        UpcomingScheduleEntry for the full accounting.

        Args:
            window_starts_on: Inclusive first day of the window, `YYYY-MM-DD`. Required — BC3
                answers 400 without it.
            window_ends_on: Inclusive last day of the window, `YYYY-MM-DD`. Required — BC3 answers
                400 without it.
        """
        return self._request(
            OperationInfo(service="reports", operation="upcoming", is_mutation=False),
            "GET",
            "/reports/schedules/upcoming.json",
            params=self._compact(window_starts_on=window_starts_on, window_ends_on=window_ends_on),
            operation="GetUpcomingSchedule",
        )

    def assigned(
        self, *, person_id: int, group_by: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> dict[str, Any]:
        """Get todos assigned to a specific person, 50 per page. Every page repeats
        `person` and `grouped_by`; `todos` holds that page's to-dos.

        Args:
            person_id: The person id.
            group_by: Group by "bucket" or "date"
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated_wrapped(
            OperationInfo(service="reports", operation="assigned", is_mutation=False, resource_id=person_id),
            f"/reports/todos/assigned/{person_id}",
            "todos",
            params=self._compact(group_by=group_by, page=page),
            max_items=max_items,
            operation="GetAssignedTodos",
        )

    def overdue(self, *, page: int | None = None) -> dict[str, Any]:
        """Get one page of overdue todos grouped by lateness, latest first.

        Pages hold 15, 30, 50, then 100 to-dos, and each page fills the same four
        groups with only its own to-dos, so a group can continue onto the next
        page. A `Link: rel="next"` header names the next page and `X-Total-Count`
        the total. This operation is not auto-paginated in any SDK: the response
        is an object of four arrays, which no pagination key can name.

        Args:
            page: Page number for paginating through results. Defaults to 1. This operation is not
                auto-paginated in any SDK, so a page is returned as asked for and later pages are
                not followed.
        """
        return self._request(
            OperationInfo(service="reports", operation="overdue", is_mutation=False),
            "GET",
            "/reports/todos/overdue.json",
            params=self._compact(page=page),
            operation="GetOverdueTodos",
        )

    def person_progress(
        self, *, person_id: int, page: int | None = None, max_items: int | None = None
    ) -> dict[str, Any]:
        """Get a person's activity timeline.

        Args:
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated_wrapped(
            OperationInfo(service="reports", operation="person_progress", is_mutation=False, resource_id=person_id),
            f"/reports/users/progress/{person_id}.json",
            "events",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetPersonProgress",
        )


class AsyncReportsService(AsyncBaseService):
    async def progress(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get account-wide activity feed (progress report).

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="reports", operation="progress", is_mutation=False),
            "/reports/progress.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetProgressReport",
        )

    async def upcoming(self, *, window_starts_on: str, window_ends_on: str) -> dict[str, Any]:
        """Get upcoming schedule entries and assignable items within a date window.
        This endpoint is preserved as the canonical API path on BC5;
        the BC5 `/calendar` web view is HTML-only.

        The three arrays carry the report's own reduced projections —
        UpcomingScheduleEntry and UpcomingAssignable — not the shared ScheduleEntry
        and Todo/Card shapes. BC3 renders this report through
        `app/views/api/schedules/calendar/_entry.json.jbuilder` and
        `_assignable.json.jbuilder`, which emit a narrower key set than the
        per-resource partials, plus keys those partials never emit. See
        UpcomingScheduleEntry for the full accounting.

        Args:
            window_starts_on: Inclusive first day of the window, `YYYY-MM-DD`. Required — BC3
                answers 400 without it.
            window_ends_on: Inclusive last day of the window, `YYYY-MM-DD`. Required — BC3 answers
                400 without it.
        """
        return await self._request(
            OperationInfo(service="reports", operation="upcoming", is_mutation=False),
            "GET",
            "/reports/schedules/upcoming.json",
            params=self._compact(window_starts_on=window_starts_on, window_ends_on=window_ends_on),
            operation="GetUpcomingSchedule",
        )

    async def assigned(
        self, *, person_id: int, group_by: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> dict[str, Any]:
        """Get todos assigned to a specific person, 50 per page. Every page repeats
        `person` and `grouped_by`; `todos` holds that page's to-dos.

        Args:
            person_id: The person id.
            group_by: Group by "bucket" or "date"
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated_wrapped(
            OperationInfo(service="reports", operation="assigned", is_mutation=False, resource_id=person_id),
            f"/reports/todos/assigned/{person_id}",
            "todos",
            params=self._compact(group_by=group_by, page=page),
            max_items=max_items,
            operation="GetAssignedTodos",
        )

    async def overdue(self, *, page: int | None = None) -> dict[str, Any]:
        """Get one page of overdue todos grouped by lateness, latest first.

        Pages hold 15, 30, 50, then 100 to-dos, and each page fills the same four
        groups with only its own to-dos, so a group can continue onto the next
        page. A `Link: rel="next"` header names the next page and `X-Total-Count`
        the total. This operation is not auto-paginated in any SDK: the response
        is an object of four arrays, which no pagination key can name.

        Args:
            page: Page number for paginating through results. Defaults to 1. This operation is not
                auto-paginated in any SDK, so a page is returned as asked for and later pages are
                not followed.
        """
        return await self._request(
            OperationInfo(service="reports", operation="overdue", is_mutation=False),
            "GET",
            "/reports/todos/overdue.json",
            params=self._compact(page=page),
            operation="GetOverdueTodos",
        )

    async def person_progress(
        self, *, person_id: int, page: int | None = None, max_items: int | None = None
    ) -> dict[str, Any]:
        """Get a person's activity timeline.

        Args:
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated_wrapped(
            OperationInfo(service="reports", operation="person_progress", is_mutation=False, resource_id=person_id),
            f"/reports/users/progress/{person_id}.json",
            "events",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetPersonProgress",
        )
