# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TodosetsService(BaseService):
    def get(self, *, todoset_id: int) -> dict[str, Any]:
        """Get a todoset (container for todolists in a project).

        Args:
            todoset_id: The todoset id.
        """
        return self._request(
            OperationInfo(service="todosets", operation="get", is_mutation=False, resource_id=todoset_id),
            "GET",
            f"/todosets/{todoset_id}",
            operation="GetTodoset",
        )


class AsyncTodosetsService(AsyncBaseService):
    async def get(self, *, todoset_id: int) -> dict[str, Any]:
        """Get a todoset (container for todolists in a project).

        Args:
            todoset_id: The todoset id.
        """
        return await self._request(
            OperationInfo(service="todosets", operation="get", is_mutation=False, resource_id=todoset_id),
            "GET",
            f"/todosets/{todoset_id}",
            operation="GetTodoset",
        )
