# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TodosService(BaseService):
    def list(self, *, todolist_id: int | str, status: str | None = None, completed: bool | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="todos", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/todos.json",
            params=self._compact(status=status, completed=completed),
        )

    def create(
        self,
        *,
        todolist_id: int | str,
        content: str,
        description: str | None = None,
        assignee_ids: list | None = None,
        completion_subscriber_ids: list | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="todos", operation="create", is_mutation=True, resource_id=todolist_id),
            "POST",
            f"/todolists/{todolist_id}/todos.json",
            json_body=self._compact(
                content=content,
                description=description,
                assignee_ids=assignee_ids,
                completion_subscriber_ids=completion_subscriber_ids,
                notify=notify,
                due_on=due_on,
                starts_on=starts_on,
            ),
            operation="CreateTodo",
        )

    def get(self, *, todo_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="todos", operation="get", is_mutation=False, resource_id=todo_id),
            "GET",
            f"/todos/{todo_id}",
        )

    def update(
        self,
        *,
        todo_id: int | str,
        content: str | None = None,
        description: str | None = None,
        assignee_ids: list | None = None,
        completion_subscriber_ids: list | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="todos", operation="update", is_mutation=True, resource_id=todo_id),
            "PUT",
            f"/todos/{todo_id}",
            json_body=self._compact(
                content=content,
                description=description,
                assignee_ids=assignee_ids,
                completion_subscriber_ids=completion_subscriber_ids,
                notify=notify,
                due_on=due_on,
                starts_on=starts_on,
            ),
            operation="UpdateTodo",
        )

    def trash(self, *, todo_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="todos", operation="trash", is_mutation=True, resource_id=todo_id),
            "DELETE",
            f"/todos/{todo_id}",
            operation="TrashTodo",
        )

    def complete(self, *, todo_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="todos", operation="complete", is_mutation=True, resource_id=todo_id),
            "POST",
            f"/todos/{todo_id}/completion.json",
            operation="CompleteTodo",
        )

    def uncomplete(self, *, todo_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="todos", operation="uncomplete", is_mutation=True, resource_id=todo_id),
            "DELETE",
            f"/todos/{todo_id}/completion.json",
            operation="UncompleteTodo",
        )

    def reposition(self, *, todo_id: int | str, position: int, parent_id: int | None = None) -> None:
        self._request_void(
            OperationInfo(service="todos", operation="reposition", is_mutation=True, resource_id=todo_id),
            "PUT",
            f"/todos/{todo_id}/position.json",
            json_body=self._compact(position=position, parent_id=parent_id),
            operation="RepositionTodo",
        )


class AsyncTodosService(AsyncBaseService):
    async def list(
        self, *, todolist_id: int | str, status: str | None = None, completed: bool | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="todos", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/todos.json",
            params=self._compact(status=status, completed=completed),
        )

    async def create(
        self,
        *,
        todolist_id: int | str,
        content: str,
        description: str | None = None,
        assignee_ids: list | None = None,
        completion_subscriber_ids: list | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="todos", operation="create", is_mutation=True, resource_id=todolist_id),
            "POST",
            f"/todolists/{todolist_id}/todos.json",
            json_body=self._compact(
                content=content,
                description=description,
                assignee_ids=assignee_ids,
                completion_subscriber_ids=completion_subscriber_ids,
                notify=notify,
                due_on=due_on,
                starts_on=starts_on,
            ),
            operation="CreateTodo",
        )

    async def get(self, *, todo_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="todos", operation="get", is_mutation=False, resource_id=todo_id),
            "GET",
            f"/todos/{todo_id}",
        )

    async def update(
        self,
        *,
        todo_id: int | str,
        content: str | None = None,
        description: str | None = None,
        assignee_ids: list | None = None,
        completion_subscriber_ids: list | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="todos", operation="update", is_mutation=True, resource_id=todo_id),
            "PUT",
            f"/todos/{todo_id}",
            json_body=self._compact(
                content=content,
                description=description,
                assignee_ids=assignee_ids,
                completion_subscriber_ids=completion_subscriber_ids,
                notify=notify,
                due_on=due_on,
                starts_on=starts_on,
            ),
            operation="UpdateTodo",
        )

    async def trash(self, *, todo_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="todos", operation="trash", is_mutation=True, resource_id=todo_id),
            "DELETE",
            f"/todos/{todo_id}",
            operation="TrashTodo",
        )

    async def complete(self, *, todo_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="todos", operation="complete", is_mutation=True, resource_id=todo_id),
            "POST",
            f"/todos/{todo_id}/completion.json",
            operation="CompleteTodo",
        )

    async def uncomplete(self, *, todo_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="todos", operation="uncomplete", is_mutation=True, resource_id=todo_id),
            "DELETE",
            f"/todos/{todo_id}/completion.json",
            operation="UncompleteTodo",
        )

    async def reposition(self, *, todo_id: int | str, position: int, parent_id: int | None = None) -> None:
        await self._request_void(
            OperationInfo(service="todos", operation="reposition", is_mutation=True, resource_id=todo_id),
            "PUT",
            f"/todos/{todo_id}/position.json",
            json_body=self._compact(position=position, parent_id=parent_id),
            operation="RepositionTodo",
        )
