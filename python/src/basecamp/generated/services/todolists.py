# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TodolistsService(BaseService):
    def get(self, *, id: int) -> dict[str, Any]:
        """Get a single todolist or todolist group by id
        The endpoint is polymorphic, but it answers with one shape: BC3 has no group
        model, so both variants come back as a flat Todolist (see the Todolist shape).

        Args:
            id: The id.
        """
        return self._request(
            OperationInfo(service="todolists", operation="get", is_mutation=False, resource_id=id),
            "GET",
            f"/todolists/{id}",
            operation="GetTodolistOrGroup",
        )

    def replace(self, *, id: int, name: str, description: str | None = None) -> dict[str, Any]:
        """Replace a todolist (or todolist group) with a new complete representation.
        The endpoint is polymorphic - it addresses a to-do list or a group, and answers
        with the same flat Todolist shape either way.
        The request body is the recordable's full writable state: TodolistsController#update
        builds a brand-new Todolist from the permitted params and swaps it in, so any
        writable field omitted from the request is cleared server-side (a request that
        omits description erases the description). name is required - it is
        presence-validated on the model, so a request without it is rejected.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current list and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            id: The id.
            name: Name (required for a to-do list and for a group alike) - presence-validated
                server-side, so omitting it is a 422, not a preserve
            description: Description (rich text HTML) - writable for a todolist group as well as a
                todolist, and omitting it clears it either way
        """
        return self._request(
            OperationInfo(service="todolists", operation="replace", is_mutation=True, resource_id=id),
            "PUT",
            f"/todolists/{id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTodolistOrGroup",
        )

    def reposition(self, *, todolist_id: int, position: int) -> None:
        """Reposition a to-do list within its to-do set.
        position is the 1-based index among the to-do lists the caller can see; the server
        translates it relative to loose to-dos and hidden completed lists. Shifts siblings.

        Args:
            todolist_id: The todolist id.
            position: The position.
        """
        self._request_void(
            OperationInfo(service="todolists", operation="reposition", is_mutation=True, resource_id=todolist_id),
            "PUT",
            f"/todosets/todolists/{todolist_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTodolist",
        )

    def list(
        self, *, todoset_id: int, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List todolists in a todoset.

        Args:
            todoset_id: The todoset id.
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="todolists", operation="list", is_mutation=False, resource_id=todoset_id),
            f"/todosets/{todoset_id}/todolists.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListTodolists",
        )

    def create(
        self, *, todoset_id: int, name: str, description: str | None = None, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Create a new todolist in a todoset.

        Args:
            todoset_id: The todoset id.
            name: The name.
            description: The description.
            visible_to_clients: The visible to clients.
        """
        return self._request(
            OperationInfo(service="todolists", operation="create", is_mutation=True, resource_id=todoset_id),
            "POST",
            f"/todosets/{todoset_id}/todolists.json",
            json_body=self._compact(name=name, description=description, visible_to_clients=visible_to_clients),
            operation="CreateTodolist",
        )


class AsyncTodolistsService(AsyncBaseService):
    async def get(self, *, id: int) -> dict[str, Any]:
        """Get a single todolist or todolist group by id
        The endpoint is polymorphic, but it answers with one shape: BC3 has no group
        model, so both variants come back as a flat Todolist (see the Todolist shape).

        Args:
            id: The id.
        """
        return await self._request(
            OperationInfo(service="todolists", operation="get", is_mutation=False, resource_id=id),
            "GET",
            f"/todolists/{id}",
            operation="GetTodolistOrGroup",
        )

    async def replace(self, *, id: int, name: str, description: str | None = None) -> dict[str, Any]:
        """Replace a todolist (or todolist group) with a new complete representation.
        The endpoint is polymorphic - it addresses a to-do list or a group, and answers
        with the same flat Todolist shape either way.
        The request body is the recordable's full writable state: TodolistsController#update
        builds a brand-new Todolist from the permitted params and swaps it in, so any
        writable field omitted from the request is cleared server-side (a request that
        omits description erases the description). name is required - it is
        presence-validated on the model, so a request without it is rejected.
        To set some fields while preserving the rest, use the SDK's merge-safe
        update or edit methods, which GET the current list and PUT the full
        representation back. Those read-modify-write helpers are not atomic:
        a concurrent write between the GET and PUT is overwritten (last write
        wins for the whole representation; the window is one round-trip).

        Args:
            id: The id.
            name: Name (required for a to-do list and for a group alike) - presence-validated
                server-side, so omitting it is a 422, not a preserve
            description: Description (rich text HTML) - writable for a todolist group as well as a
                todolist, and omitting it clears it either way
        """
        return await self._request(
            OperationInfo(service="todolists", operation="replace", is_mutation=True, resource_id=id),
            "PUT",
            f"/todolists/{id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTodolistOrGroup",
        )

    async def reposition(self, *, todolist_id: int, position: int) -> None:
        """Reposition a to-do list within its to-do set.
        position is the 1-based index among the to-do lists the caller can see; the server
        translates it relative to loose to-dos and hidden completed lists. Shifts siblings.

        Args:
            todolist_id: The todolist id.
            position: The position.
        """
        await self._request_void(
            OperationInfo(service="todolists", operation="reposition", is_mutation=True, resource_id=todolist_id),
            "PUT",
            f"/todosets/todolists/{todolist_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTodolist",
        )

    async def list(
        self, *, todoset_id: int, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List todolists in a todoset.

        Args:
            todoset_id: The todoset id.
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="todolists", operation="list", is_mutation=False, resource_id=todoset_id),
            f"/todosets/{todoset_id}/todolists.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListTodolists",
        )

    async def create(
        self, *, todoset_id: int, name: str, description: str | None = None, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Create a new todolist in a todoset.

        Args:
            todoset_id: The todoset id.
            name: The name.
            description: The description.
            visible_to_clients: The visible to clients.
        """
        return await self._request(
            OperationInfo(service="todolists", operation="create", is_mutation=True, resource_id=todoset_id),
            "POST",
            f"/todosets/{todoset_id}/todolists.json",
            json_body=self._compact(name=name, description=description, visible_to_clients=visible_to_clients),
            operation="CreateTodolist",
        )
