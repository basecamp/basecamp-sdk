# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class SchedulesService(BaseService):
    def get_entry(self, *, entry_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="get_entry", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/schedule_entries/{entry_id}",
        )

    def update_entry(
        self,
        *,
        entry_id: int | str,
        summary: str | None = None,
        starts_at: str | None = None,
        ends_at: str | None = None,
        description: str | None = None,
        participant_ids: list | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="update_entry", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/schedule_entries/{entry_id}",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
            ),
            operation="UpdateScheduleEntry",
        )

    def get_entry_occurrence(self, *, entry_id: int | str, date: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="get_entry_occurrence", is_mutation=False, resource_id=date),
            "GET",
            f"/schedule_entries/{entry_id}/occurrences/{date}",
        )

    def get(self, *, schedule_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="get", is_mutation=False, resource_id=schedule_id),
            "GET",
            f"/schedules/{schedule_id}",
        )

    def update_settings(self, *, schedule_id: int | str, include_due_assignments: bool) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="update_settings", is_mutation=True, resource_id=schedule_id),
            "PUT",
            f"/schedules/{schedule_id}",
            json_body=self._compact(include_due_assignments=include_due_assignments),
            operation="UpdateScheduleSettings",
        )

    def list_entries(self, *, schedule_id: int | str, status: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="schedules", operation="list_entries", is_mutation=False, resource_id=schedule_id),
            f"/schedules/{schedule_id}/entries.json",
            params=self._compact(status=status),
        )

    def create_entry(
        self,
        *,
        schedule_id: int | str,
        summary: str,
        starts_at: str,
        ends_at: str,
        description: str | None = None,
        participant_ids: list | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="create_entry", is_mutation=True, resource_id=schedule_id),
            "POST",
            f"/schedules/{schedule_id}/entries.json",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
                subscriptions=subscriptions,
            ),
            operation="CreateScheduleEntry",
        )


class AsyncSchedulesService(AsyncBaseService):
    async def get_entry(self, *, entry_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="get_entry", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/schedule_entries/{entry_id}",
        )

    async def update_entry(
        self,
        *,
        entry_id: int | str,
        summary: str | None = None,
        starts_at: str | None = None,
        ends_at: str | None = None,
        description: str | None = None,
        participant_ids: list | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="update_entry", is_mutation=True, resource_id=entry_id),
            "PUT",
            f"/schedule_entries/{entry_id}",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
            ),
            operation="UpdateScheduleEntry",
        )

    async def get_entry_occurrence(self, *, entry_id: int | str, date: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="get_entry_occurrence", is_mutation=False, resource_id=date),
            "GET",
            f"/schedule_entries/{entry_id}/occurrences/{date}",
        )

    async def get(self, *, schedule_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="get", is_mutation=False, resource_id=schedule_id),
            "GET",
            f"/schedules/{schedule_id}",
        )

    async def update_settings(self, *, schedule_id: int | str, include_due_assignments: bool) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="update_settings", is_mutation=True, resource_id=schedule_id),
            "PUT",
            f"/schedules/{schedule_id}",
            json_body=self._compact(include_due_assignments=include_due_assignments),
            operation="UpdateScheduleSettings",
        )

    async def list_entries(self, *, schedule_id: int | str, status: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="schedules", operation="list_entries", is_mutation=False, resource_id=schedule_id),
            f"/schedules/{schedule_id}/entries.json",
            params=self._compact(status=status),
        )

    async def create_entry(
        self,
        *,
        schedule_id: int | str,
        summary: str,
        starts_at: str,
        ends_at: str,
        description: str | None = None,
        participant_ids: list | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        subscriptions: list | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="create_entry", is_mutation=True, resource_id=schedule_id),
            "POST",
            f"/schedules/{schedule_id}/entries.json",
            json_body=self._compact(
                summary=summary,
                starts_at=starts_at,
                ends_at=ends_at,
                description=description,
                participant_ids=participant_ids,
                all_day=all_day,
                notify=notify,
                subscriptions=subscriptions,
            ),
            operation="CreateScheduleEntry",
        )
