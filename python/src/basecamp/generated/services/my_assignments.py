# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MyAssignmentsService(BaseService):
    def get_my_assignments(self) -> dict[str, Any]:
        """Get the current user's active assignments grouped into priorities and non_priorities.
        Card table steps are normalized to their parent card with steps as children.
        This endpoint is not paginated.
        """
        return self._request(
            OperationInfo(service="myassignments", operation="get_my_assignments", is_mutation=False),
            "GET",
            "/my/assignments.json",
            operation="GetMyAssignments",
        )

    def get_my_completed_assignments(self) -> ListResult:
        """Get the current user's completed assignments.
        Archived and trashed recordings are excluded. This endpoint is not paginated.
        """
        return self._request_list(
            OperationInfo(service="myassignments", operation="get_my_completed_assignments", is_mutation=False),
            "/my/assignments/completed.json",
            operation="GetMyCompletedAssignments",
        )

    def get_my_due_assignments(self, *, scope: str | None = None) -> ListResult:
        """Get the current user's assignments filtered by due date scope.
        Defaults to overdue when no scope is provided. This endpoint is not paginated.

        Args:
            scope: Filter by due date range: overdue, due_today, due_tomorrow, due_later_this_week,
                due_next_week, due_later
        """
        return self._request_list(
            OperationInfo(service="myassignments", operation="get_my_due_assignments", is_mutation=False),
            "/my/assignments/due.json",
            params=self._compact(scope=scope),
            operation="GetMyDueAssignments",
        )

    def prioritize_assignment(self, *, id: int) -> None:
        """Add a recording to Up Next — the current user's ordered list of prioritized
        assignments (the priorities returned by GetMyAssignments). Identify the item
        by the recording id that carries the priority; for a card table step
        surfaced under its parent card, that is the entry's priority_recording_id.
        Idempotent: re-prioritizing an already-prioritized recording is a no-op.

        Args:
            id: The recording id to prioritize.
        """
        self._request_void(
            OperationInfo(service="myassignments", operation="prioritize_assignment", is_mutation=True),
            "POST",
            "/my/priorities.json",
            json_body=self._compact(id=id),
            operation="PrioritizeAssignment",
        )

    def deprioritize_assignment(self, *, recording_id: int) -> None:
        """Remove a recording from Up Next (returns 204 No Content). Exact-target:
        only the priority carried by the identified recording is cleared, and
        deleting an absent priority is a no-op 204 — so the DELETE is idempotent
        and safe to retry (BC3 #12483). Address a surfaced card table step by its
        priority_recording_id, not its parent card's id.

        Args:
            recording_id: The recording id.
        """
        self._request_void(
            OperationInfo(
                service="myassignments", operation="deprioritize_assignment", is_mutation=True, resource_id=recording_id
            ),
            "DELETE",
            f"/my/priorities/{recording_id}",
            operation="DeprioritizeAssignment",
        )

    def reorder_up_next(self, *, source_id: int, position: int) -> None:
        """Move an already-prioritized recording to a new 1-based position in Up Next
        (returns 204 No Content). NOT idempotent: a positional move's meaning
        shifts as the list changes, so a retry can land the item somewhere else —
        no retry gating is declared. Errors: 400 for a missing or non-integer
        position, 422 (flat {error} body) for an out-of-range position or an
        unprioritized recording, and a bare bodyless 404 for an inaccessible
        recording.

        Args:
            source_id: The recording id to move, chosen the same way as when prioritizing.
            position: The 1-based position to move it to.
        """
        self._request_void(
            OperationInfo(service="myassignments", operation="reorder_up_next", is_mutation=True),
            "POST",
            "/my/priority_moves.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="ReorderUpNext",
        )


class AsyncMyAssignmentsService(AsyncBaseService):
    async def get_my_assignments(self) -> dict[str, Any]:
        """Get the current user's active assignments grouped into priorities and non_priorities.
        Card table steps are normalized to their parent card with steps as children.
        This endpoint is not paginated.
        """
        return await self._request(
            OperationInfo(service="myassignments", operation="get_my_assignments", is_mutation=False),
            "GET",
            "/my/assignments.json",
            operation="GetMyAssignments",
        )

    async def get_my_completed_assignments(self) -> ListResult:
        """Get the current user's completed assignments.
        Archived and trashed recordings are excluded. This endpoint is not paginated.
        """
        return await self._request_list(
            OperationInfo(service="myassignments", operation="get_my_completed_assignments", is_mutation=False),
            "/my/assignments/completed.json",
            operation="GetMyCompletedAssignments",
        )

    async def get_my_due_assignments(self, *, scope: str | None = None) -> ListResult:
        """Get the current user's assignments filtered by due date scope.
        Defaults to overdue when no scope is provided. This endpoint is not paginated.

        Args:
            scope: Filter by due date range: overdue, due_today, due_tomorrow, due_later_this_week,
                due_next_week, due_later
        """
        return await self._request_list(
            OperationInfo(service="myassignments", operation="get_my_due_assignments", is_mutation=False),
            "/my/assignments/due.json",
            params=self._compact(scope=scope),
            operation="GetMyDueAssignments",
        )

    async def prioritize_assignment(self, *, id: int) -> None:
        """Add a recording to Up Next — the current user's ordered list of prioritized
        assignments (the priorities returned by GetMyAssignments). Identify the item
        by the recording id that carries the priority; for a card table step
        surfaced under its parent card, that is the entry's priority_recording_id.
        Idempotent: re-prioritizing an already-prioritized recording is a no-op.

        Args:
            id: The recording id to prioritize.
        """
        await self._request_void(
            OperationInfo(service="myassignments", operation="prioritize_assignment", is_mutation=True),
            "POST",
            "/my/priorities.json",
            json_body=self._compact(id=id),
            operation="PrioritizeAssignment",
        )

    async def deprioritize_assignment(self, *, recording_id: int) -> None:
        """Remove a recording from Up Next (returns 204 No Content). Exact-target:
        only the priority carried by the identified recording is cleared, and
        deleting an absent priority is a no-op 204 — so the DELETE is idempotent
        and safe to retry (BC3 #12483). Address a surfaced card table step by its
        priority_recording_id, not its parent card's id.

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(
                service="myassignments", operation="deprioritize_assignment", is_mutation=True, resource_id=recording_id
            ),
            "DELETE",
            f"/my/priorities/{recording_id}",
            operation="DeprioritizeAssignment",
        )

    async def reorder_up_next(self, *, source_id: int, position: int) -> None:
        """Move an already-prioritized recording to a new 1-based position in Up Next
        (returns 204 No Content). NOT idempotent: a positional move's meaning
        shifts as the list changes, so a retry can land the item somewhere else —
        no retry gating is declared. Errors: 400 for a missing or non-integer
        position, 422 (flat {error} body) for an out-of-range position or an
        unprioritized recording, and a bare bodyless 404 for an inaccessible
        recording.

        Args:
            source_id: The recording id to move, chosen the same way as when prioritizing.
            position: The 1-based position to move it to.
        """
        await self._request_void(
            OperationInfo(service="myassignments", operation="reorder_up_next", is_mutation=True),
            "POST",
            "/my/priority_moves.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="ReorderUpNext",
        )
