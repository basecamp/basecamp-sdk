# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class BubbleUpsService(BaseService):
    def create_bubble_up(self, *, recording_id: int, at: str | None = None) -> None:
        """Bubble up a recording for the current user, resurfacing it in the current
        user's readings (the BC5 successor to "save"). Returns 204 No Content with
        no body.

        The `at` field controls timing. Send `"now"` to bubble up immediately, or a
        scheduling keyword (`"today"`, `"tomorrow"`, `"weekend"`, `"next_week"`) or
        an ISO8601 date (e.g. `"2026-09-10"`) to schedule it to resurface later.
        NOTE: bc3 currently requires `at` — omitting it raises on the server
        (`Date.iso8601(nil)`), so send `"now"` for the immediate case. The field is
        modeled optional (not `@required`) so a future bc3 default (`params[:at] ||=
        "now"`) makes omission mean "now" without an SDK change.

        Idempotent: bubbling up an already-bubbled recording is set-membership and
        still returns 204.

        Args:
            recording_id: The recording id.
            at: Timing for the bubble-up. `"now"` bubbles up immediately; a scheduling keyword
                (`"today"`, `"tomorrow"`, `"weekend"`, `"next_week"`) or an ISO8601 date (e.g.
                `"2026-09-10"`) schedules it to resurface later. bc3 requires a value — omitting
                `at` errors server-side (`Date.iso8601(nil)`) — so send `"now"` for the immediate
                case.
        """
        self._request_void(
            OperationInfo(
                service="bubbleups", operation="create_bubble_up", is_mutation=True, resource_id=recording_id
            ),
            "POST",
            f"/recordings/{recording_id}/bubble_up.json",
            json_body=self._compact(at=at),
            operation="CreateBubbleUp",
        )

    def delete_bubble_up(self, *, recording_id: int) -> None:
        """Remove the current user's bubble-up from a recording (returns 204 No Content).
        Idempotent: popping an absent bubble-up also returns 204.

        Args:
            recording_id: The recording id.
        """
        self._request_void(
            OperationInfo(
                service="bubbleups", operation="delete_bubble_up", is_mutation=True, resource_id=recording_id
            ),
            "DELETE",
            f"/recordings/{recording_id}/bubble_up.json",
            operation="DeleteBubbleUp",
        )


class AsyncBubbleUpsService(AsyncBaseService):
    async def create_bubble_up(self, *, recording_id: int, at: str | None = None) -> None:
        """Bubble up a recording for the current user, resurfacing it in the current
        user's readings (the BC5 successor to "save"). Returns 204 No Content with
        no body.

        The `at` field controls timing. Send `"now"` to bubble up immediately, or a
        scheduling keyword (`"today"`, `"tomorrow"`, `"weekend"`, `"next_week"`) or
        an ISO8601 date (e.g. `"2026-09-10"`) to schedule it to resurface later.
        NOTE: bc3 currently requires `at` — omitting it raises on the server
        (`Date.iso8601(nil)`), so send `"now"` for the immediate case. The field is
        modeled optional (not `@required`) so a future bc3 default (`params[:at] ||=
        "now"`) makes omission mean "now" without an SDK change.

        Idempotent: bubbling up an already-bubbled recording is set-membership and
        still returns 204.

        Args:
            recording_id: The recording id.
            at: Timing for the bubble-up. `"now"` bubbles up immediately; a scheduling keyword
                (`"today"`, `"tomorrow"`, `"weekend"`, `"next_week"`) or an ISO8601 date (e.g.
                `"2026-09-10"`) schedules it to resurface later. bc3 requires a value — omitting
                `at` errors server-side (`Date.iso8601(nil)`) — so send `"now"` for the immediate
                case.
        """
        await self._request_void(
            OperationInfo(
                service="bubbleups", operation="create_bubble_up", is_mutation=True, resource_id=recording_id
            ),
            "POST",
            f"/recordings/{recording_id}/bubble_up.json",
            json_body=self._compact(at=at),
            operation="CreateBubbleUp",
        )

    async def delete_bubble_up(self, *, recording_id: int) -> None:
        """Remove the current user's bubble-up from a recording (returns 204 No Content).
        Idempotent: popping an absent bubble-up also returns 204.

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(
                service="bubbleups", operation="delete_bubble_up", is_mutation=True, resource_id=recording_id
            ),
            "DELETE",
            f"/recordings/{recording_id}/bubble_up.json",
            operation="DeleteBubbleUp",
        )
