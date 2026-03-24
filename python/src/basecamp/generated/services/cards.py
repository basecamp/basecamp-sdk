# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CardsService(BaseService):
    def get(self, *, card_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cards", operation="get", is_mutation=False, resource_id=card_id),
            "GET",
            f"/card_tables/cards/{card_id}",
        )

    def update(
        self,
        *,
        card_id: int | str,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cards", operation="update", is_mutation=True, resource_id=card_id),
            "PUT",
            f"/card_tables/cards/{card_id}",
            json_body=self._compact(title=title, content=content, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateCard",
        )

    def move(self, *, card_id: int | str, column_id: int, position: int | None = None) -> None:
        self._request_void(
            OperationInfo(service="cards", operation="move", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/moves.json",
            json_body=self._compact(column_id=column_id, position=position),
            operation="MoveCard",
        )

    def list(self, *, column_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="cards", operation="list", is_mutation=False, resource_id=column_id),
            f"/card_tables/lists/{column_id}/cards.json",
        )

    def create(
        self,
        *,
        column_id: int | str,
        title: str,
        content: str | None = None,
        due_on: str | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cards", operation="create", is_mutation=True, resource_id=column_id),
            "POST",
            f"/card_tables/lists/{column_id}/cards.json",
            json_body=self._compact(title=title, content=content, due_on=due_on, notify=notify),
            operation="CreateCard",
        )


class AsyncCardsService(AsyncBaseService):
    async def get(self, *, card_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cards", operation="get", is_mutation=False, resource_id=card_id),
            "GET",
            f"/card_tables/cards/{card_id}",
        )

    async def update(
        self,
        *,
        card_id: int | str,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cards", operation="update", is_mutation=True, resource_id=card_id),
            "PUT",
            f"/card_tables/cards/{card_id}",
            json_body=self._compact(title=title, content=content, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateCard",
        )

    async def move(self, *, card_id: int | str, column_id: int, position: int | None = None) -> None:
        await self._request_void(
            OperationInfo(service="cards", operation="move", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/moves.json",
            json_body=self._compact(column_id=column_id, position=position),
            operation="MoveCard",
        )

    async def list(self, *, column_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="cards", operation="list", is_mutation=False, resource_id=column_id),
            f"/card_tables/lists/{column_id}/cards.json",
        )

    async def create(
        self,
        *,
        column_id: int | str,
        title: str,
        content: str | None = None,
        due_on: str | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cards", operation="create", is_mutation=True, resource_id=column_id),
            "POST",
            f"/card_tables/lists/{column_id}/cards.json",
            json_body=self._compact(title=title, content=content, due_on=due_on, notify=notify),
            operation="CreateCard",
        )
