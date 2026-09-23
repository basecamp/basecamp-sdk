# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class SubtasksService(BaseService):
    def list(self, *, recording_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List a recording's subtasks, in position order.

        Only to-dos and cards hold subtasks; check for `subtasks_count` and
        `subtasks_url` on the parent's JSON.

        Args:
            recording_id: The recording id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="subtasks", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/subtasks.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListSubtasks",
        )

    def create(
        self, *, recording_id: int, title: str, due_on: str | None = None, assignee_ids: list[int] | None = None
    ) -> dict[str, Any]:
        """Create a subtask under a to-do or a card.

        Any other recording answers `403 Forbidden`.

        Args:
            recording_id: The recording id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return self._request(
            OperationInfo(service="subtasks", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/subtasks.json",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="CreateSubtask",
        )

    def get(self, *, subtask_id: int) -> dict[str, Any]:
        """Get a subtask by ID.

        Args:
            subtask_id: The subtask id.
        """
        return self._request(
            OperationInfo(service="subtasks", operation="get", is_mutation=False, resource_id=subtask_id),
            "GET",
            f"/subtasks/{subtask_id}",
            operation="GetSubtask",
        )

    def update(
        self,
        *,
        subtask_id: int,
        title: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Update a subtask.

        A partial update: every omitted parameter is left unchanged. Clearing a
        value takes an explicit send — `"due_on": null` clears the due date (an
        empty string is accepted too, and is what the Ruby, Python and TypeScript
        SDKs send, since they drop nil, None and undefined from the body);
        `"assignee_ids": []` removes every assignee.

        Args:
            subtask_id: The subtask id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return self._request(
            OperationInfo(service="subtasks", operation="update", is_mutation=True, resource_id=subtask_id),
            "PUT",
            f"/subtasks/{subtask_id}",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateSubtask",
        )

    def delete(self, *, subtask_id: int) -> None:
        """Delete a subtask.

        On accounts where deleting is limited to admins and the creator, everyone
        else gets `403 Forbidden`.

        Args:
            subtask_id: The subtask id.
        """
        self._request_void(
            OperationInfo(service="subtasks", operation="delete", is_mutation=True, resource_id=subtask_id),
            "DELETE",
            f"/subtasks/{subtask_id}",
            operation="DeleteSubtask",
        )

    def complete(self, *, subtask_id: int) -> None:
        """Mark a subtask as completed.

        Args:
            subtask_id: The subtask id.
        """
        self._request_void(
            OperationInfo(service="subtasks", operation="complete", is_mutation=True, resource_id=subtask_id),
            "POST",
            f"/subtasks/{subtask_id}/completion.json",
            operation="CompleteSubtask",
        )

    def uncomplete(self, *, subtask_id: int) -> None:
        """Mark a subtask as not completed.

        Args:
            subtask_id: The subtask id.
        """
        self._request_void(
            OperationInfo(service="subtasks", operation="uncomplete", is_mutation=True, resource_id=subtask_id),
            "DELETE",
            f"/subtasks/{subtask_id}/completion.json",
            operation="UncompleteSubtask",
        )

    def reposition(self, *, subtask_id: int, position: int) -> None:
        """Move a subtask to a new position among its siblings.

        Args:
            subtask_id: The subtask id.
            position: The 1-based position to move it to
        """
        self._request_void(
            OperationInfo(service="subtasks", operation="reposition", is_mutation=True, resource_id=subtask_id),
            "PUT",
            f"/subtasks/{subtask_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionSubtask",
        )


class AsyncSubtasksService(AsyncBaseService):
    async def list(self, *, recording_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List a recording's subtasks, in position order.

        Only to-dos and cards hold subtasks; check for `subtasks_count` and
        `subtasks_url` on the parent's JSON.

        Args:
            recording_id: The recording id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="subtasks", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/subtasks.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListSubtasks",
        )

    async def create(
        self, *, recording_id: int, title: str, due_on: str | None = None, assignee_ids: list[int] | None = None
    ) -> dict[str, Any]:
        """Create a subtask under a to-do or a card.

        Any other recording answers `403 Forbidden`.

        Args:
            recording_id: The recording id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return await self._request(
            OperationInfo(service="subtasks", operation="create", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/subtasks.json",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="CreateSubtask",
        )

    async def get(self, *, subtask_id: int) -> dict[str, Any]:
        """Get a subtask by ID.

        Args:
            subtask_id: The subtask id.
        """
        return await self._request(
            OperationInfo(service="subtasks", operation="get", is_mutation=False, resource_id=subtask_id),
            "GET",
            f"/subtasks/{subtask_id}",
            operation="GetSubtask",
        )

    async def update(
        self,
        *,
        subtask_id: int,
        title: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Update a subtask.

        A partial update: every omitted parameter is left unchanged. Clearing a
        value takes an explicit send — `"due_on": null` clears the due date (an
        empty string is accepted too, and is what the Ruby, Python and TypeScript
        SDKs send, since they drop nil, None and undefined from the body);
        `"assignee_ids": []` removes every assignee.

        Args:
            subtask_id: The subtask id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return await self._request(
            OperationInfo(service="subtasks", operation="update", is_mutation=True, resource_id=subtask_id),
            "PUT",
            f"/subtasks/{subtask_id}",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateSubtask",
        )

    async def delete(self, *, subtask_id: int) -> None:
        """Delete a subtask.

        On accounts where deleting is limited to admins and the creator, everyone
        else gets `403 Forbidden`.

        Args:
            subtask_id: The subtask id.
        """
        await self._request_void(
            OperationInfo(service="subtasks", operation="delete", is_mutation=True, resource_id=subtask_id),
            "DELETE",
            f"/subtasks/{subtask_id}",
            operation="DeleteSubtask",
        )

    async def complete(self, *, subtask_id: int) -> None:
        """Mark a subtask as completed.

        Args:
            subtask_id: The subtask id.
        """
        await self._request_void(
            OperationInfo(service="subtasks", operation="complete", is_mutation=True, resource_id=subtask_id),
            "POST",
            f"/subtasks/{subtask_id}/completion.json",
            operation="CompleteSubtask",
        )

    async def uncomplete(self, *, subtask_id: int) -> None:
        """Mark a subtask as not completed.

        Args:
            subtask_id: The subtask id.
        """
        await self._request_void(
            OperationInfo(service="subtasks", operation="uncomplete", is_mutation=True, resource_id=subtask_id),
            "DELETE",
            f"/subtasks/{subtask_id}/completion.json",
            operation="UncompleteSubtask",
        )

    async def reposition(self, *, subtask_id: int, position: int) -> None:
        """Move a subtask to a new position among its siblings.

        Args:
            subtask_id: The subtask id.
            position: The 1-based position to move it to
        """
        await self._request_void(
            OperationInfo(service="subtasks", operation="reposition", is_mutation=True, resource_id=subtask_id),
            "PUT",
            f"/subtasks/{subtask_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionSubtask",
        )
