# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TodolistsService(BaseService):
    def get(self, *, id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="todolists", operation="get", is_mutation=False, resource_id=id),
            "GET",
            f"/todolists/{id}",
        )

    def update(self, *, id: int, name: str | None = None, description: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="todolists", operation="update", is_mutation=True, resource_id=id),
            "PUT",
            f"/todolists/{id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTodolistOrGroup",
        )

    def list(self, *, todoset_id: int, status: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="todolists", operation="list", is_mutation=False, resource_id=todoset_id),
            f"/todosets/{todoset_id}/todolists.json",
            params=self._compact(status=status),
        )

    def create(self, *, todoset_id: int, name: str, description: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="todolists", operation="create", is_mutation=True, resource_id=todoset_id),
            "POST",
            f"/todosets/{todoset_id}/todolists.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateTodolist",
        )


class AsyncTodolistsService(AsyncBaseService):
    async def get(self, *, id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="todolists", operation="get", is_mutation=False, resource_id=id),
            "GET",
            f"/todolists/{id}",
        )

    async def update(self, *, id: int, name: str | None = None, description: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="todolists", operation="update", is_mutation=True, resource_id=id),
            "PUT",
            f"/todolists/{id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTodolistOrGroup",
        )

    async def list(self, *, todoset_id: int, status: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="todolists", operation="list", is_mutation=False, resource_id=todoset_id),
            f"/todosets/{todoset_id}/todolists.json",
            params=self._compact(status=status),
        )

    async def create(self, *, todoset_id: int, name: str, description: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="todolists", operation="create", is_mutation=True, resource_id=todoset_id),
            "POST",
            f"/todosets/{todoset_id}/todolists.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateTodolist",
        )
