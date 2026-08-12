# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TodolistGroupsService(BaseService):
    def reposition(self, *, group_id: int, position: int) -> None:
        """Reposition a todolist group.

        Args:
            group_id: The group id.
            position: The position.
        """
        self._request_void(
            OperationInfo(service="todolistgroups", operation="reposition", is_mutation=True, resource_id=group_id),
            "PUT",
            f"/todolists/groups/{group_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTodolistGroup",
        )

    def list(self, *, todolist_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List groups in a todolist.

        Args:
            todolist_id: The todolist id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="todolistgroups", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/groups.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListTodolistGroups",
        )

    def create(self, *, todolist_id: int, name: str) -> dict[str, Any]:
        """Create a new group in a todolist.

        Args:
            todolist_id: The todolist id.
            name: The name.
        """
        return self._request(
            OperationInfo(service="todolistgroups", operation="create", is_mutation=True, resource_id=todolist_id),
            "POST",
            f"/todolists/{todolist_id}/groups.json",
            json_body=self._compact(name=name),
            operation="CreateTodolistGroup",
        )


class AsyncTodolistGroupsService(AsyncBaseService):
    async def reposition(self, *, group_id: int, position: int) -> None:
        """Reposition a todolist group.

        Args:
            group_id: The group id.
            position: The position.
        """
        await self._request_void(
            OperationInfo(service="todolistgroups", operation="reposition", is_mutation=True, resource_id=group_id),
            "PUT",
            f"/todolists/groups/{group_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTodolistGroup",
        )

    async def list(self, *, todolist_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List groups in a todolist.

        Args:
            todolist_id: The todolist id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="todolistgroups", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/groups.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListTodolistGroups",
        )

    async def create(self, *, todolist_id: int, name: str) -> dict[str, Any]:
        """Create a new group in a todolist.

        Args:
            todolist_id: The todolist id.
            name: The name.
        """
        return await self._request(
            OperationInfo(service="todolistgroups", operation="create", is_mutation=True, resource_id=todolist_id),
            "POST",
            f"/todolists/{todolist_id}/groups.json",
            json_body=self._compact(name=name),
            operation="CreateTodolistGroup",
        )
