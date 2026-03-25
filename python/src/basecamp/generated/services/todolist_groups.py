# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TodolistGroupsService(BaseService):
    def reposition(self, *, group_id: int, position: int) -> None:
        self._request_void(
            OperationInfo(service="todolistgroups", operation="reposition", is_mutation=True, resource_id=group_id),
            "PUT",
            f"/todolists/{group_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTodolistGroup",
        )

    def list(self, *, todolist_id: int) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="todolistgroups", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/groups.json",
        )

    def create(self, *, todolist_id: int, name: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="todolistgroups", operation="create", is_mutation=True, resource_id=todolist_id),
            "POST",
            f"/todolists/{todolist_id}/groups.json",
            json_body=self._compact(name=name),
            operation="CreateTodolistGroup",
        )


class AsyncTodolistGroupsService(AsyncBaseService):
    async def reposition(self, *, group_id: int, position: int) -> None:
        await self._request_void(
            OperationInfo(service="todolistgroups", operation="reposition", is_mutation=True, resource_id=group_id),
            "PUT",
            f"/todolists/{group_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTodolistGroup",
        )

    async def list(self, *, todolist_id: int) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="todolistgroups", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/groups.json",
        )

    async def create(self, *, todolist_id: int, name: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="todolistgroups", operation="create", is_mutation=True, resource_id=todolist_id),
            "POST",
            f"/todolists/{todolist_id}/groups.json",
            json_body=self._compact(name=name),
            operation="CreateTodolistGroup",
        )
