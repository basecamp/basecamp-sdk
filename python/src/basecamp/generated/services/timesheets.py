# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TimesheetsService(BaseService):
    def for_project(
        self, *, project_id: int | str, from_: str | None = None, to: str | None = None, person_id: int | None = None
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="timesheets", operation="for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
        )

    def for_recording(
        self, *, recording_id: int | str, from_: str | None = None, to: str | None = None, person_id: int | None = None
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="timesheets", operation="for_recording", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
        )

    def create(
        self,
        *,
        recording_id: int | str,
        date: str,
        hours: str,
        description: str | None = None,
        person_id: int | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="timesheets", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/timesheet/entries.json",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="CreateTimesheetEntry",
        )

    def report(
        self, *, from_: str | None = None, to: str | None = None, person_id: int | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="timesheets", operation="report", is_mutation=False),
            "GET",
            "/reports/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
        )

    def get(self, *, entry_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="timesheets", operation="get", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/timesheet_entries/{entry_id}",
        )

    def update(
        self,
        *,
        entry_id: int | str,
        date: str | None = None,
        hours: str | None = None,
        description: str | None = None,
        person_id: int | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="timesheets", operation="update", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/timesheet_entries/{entry_id}",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="UpdateTimesheetEntry",
        )


class AsyncTimesheetsService(AsyncBaseService):
    async def for_project(
        self, *, project_id: int | str, from_: str | None = None, to: str | None = None, person_id: int | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="timesheets", operation="for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
        )

    async def for_recording(
        self, *, recording_id: int | str, from_: str | None = None, to: str | None = None, person_id: int | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="timesheets", operation="for_recording", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
        )

    async def create(
        self,
        *,
        recording_id: int | str,
        date: str,
        hours: str,
        description: str | None = None,
        person_id: int | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="timesheets", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/timesheet/entries.json",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="CreateTimesheetEntry",
        )

    async def report(
        self, *, from_: str | None = None, to: str | None = None, person_id: int | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="timesheets", operation="report", is_mutation=False),
            "GET",
            "/reports/timesheet.json",
            params={k: v for k, v in {"from": from_, "to": to, "person_id": person_id}.items() if v is not None},
        )

    async def get(self, *, entry_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="timesheets", operation="get", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/timesheet_entries/{entry_id}",
        )

    async def update(
        self,
        *,
        entry_id: int | str,
        date: str | None = None,
        hours: str | None = None,
        description: str | None = None,
        person_id: int | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="timesheets", operation="update", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/timesheet_entries/{entry_id}",
            json_body=self._compact(date=date, hours=hours, description=description, person_id=person_id),
            operation="UpdateTimesheetEntry",
        )
