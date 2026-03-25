# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CardStepsService(BaseService):
    def reposition(self, *, card_id: int, source_id: int, position: int) -> None:
        self._request_void(
            OperationInfo(service="cardsteps", operation="reposition", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/positions.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="RepositionCardStep",
        )

    def create(
        self, *, card_id: int, title: str, due_on: str | None = None, assignees: list | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardsteps", operation="create", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/steps.json",
            json_body=self._compact(title=title, due_on=due_on, assignees=assignees),
            operation="CreateCardStep",
        )

    def get(self, *, step_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardsteps", operation="get", is_mutation=False, resource_id=step_id),
            "GET",
            f"/card_tables/steps/{step_id}",
        )

    def update(
        self, *, step_id: int, title: str | None = None, due_on: str | None = None, assignees: list | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardsteps", operation="update", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}",
            json_body=self._compact(title=title, due_on=due_on, assignees=assignees),
            operation="UpdateCardStep",
        )

    def set_completion(self, *, step_id: int, completion: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardsteps", operation="set_completion", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}/completions.json",
            json_body=self._compact(completion=completion),
            operation="SetCardStepCompletion",
        )


class AsyncCardStepsService(AsyncBaseService):
    async def reposition(self, *, card_id: int, source_id: int, position: int) -> None:
        await self._request_void(
            OperationInfo(service="cardsteps", operation="reposition", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/positions.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="RepositionCardStep",
        )

    async def create(
        self, *, card_id: int, title: str, due_on: str | None = None, assignees: list | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardsteps", operation="create", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/steps.json",
            json_body=self._compact(title=title, due_on=due_on, assignees=assignees),
            operation="CreateCardStep",
        )

    async def get(self, *, step_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardsteps", operation="get", is_mutation=False, resource_id=step_id),
            "GET",
            f"/card_tables/steps/{step_id}",
        )

    async def update(
        self, *, step_id: int, title: str | None = None, due_on: str | None = None, assignees: list | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardsteps", operation="update", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}",
            json_body=self._compact(title=title, due_on=due_on, assignees=assignees),
            operation="UpdateCardStep",
        )

    async def set_completion(self, *, step_id: int, completion: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardsteps", operation="set_completion", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}/completions.json",
            json_body=self._compact(completion=completion),
            operation="SetCardStepCompletion",
        )
