# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CardStepsService(BaseService):
    def reposition(self, *, card_id: int, source_id: int, position: int) -> None:
        """Reposition a step within a card.

        Args:
            card_id: The card id.
            source_id: The source id.
            position: 0-indexed position
        """
        self._request_void(
            OperationInfo(service="cardsteps", operation="reposition", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/positions.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="RepositionCardStep",
        )

    def create(
        self, *, card_id: int, title: str, due_on: str | None = None, assignee_ids: list[int] | None = None
    ) -> dict[str, Any]:
        """Create a step on a card.

        Args:
            card_id: The card id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return self._request(
            OperationInfo(service="cardsteps", operation="create", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/steps.json",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="CreateCardStep",
        )

    def get(self, *, step_id: int) -> dict[str, Any]:
        """Get a step by ID.

        Args:
            step_id: The step id.
        """
        return self._request(
            OperationInfo(service="cardsteps", operation="get", is_mutation=False, resource_id=step_id),
            "GET",
            f"/card_tables/steps/{step_id}",
            operation="GetCardStep",
        )

    def update(
        self,
        *,
        step_id: int,
        title: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Update an existing step.

        Args:
            step_id: The step id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return self._request(
            OperationInfo(service="cardsteps", operation="update", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateCardStep",
        )

    def set_completion(self, *, step_id: int, completion: str) -> dict[str, Any]:
        """Set card step completion status (PUT with completion: "on" to complete, "" to uncomplete).

        Args:
            step_id: The step id.
            completion: Set to "on" to complete the step, "" (empty) to uncomplete
        """
        return self._request(
            OperationInfo(service="cardsteps", operation="set_completion", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}/completions.json",
            json_body=self._compact(completion=completion),
            operation="SetCardStepCompletion",
        )


class AsyncCardStepsService(AsyncBaseService):
    async def reposition(self, *, card_id: int, source_id: int, position: int) -> None:
        """Reposition a step within a card.

        Args:
            card_id: The card id.
            source_id: The source id.
            position: 0-indexed position
        """
        await self._request_void(
            OperationInfo(service="cardsteps", operation="reposition", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/positions.json",
            json_body=self._compact(source_id=source_id, position=position),
            operation="RepositionCardStep",
        )

    async def create(
        self, *, card_id: int, title: str, due_on: str | None = None, assignee_ids: list[int] | None = None
    ) -> dict[str, Any]:
        """Create a step on a card.

        Args:
            card_id: The card id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return await self._request(
            OperationInfo(service="cardsteps", operation="create", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/steps.json",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="CreateCardStep",
        )

    async def get(self, *, step_id: int) -> dict[str, Any]:
        """Get a step by ID.

        Args:
            step_id: The step id.
        """
        return await self._request(
            OperationInfo(service="cardsteps", operation="get", is_mutation=False, resource_id=step_id),
            "GET",
            f"/card_tables/steps/{step_id}",
            operation="GetCardStep",
        )

    async def update(
        self,
        *,
        step_id: int,
        title: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Update an existing step.

        Args:
            step_id: The step id.
            title: The title.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return await self._request(
            OperationInfo(service="cardsteps", operation="update", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}",
            json_body=self._compact(title=title, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateCardStep",
        )

    async def set_completion(self, *, step_id: int, completion: str) -> dict[str, Any]:
        """Set card step completion status (PUT with completion: "on" to complete, "" to uncomplete).

        Args:
            step_id: The step id.
            completion: Set to "on" to complete the step, "" (empty) to uncomplete
        """
        return await self._request(
            OperationInfo(service="cardsteps", operation="set_completion", is_mutation=True, resource_id=step_id),
            "PUT",
            f"/card_tables/steps/{step_id}/completions.json",
            json_body=self._compact(completion=completion),
            operation="SetCardStepCompletion",
        )
