# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class SchedulesService(BaseService):
    def get_entry(self, *, entry_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="get_entry", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/schedule_entries/{entry_id}",
            operation="GetScheduleEntry",
        )

    def replace_entry(
        self,
        *,
        entry_id: int,
        starts_at: str,
        ends_at: str,
        summary: str | None = None,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="replace_entry", is_mutation=True, resource_id=entry_id),
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
                url=url,
                highlighted=highlighted,
            ),
            operation="ReplaceScheduleEntry",
        )

    def get_entry_occurrence(self, *, entry_id: int, date: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="schedules", operation="get_entry_occurrence", is_mutation=False, resource_id=entry_id
            ),
            "GET",
            f"/schedule_entries/{entry_id}/occurrences/{date}",
            operation="GetScheduleEntryOccurrence",
        )

    def get(self, *, schedule_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="get", is_mutation=False, resource_id=schedule_id),
            "GET",
            f"/schedules/{schedule_id}",
            operation="GetSchedule",
        )

    def update_settings(self, *, schedule_id: int, include_due_assignments: bool) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="schedules", operation="update_settings", is_mutation=True, resource_id=schedule_id),
            "PUT",
            f"/schedules/{schedule_id}",
            json_body=self._compact(include_due_assignments=include_due_assignments),
            operation="UpdateScheduleSettings",
        )

    def list_entries(
        self, *, schedule_id: int, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="schedules", operation="list_entries", is_mutation=False, resource_id=schedule_id),
            f"/schedules/{schedule_id}/entries.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListScheduleEntries",
        )

    def create_entry(
        self,
        *,
        schedule_id: int,
        summary: str,
        starts_at: str,
        ends_at: str,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
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
                url=url,
                highlighted=highlighted,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateScheduleEntry",
        )


class AsyncSchedulesService(AsyncBaseService):
    async def get_entry(self, *, entry_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="get_entry", is_mutation=False, resource_id=entry_id),
            "GET",
            f"/schedule_entries/{entry_id}",
            operation="GetScheduleEntry",
        )

    async def replace_entry(
        self,
        *,
        entry_id: int,
        starts_at: str,
        ends_at: str,
        summary: str | None = None,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="replace_entry", is_mutation=True, resource_id=entry_id),
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
                url=url,
                highlighted=highlighted,
            ),
            operation="ReplaceScheduleEntry",
        )

    async def get_entry_occurrence(self, *, entry_id: int, date: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="schedules", operation="get_entry_occurrence", is_mutation=False, resource_id=entry_id
            ),
            "GET",
            f"/schedule_entries/{entry_id}/occurrences/{date}",
            operation="GetScheduleEntryOccurrence",
        )

    async def get(self, *, schedule_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="get", is_mutation=False, resource_id=schedule_id),
            "GET",
            f"/schedules/{schedule_id}",
            operation="GetSchedule",
        )

    async def update_settings(self, *, schedule_id: int, include_due_assignments: bool) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="schedules", operation="update_settings", is_mutation=True, resource_id=schedule_id),
            "PUT",
            f"/schedules/{schedule_id}",
            json_body=self._compact(include_due_assignments=include_due_assignments),
            operation="UpdateScheduleSettings",
        )

    async def list_entries(
        self, *, schedule_id: int, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="schedules", operation="list_entries", is_mutation=False, resource_id=schedule_id),
            f"/schedules/{schedule_id}/entries.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListScheduleEntries",
        )

    async def create_entry(
        self,
        *,
        schedule_id: int,
        summary: str,
        starts_at: str,
        ends_at: str,
        description: str | None = None,
        participant_ids: list[int] | None = None,
        all_day: bool | None = None,
        notify: bool | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
        status: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
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
                url=url,
                highlighted=highlighted,
                status=status,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateScheduleEntry",
        )
