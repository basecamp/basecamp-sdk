# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TimesheetsService(BaseService):
    def for_project(
        self,
        *,
        project_id: int,
        from_: str | None = None,
        to: str | None = None,
        person_id: int | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get timesheet for a specific project.

        Args:
            project_id: The project id.
            from_: The from.
            to: The to.
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="timesheets", operation="for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/timesheet.json",
            params={
                k: v
                for k, v in {"from": from_, "to": to, "person_id": person_id, "page": page}.items()
                if v is not None
            },
            max_items=max_items,
            operation="GetProjectTimesheet",
        )

    def for_recording(
        self,
        *,
        recording_id: int,
        from_: str | None = None,
        to: str | None = None,
        person_id: int | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get timesheet for a specific recording.

        Args:
            recording_id: The recording id.
            from_: The from.
            to: The to.
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="timesheets", operation="for_recording", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/timesheet.json",
            params={
                k: v
                for k, v in {"from": from_, "to": to, "person_id": person_id, "page": page}.items()
                if v is not None
            },
            max_items=max_items,
            operation="GetRecordingTimesheet",
        )

    def create(
        self, *, recording_id: int, date: str, hours: str, description: str | None = None, person_id: int | None = None
    ) -> dict[str, Any]:
        """Create a timesheet entry on a recording.

        Args:
            recording_id: The recording id.
            date: The date.
            hours: The hours.
            description: The description.
            person_id: The person id.
        """
        return self._request(
            OperationInfo(service="timesheets", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/timesheet/entries.json",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="CreateTimesheetEntry",
        )

    def report(self, *, from_: str | None = None, to: str | None = None, person_id: int | None = None) -> ListResult:
        """Get account-wide timesheet report.

        Args:
            from_: The from.
            to: The to.
            person_id: The person id.
        """
        return self._request_list(
            OperationInfo(service="timesheets", operation="report", is_mutation=False),
            "/reports/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
            operation="GetTimesheetReport",
        )

    def get(self, *, entry_id: int) -> dict[str, Any]:
        """Get a single timesheet entry.

        Args:
            entry_id: The entry id.
        """
        return self._request(
            OperationInfo(service="timesheets", operation="get", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/timesheet_entries/{entry_id}",
            operation="GetTimesheetEntry",
        )

    def update(
        self,
        *,
        entry_id: int,
        date: str | None = None,
        hours: str | None = None,
        description: str | None = None,
        person_id: int | None = None,
    ) -> dict[str, Any]:
        """Update a timesheet entry.

        Args:
            entry_id: The entry id.
            date: The date.
            hours: The hours.
            description: The description.
            person_id: The person id.
        """
        return self._request(
            OperationInfo(service="timesheets", operation="update", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/timesheet_entries/{entry_id}",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="UpdateTimesheetEntry",
        )

    def destroy(self, *, entry_id: int) -> None:
        """Permanently delete a timesheet entry; answers 403 when the caller may not archive or trash it.

        Args:
            entry_id: The entry id.
        """
        self._request_void(
            OperationInfo(service="timesheets", operation="destroy", is_mutation=True, resource_id=entry_id),
            "DELETE",
            f"/timesheet_entries/{entry_id}",
            operation="DestroyTimesheetEntry",
        )


class AsyncTimesheetsService(AsyncBaseService):
    async def for_project(
        self,
        *,
        project_id: int,
        from_: str | None = None,
        to: str | None = None,
        person_id: int | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get timesheet for a specific project.

        Args:
            project_id: The project id.
            from_: The from.
            to: The to.
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="timesheets", operation="for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/timesheet.json",
            params={
                k: v
                for k, v in {"from": from_, "to": to, "person_id": person_id, "page": page}.items()
                if v is not None
            },
            max_items=max_items,
            operation="GetProjectTimesheet",
        )

    async def for_recording(
        self,
        *,
        recording_id: int,
        from_: str | None = None,
        to: str | None = None,
        person_id: int | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get timesheet for a specific recording.

        Args:
            recording_id: The recording id.
            from_: The from.
            to: The to.
            person_id: The person id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="timesheets", operation="for_recording", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/timesheet.json",
            params={
                k: v
                for k, v in {"from": from_, "to": to, "person_id": person_id, "page": page}.items()
                if v is not None
            },
            max_items=max_items,
            operation="GetRecordingTimesheet",
        )

    async def create(
        self, *, recording_id: int, date: str, hours: str, description: str | None = None, person_id: int | None = None
    ) -> dict[str, Any]:
        """Create a timesheet entry on a recording.

        Args:
            recording_id: The recording id.
            date: The date.
            hours: The hours.
            description: The description.
            person_id: The person id.
        """
        return await self._request(
            OperationInfo(service="timesheets", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/timesheet/entries.json",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="CreateTimesheetEntry",
        )

    async def report(
        self, *, from_: str | None = None, to: str | None = None, person_id: int | None = None
    ) -> ListResult:
        """Get account-wide timesheet report.

        Args:
            from_: The from.
            to: The to.
            person_id: The person id.
        """
        return await self._request_list(
            OperationInfo(service="timesheets", operation="report", is_mutation=False),
            "/reports/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
            operation="GetTimesheetReport",
        )

    async def get(self, *, entry_id: int) -> dict[str, Any]:
        """Get a single timesheet entry.

        Args:
            entry_id: The entry id.
        """
        return await self._request(
            OperationInfo(service="timesheets", operation="get", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/timesheet_entries/{entry_id}",
            operation="GetTimesheetEntry",
        )

    async def update(
        self,
        *,
        entry_id: int,
        date: str | None = None,
        hours: str | None = None,
        description: str | None = None,
        person_id: int | None = None,
    ) -> dict[str, Any]:
        """Update a timesheet entry.

        Args:
            entry_id: The entry id.
            date: The date.
            hours: The hours.
            description: The description.
            person_id: The person id.
        """
        return await self._request(
            OperationInfo(service="timesheets", operation="update", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/timesheet_entries/{entry_id}",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="UpdateTimesheetEntry",
        )

    async def destroy(self, *, entry_id: int) -> None:
        """Permanently delete a timesheet entry; answers 403 when the caller may not archive or trash it.

        Args:
            entry_id: The entry id.
        """
        await self._request_void(
            OperationInfo(service="timesheets", operation="destroy", is_mutation=True, resource_id=entry_id),
            "DELETE",
            f"/timesheet_entries/{entry_id}",
            operation="DestroyTimesheetEntry",
        )
