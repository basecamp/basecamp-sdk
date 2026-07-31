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
            operation="GetMyAssignments",
        )

    def get_my_completed_assignments(self) -> ListResult:
        return self._request_list(
            OperationInfo(service="myassignments", operation="get_my_completed_assignments", is_mutation=False),
            "/my/assignments/completed.json",
            operation="GetMyCompletedAssignments",
        )

    def get_my_due_assignments(self, *, scope: str | None = None) -> ListResult:
        return self._request_list(
            OperationInfo(service="myassignments", operation="get_my_due_assignments", is_mutation=False),
            "/my/assignments/due.json",
            params=self._compact(scope=scope),
            operation="GetMyDueAssignments",
        )

    def prioritize_assignment(self, *, id: int) -> None:
        self._request_void(
            OperationInfo(service="myassignments", operation="prioritize_assignment", is_mutation=True),
            "POST",
            "/my/priorities.json",
            json_body=self._compact(id=id),
            operation="PrioritizeAssignment",
        )

    def deprioritize_assignment(self, *, recording_id: int) -> None:
        self._request_void(
            OperationInfo(
                service="myassignments", operation="deprioritize_assignment", is_mutation=True, resource_id=recording_id
            ),
            "DELETE",
            f"/my/priorities/{recording_id}",
            operation="DeprioritizeAssignment",
        )

    def reorder_up_next(self, *, source_id: int, position: int) -> None:
        self._request_void(
            OperationInfo(service="myassignments", operation="reorder_up_next", is_mutation=True),
            "POST",
            "/my/priority_moves.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="ReorderUpNext",
        )


class AsyncMyAssignmentsService(AsyncBaseService):
    async def get_my_assignments(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="myassignments", operation="get_my_assignments", is_mutation=False),
            "GET",
            "/my/assignments.json",
            operation="GetMyAssignments",
        )

    async def get_my_completed_assignments(self) -> ListResult:
        return await self._request_list(
            OperationInfo(service="myassignments", operation="get_my_completed_assignments", is_mutation=False),
            "/my/assignments/completed.json",
            operation="GetMyCompletedAssignments",
        )

    async def get_my_due_assignments(self, *, scope: str | None = None) -> ListResult:
        return await self._request_list(
            OperationInfo(service="myassignments", operation="get_my_due_assignments", is_mutation=False),
            "/my/assignments/due.json",
            params=self._compact(scope=scope),
            operation="GetMyDueAssignments",
        )

    async def prioritize_assignment(self, *, id: int) -> None:
        await self._request_void(
            OperationInfo(service="myassignments", operation="prioritize_assignment", is_mutation=True),
            "POST",
            "/my/priorities.json",
            json_body=self._compact(id=id),
            operation="PrioritizeAssignment",
        )

    async def deprioritize_assignment(self, *, recording_id: int) -> None:
        await self._request_void(
            OperationInfo(
                service="myassignments", operation="deprioritize_assignment", is_mutation=True, resource_id=recording_id
            ),
            "DELETE",
            f"/my/priorities/{recording_id}",
            operation="DeprioritizeAssignment",
        )

    async def reorder_up_next(self, *, source_id: int, position: int) -> None:
        await self._request_void(
            OperationInfo(service="myassignments", operation="reorder_up_next", is_mutation=True),
            "POST",
            "/my/priority_moves.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="ReorderUpNext",
        )
