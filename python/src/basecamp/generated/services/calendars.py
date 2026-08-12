# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CalendarsService(BaseService):
    def get_calendar(self, *, calendar_id: int) -> dict[str, Any]:
        """Get a calendar by its bucket id. A Calendar is a top-level BC5 bucketable
        (distinct from a project) exposing display metadata and a link to its
        underlying schedule resource. Shipped scope is show + update only.

        Args:
            calendar_id: The calendar id.
        """
        return self._request(
            OperationInfo(service="calendars", operation="get_calendar", is_mutation=False, resource_id=calendar_id),
            "GET",
            f"/calendars/{calendar_id}",
            operation="GetCalendar",
        )

    def update_calendar(self, *, calendar_id: int, calendar: dict) -> dict[str, Any]:
        """Update a calendar's display color. An unknown color returns 422 with a JSON
        errors payload keyed by field ({"errors": {"color": ["is not a valid
        color"]}}) — the controller rejects invalid enum values up front.

        Args:
            calendar_id: The calendar id.
            calendar: The calendar.
        """
        return self._request(
            OperationInfo(service="calendars", operation="update_calendar", is_mutation=True, resource_id=calendar_id),
            "PUT",
            f"/calendars/{calendar_id}",
            json_body=self._compact(calendar=calendar),
            operation="UpdateCalendar",
        )


class AsyncCalendarsService(AsyncBaseService):
    async def get_calendar(self, *, calendar_id: int) -> dict[str, Any]:
        """Get a calendar by its bucket id. A Calendar is a top-level BC5 bucketable
        (distinct from a project) exposing display metadata and a link to its
        underlying schedule resource. Shipped scope is show + update only.

        Args:
            calendar_id: The calendar id.
        """
        return await self._request(
            OperationInfo(service="calendars", operation="get_calendar", is_mutation=False, resource_id=calendar_id),
            "GET",
            f"/calendars/{calendar_id}",
            operation="GetCalendar",
        )

    async def update_calendar(self, *, calendar_id: int, calendar: dict) -> dict[str, Any]:
        """Update a calendar's display color. An unknown color returns 422 with a JSON
        errors payload keyed by field ({"errors": {"color": ["is not a valid
        color"]}}) — the controller rejects invalid enum values up front.

        Args:
            calendar_id: The calendar id.
            calendar: The calendar.
        """
        return await self._request(
            OperationInfo(service="calendars", operation="update_calendar", is_mutation=True, resource_id=calendar_id),
            "PUT",
            f"/calendars/{calendar_id}",
            json_body=self._compact(calendar=calendar),
            operation="UpdateCalendar",
        )
