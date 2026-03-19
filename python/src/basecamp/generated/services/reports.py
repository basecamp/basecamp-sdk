# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ReportsService(BaseService):
    def progress(self) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="reports", operation="progress", is_mutation=False), "/reports/progress.json"
        )

    def upcoming(self, *, window_starts_on: str | None = None, window_ends_on: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="reports", operation="upcoming", is_mutation=False),
            "GET",
            "/reports/schedules/upcoming.json",
            params=self._compact(window_starts_on=window_starts_on, window_ends_on=window_ends_on),
        )

    def assigned(self, *, person_id: int | str, group_by: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="reports", operation="assigned", is_mutation=False, resource_id=person_id),
            "GET",
            f"/reports/todos/assigned/{person_id}",
            params=self._compact(group_by=group_by),
        )

    def overdue(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="reports", operation="overdue", is_mutation=False),
            "GET",
            "/reports/todos/overdue.json",
        )

    def person_progress(self, *, person_id: int | str) -> dict[str, Any]:
        return self._request_paginated_wrapped(
            OperationInfo(service="reports", operation="person_progress", is_mutation=False, resource_id=person_id),
            f"/reports/users/progress/{person_id}.json",
            "events",
        )


class AsyncReportsService(AsyncBaseService):
    async def progress(self) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="reports", operation="progress", is_mutation=False), "/reports/progress.json"
        )

    async def upcoming(
        self, *, window_starts_on: str | None = None, window_ends_on: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="reports", operation="upcoming", is_mutation=False),
            "GET",
            "/reports/schedules/upcoming.json",
            params=self._compact(window_starts_on=window_starts_on, window_ends_on=window_ends_on),
        )

    async def assigned(self, *, person_id: int | str, group_by: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="reports", operation="assigned", is_mutation=False, resource_id=person_id),
            "GET",
            f"/reports/todos/assigned/{person_id}",
            params=self._compact(group_by=group_by),
        )

    async def overdue(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="reports", operation="overdue", is_mutation=False),
            "GET",
            "/reports/todos/overdue.json",
        )

    async def person_progress(self, *, person_id: int | str) -> dict[str, Any]:
        return await self._request_paginated_wrapped(
            OperationInfo(service="reports", operation="person_progress", is_mutation=False, resource_id=person_id),
            f"/reports/users/progress/{person_id}.json",
            "events",
        )
