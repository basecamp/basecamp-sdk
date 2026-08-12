# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CardsService(BaseService):
    def get(self, *, card_id: int) -> dict[str, Any]:
        """Get a card by ID.

        Args:
            card_id: The card id.
        """
        return self._request(
            OperationInfo(service="cards", operation="get", is_mutation=False, resource_id=card_id),
            "GET",
            f"/card_tables/cards/{card_id}",
            operation="GetCard",
        )

    def update_verbatim(
        self,
        *,
        card_id: int,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Update an existing card.

        Args:
            card_id: The card id.
            title: The title.
            content: The content.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return self._request(
            OperationInfo(service="cards", operation="update_verbatim", is_mutation=True, resource_id=card_id),
            "PUT",
            f"/card_tables/cards/{card_id}",
            json_body=self._compact(title=title, content=content, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateCard",
        )

    def move(self, *, card_id: int, column_id: int, position: int | None = None) -> None:
        """Move a card to a different column.

        Args:
            card_id: The card id.
            column_id: The column id.
            position: 1-indexed position within the destination column. Defaults to 1 (top).
        """
        self._request_void(
            OperationInfo(service="cards", operation="move", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/moves.json",
            json_body=self._compact(column_id=column_id, position=position),
            operation="MoveCard",
        )

    def list(self, *, column_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List cards in a column.

        Args:
            column_id: The column id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="cards", operation="list", is_mutation=False, resource_id=column_id),
            f"/card_tables/lists/{column_id}/cards.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListCards",
        )

    def create(
        self,
        *,
        column_id: int,
        title: str,
        content: str | None = None,
        due_on: str | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        """Create a card in a column.

        Args:
            column_id: The column id.
            title: The title.
            content: The content.
            due_on: The due on.
            notify: The notify.
        """
        return self._request(
            OperationInfo(service="cards", operation="create", is_mutation=True, resource_id=column_id),
            "POST",
            f"/card_tables/lists/{column_id}/cards.json",
            json_body=self._compact(title=title, content=content, due_on=due_on, notify=notify),
            operation="CreateCard",
        )


class AsyncCardsService(AsyncBaseService):
    async def get(self, *, card_id: int) -> dict[str, Any]:
        """Get a card by ID.

        Args:
            card_id: The card id.
        """
        return await self._request(
            OperationInfo(service="cards", operation="get", is_mutation=False, resource_id=card_id),
            "GET",
            f"/card_tables/cards/{card_id}",
            operation="GetCard",
        )

    async def update_verbatim(
        self,
        *,
        card_id: int,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Update an existing card.

        Args:
            card_id: The card id.
            title: The title.
            content: The content.
            due_on: The due on.
            assignee_ids: The assignee ids.
        """
        return await self._request(
            OperationInfo(service="cards", operation="update_verbatim", is_mutation=True, resource_id=card_id),
            "PUT",
            f"/card_tables/cards/{card_id}",
            json_body=self._compact(title=title, content=content, due_on=due_on, assignee_ids=assignee_ids),
            operation="UpdateCard",
        )

    async def move(self, *, card_id: int, column_id: int, position: int | None = None) -> None:
        """Move a card to a different column.

        Args:
            card_id: The card id.
            column_id: The column id.
            position: 1-indexed position within the destination column. Defaults to 1 (top).
        """
        await self._request_void(
            OperationInfo(service="cards", operation="move", is_mutation=True, resource_id=card_id),
            "POST",
            f"/card_tables/cards/{card_id}/moves.json",
            json_body=self._compact(column_id=column_id, position=position),
            operation="MoveCard",
        )

    async def list(self, *, column_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List cards in a column.

        Args:
            column_id: The column id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="cards", operation="list", is_mutation=False, resource_id=column_id),
            f"/card_tables/lists/{column_id}/cards.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListCards",
        )

    async def create(
        self,
        *,
        column_id: int,
        title: str,
        content: str | None = None,
        due_on: str | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        """Create a card in a column.

        Args:
            column_id: The column id.
            title: The title.
            content: The content.
            due_on: The due on.
            notify: The notify.
        """
        return await self._request(
            OperationInfo(service="cards", operation="create", is_mutation=True, resource_id=column_id),
            "POST",
            f"/card_tables/lists/{column_id}/cards.json",
            json_body=self._compact(title=title, content=content, due_on=due_on, notify=notify),
            operation="CreateCard",
        )
