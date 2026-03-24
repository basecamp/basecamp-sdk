# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MyAssignmentsService(BaseService):
    def get_my_assignments(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="myassignments", operation="get_my_assignments", is_mutation=False),
            "GET",
            "/my/assignments.json",
        )

    def get_my_completed_assignments(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="myassignments", operation="get_my_completed_assignments", is_mutation=False),
            "GET",
            "/my/assignments/completed.json",
        )

    def get_my_due_assignments(self, *, scope: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="myassignments", operation="get_my_due_assignments", is_mutation=False),
            "GET",
            "/my/assignments/due.json",
            params=self._compact(scope=scope),
        )


class AsyncMyAssignmentsService(AsyncBaseService):
    async def get_my_assignments(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="myassignments", operation="get_my_assignments", is_mutation=False),
            "GET",
            "/my/assignments.json",
        )

    async def get_my_completed_assignments(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="myassignments", operation="get_my_completed_assignments", is_mutation=False),
            "GET",
            "/my/assignments/completed.json",
        )

    async def get_my_due_assignments(self, *, scope: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="myassignments", operation="get_my_due_assignments", is_mutation=False),
            "GET",
            "/my/assignments/due.json",
            params=self._compact(scope=scope),
        )
