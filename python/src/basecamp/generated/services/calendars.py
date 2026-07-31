# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CalendarsService(BaseService):
    def get_calendar(self, *, calendar_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="calendars", operation="get_calendar", is_mutation=False, resource_id=calendar_id),
            "GET",
            f"/calendars/{calendar_id}",
            operation="GetCalendar",
        )

    def update_calendar(self, *, calendar_id: int, calendar: dict) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="calendars", operation="update_calendar", is_mutation=True, resource_id=calendar_id),
            "PUT",
            f"/calendars/{calendar_id}",
            json_body=self._compact(calendar=calendar),
            operation="UpdateCalendar",
        )


class AsyncCalendarsService(AsyncBaseService):
    async def get_calendar(self, *, calendar_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="calendars", operation="get_calendar", is_mutation=False, resource_id=calendar_id),
            "GET",
            f"/calendars/{calendar_id}",
            operation="GetCalendar",
        )

    async def update_calendar(self, *, calendar_id: int, calendar: dict) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="calendars", operation="update_calendar", is_mutation=True, resource_id=calendar_id),
            "PUT",
            f"/calendars/{calendar_id}",
            json_body=self._compact(calendar=calendar),
            operation="UpdateCalendar",
        )
