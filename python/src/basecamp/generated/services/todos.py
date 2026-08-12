# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TodosService(BaseService):
    def create_todoset_todo(
        self,
        *,
        bucket_id: int,
        todoset_id: int,
        content: str,
        description: str | None = None,
        assignee_ids: list[int] | None = None,
        completion_subscriber_ids: list[int] | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Create a to-do directly under a project's to-do set, outside any to-do list.
        This form exists only project-scoped (no account-scoped variant); parameters
        and response match the to-do-list create. Find a project's to-do set id via
        GetTodoset.

        Args:
            bucket_id: The bucket id.
            todoset_id: The todoset id.
            content: The content.
            description: The description.
            assignee_ids: The assignee ids.
            completion_subscriber_ids: The completion subscriber ids.
            notify: The notify.
            due_on: The due on.
            starts_on: The starts on.
        """
        return self._request(
            OperationInfo(
                service="todos",
                operation="create_todoset_todo",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=todoset_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/todosets/{todoset_id}/todos.json",
            json_body=self._compact(
                content=content,
                description=description,
                assignee_ids=assignee_ids,
                completion_subscriber_ids=completion_subscriber_ids,
                notify=notify,
                due_on=due_on,
                starts_on=starts_on,
            ),
            operation="CreateTodosetTodo",
        )

    def list(
        self,
        *,
        todolist_id: int,
        status: str | None = None,
        completed: bool | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List todos in a todolist.

        Args:
            todolist_id: The todolist id.
            status: active|archived|trashed
            completed: The completed.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="todos", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/todos.json",
            params=self._compact(status=status, completed=completed, page=page),
            max_items=max_items,
            operation="ListTodos",
        )

    def create(
        self,
        *,
        todolist_id: int,
        content: str,
        description: str | None = None,
        assignee_ids: list[int] | None = None,
        completion_subscriber_ids: list[int] | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Create a new todo in a todolist.

        Args:
            todolist_id: The todolist id.
            content: The content.
            description: The description.
            assignee_ids: The assignee ids.
            completion_subscriber_ids: The completion subscriber ids.
            notify: The notify.
            due_on: The due on.
            starts_on: The starts on.
        """
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

    def get(self, *, todo_id: int) -> dict[str, Any]:
        """Get a single todo by id.

        Args:
            todo_id: The todo id.
        """
        return self._request(
            OperationInfo(service="todos", operation="get", is_mutation=False, resource_id=todo_id),
            "GET",
            f"/todos/{todo_id}",
            operation="GetTodo",
        )

    def replace(
        self,
        *,
        todo_id: int,
        content: str,
        description: str | None = None,
        assignee_ids: list[int] | None = None,
        completion_subscriber_ids: list[int] | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Replace a todo with a new complete representation.
        The request body is the todo's full writable state: any writable field
        omitted from the request is cleared server-side (empty/missing
        assignee_ids clears assignees, missing description clears it, and so
        on). content is required — a request without it is rejected.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current todo and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            todo_id: The todo id.
            content: The content.
            description: The description.
            assignee_ids: The assignee ids.
            completion_subscriber_ids: The completion subscriber ids.
            notify: The notify.
            due_on: The due on.
            starts_on: The starts on.
        """
        return self._request(
            OperationInfo(service="todos", operation="replace", is_mutation=True, resource_id=todo_id),
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
            operation="ReplaceTodo",
        )

    def complete(self, *, todo_id: int) -> None:
        """Mark a todo as complete.

        Args:
            todo_id: The todo id.
        """
        self._request_void(
            OperationInfo(service="todos", operation="complete", is_mutation=True, resource_id=todo_id),
            "POST",
            f"/todos/{todo_id}/completion.json",
            operation="CompleteTodo",
        )

    def uncomplete(self, *, todo_id: int) -> None:
        """Mark a todo as incomplete.

        Args:
            todo_id: The todo id.
        """
        self._request_void(
            OperationInfo(service="todos", operation="uncomplete", is_mutation=True, resource_id=todo_id),
            "DELETE",
            f"/todos/{todo_id}/completion.json",
            operation="UncompleteTodo",
        )

    def reposition(self, *, todo_id: int, position: int, parent_id: int | None = None) -> None:
        """Reposition a todo within its todolist.

        Args:
            todo_id: The todo id.
            position: The position.
            parent_id: Optional todolist ID to move the todo to a different parent
        """
        self._request_void(
            OperationInfo(service="todos", operation="reposition", is_mutation=True, resource_id=todo_id),
            "PUT",
            f"/todos/{todo_id}/position.json",
            json_body=self._compact(position=position, parent_id=parent_id),
            operation="RepositionTodo",
        )


class AsyncTodosService(AsyncBaseService):
    async def create_todoset_todo(
        self,
        *,
        bucket_id: int,
        todoset_id: int,
        content: str,
        description: str | None = None,
        assignee_ids: list[int] | None = None,
        completion_subscriber_ids: list[int] | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Create a to-do directly under a project's to-do set, outside any to-do list.
        This form exists only project-scoped (no account-scoped variant); parameters
        and response match the to-do-list create. Find a project's to-do set id via
        GetTodoset.

        Args:
            bucket_id: The bucket id.
            todoset_id: The todoset id.
            content: The content.
            description: The description.
            assignee_ids: The assignee ids.
            completion_subscriber_ids: The completion subscriber ids.
            notify: The notify.
            due_on: The due on.
            starts_on: The starts on.
        """
        return await self._request(
            OperationInfo(
                service="todos",
                operation="create_todoset_todo",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=todoset_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/todosets/{todoset_id}/todos.json",
            json_body=self._compact(
                content=content,
                description=description,
                assignee_ids=assignee_ids,
                completion_subscriber_ids=completion_subscriber_ids,
                notify=notify,
                due_on=due_on,
                starts_on=starts_on,
            ),
            operation="CreateTodosetTodo",
        )

    async def list(
        self,
        *,
        todolist_id: int,
        status: str | None = None,
        completed: bool | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List todos in a todolist.

        Args:
            todolist_id: The todolist id.
            status: active|archived|trashed
            completed: The completed.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="todos", operation="list", is_mutation=False, resource_id=todolist_id),
            f"/todolists/{todolist_id}/todos.json",
            params=self._compact(status=status, completed=completed, page=page),
            max_items=max_items,
            operation="ListTodos",
        )

    async def create(
        self,
        *,
        todolist_id: int,
        content: str,
        description: str | None = None,
        assignee_ids: list[int] | None = None,
        completion_subscriber_ids: list[int] | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Create a new todo in a todolist.

        Args:
            todolist_id: The todolist id.
            content: The content.
            description: The description.
            assignee_ids: The assignee ids.
            completion_subscriber_ids: The completion subscriber ids.
            notify: The notify.
            due_on: The due on.
            starts_on: The starts on.
        """
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

    async def get(self, *, todo_id: int) -> dict[str, Any]:
        """Get a single todo by id.

        Args:
            todo_id: The todo id.
        """
        return await self._request(
            OperationInfo(service="todos", operation="get", is_mutation=False, resource_id=todo_id),
            "GET",
            f"/todos/{todo_id}",
            operation="GetTodo",
        )

    async def replace(
        self,
        *,
        todo_id: int,
        content: str,
        description: str | None = None,
        assignee_ids: list[int] | None = None,
        completion_subscriber_ids: list[int] | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Replace a todo with a new complete representation.
        The request body is the todo's full writable state: any writable field
        omitted from the request is cleared server-side (empty/missing
        assignee_ids clears assignees, missing description clears it, and so
        on). content is required — a request without it is rejected.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current todo and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            todo_id: The todo id.
            content: The content.
            description: The description.
            assignee_ids: The assignee ids.
            completion_subscriber_ids: The completion subscriber ids.
            notify: The notify.
            due_on: The due on.
            starts_on: The starts on.
        """
        return await self._request(
            OperationInfo(service="todos", operation="replace", is_mutation=True, resource_id=todo_id),
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
            operation="ReplaceTodo",
        )

    async def complete(self, *, todo_id: int) -> None:
        """Mark a todo as complete.

        Args:
            todo_id: The todo id.
        """
        await self._request_void(
            OperationInfo(service="todos", operation="complete", is_mutation=True, resource_id=todo_id),
            "POST",
            f"/todos/{todo_id}/completion.json",
            operation="CompleteTodo",
        )

    async def uncomplete(self, *, todo_id: int) -> None:
        """Mark a todo as incomplete.

        Args:
            todo_id: The todo id.
        """
        await self._request_void(
            OperationInfo(service="todos", operation="uncomplete", is_mutation=True, resource_id=todo_id),
            "DELETE",
            f"/todos/{todo_id}/completion.json",
            operation="UncompleteTodo",
        )

    async def reposition(self, *, todo_id: int, position: int, parent_id: int | None = None) -> None:
        """Reposition a todo within its todolist.

        Args:
            todo_id: The todo id.
            position: The position.
            parent_id: Optional todolist ID to move the todo to a different parent
        """
        await self._request_void(
            OperationInfo(service="todos", operation="reposition", is_mutation=True, resource_id=todo_id),
            "PUT",
            f"/todos/{todo_id}/position.json",
            json_body=self._compact(position=position, parent_id=parent_id),
            operation="RepositionTodo",
        )
