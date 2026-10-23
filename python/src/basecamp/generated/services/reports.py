# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ReportsService(BaseService):
    def progress(
        self,
        *,
        date: str | None = None,
        project_ids: list[int] | None = None,
        people_ids: list[int] | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get account-wide activity feed (progress report).

        Args:
            date: The day to read (YYYY-MM-DD). The feed serves one day per request and defaults to
                today, so step this back a day at a time to read history; `page` pages within the
                day. The server reads an unparseable date as today.
            project_ids: Only events in these projects. When omitted, the server applies the project
                filter the caller last saved in the web's activity view, not "all projects". If none
                of the IDs is a project the caller can access, the response is empty and the server
                clears that saved filter, so send only IDs the API just returned.
            people_ids: Only events created by these people. When omitted, the server applies the
                people filter the caller last saved in the web's activity view, not "everyone". If
                none of the IDs is a person the caller can see, the response is empty and the server
                clears that saved filter, so send only IDs the API just returned.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="reports", operation="progress", is_mutation=False),
            "/reports/progress.json",
            params={
                k: v
                for k, v in {
                    "date": date,
                    "project_ids[]": project_ids,
                    "people_ids[]": people_ids,
                    "page": page,
                }.items()
                if v is not None
            },
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

    def assigned(self, *, person_id: int, group_by: str | None = None) -> dict[str, Any]:
        """Get todos assigned to a specific person.

        Args:
            person_id: The person id.
            group_by: Group by "bucket" or "date"
        """
        return self._request(
            OperationInfo(service="reports", operation="assigned", is_mutation=False, resource_id=person_id),
            "GET",
            f"/reports/todos/assigned/{person_id}",
            params=self._compact(group_by=group_by),
            operation="GetAssignedTodos",
        )

    def overdue(self) -> dict[str, Any]:
        """Get overdue todos grouped by lateness."""
        return self._request(
            OperationInfo(service="reports", operation="overdue", is_mutation=False),
            "GET",
            "/reports/todos/overdue.json",
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
    async def progress(
        self,
        *,
        date: str | None = None,
        project_ids: list[int] | None = None,
        people_ids: list[int] | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get account-wide activity feed (progress report).

        Args:
            date: The day to read (YYYY-MM-DD). The feed serves one day per request and defaults to
                today, so step this back a day at a time to read history; `page` pages within the
                day. The server reads an unparseable date as today.
            project_ids: Only events in these projects. When omitted, the server applies the project
                filter the caller last saved in the web's activity view, not "all projects". If none
                of the IDs is a project the caller can access, the response is empty and the server
                clears that saved filter, so send only IDs the API just returned.
            people_ids: Only events created by these people. When omitted, the server applies the
                people filter the caller last saved in the web's activity view, not "everyone". If
                none of the IDs is a person the caller can see, the response is empty and the server
                clears that saved filter, so send only IDs the API just returned.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="reports", operation="progress", is_mutation=False),
            "/reports/progress.json",
            params={
                k: v
                for k, v in {
                    "date": date,
                    "project_ids[]": project_ids,
                    "people_ids[]": people_ids,
                    "page": page,
                }.items()
                if v is not None
            },
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

    async def assigned(self, *, person_id: int, group_by: str | None = None) -> dict[str, Any]:
        """Get todos assigned to a specific person.

        Args:
            person_id: The person id.
            group_by: Group by "bucket" or "date"
        """
        return await self._request(
            OperationInfo(service="reports", operation="assigned", is_mutation=False, resource_id=person_id),
            "GET",
            f"/reports/todos/assigned/{person_id}",
            params=self._compact(group_by=group_by),
            operation="GetAssignedTodos",
        )

    async def overdue(self) -> dict[str, Any]:
        """Get overdue todos grouped by lateness."""
        return await self._request(
            OperationInfo(service="reports", operation="overdue", is_mutation=False),
            "GET",
            "/reports/todos/overdue.json",
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
