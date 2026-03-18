# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CardColumnsService(BaseService):
    def get(self, *, column_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardcolumns", operation="get", is_mutation=False, resource_id=column_id),
            "GET",
            f"/card_tables/columns/{column_id}",
        )

    def update(
        self, *, column_id: int | str, title: str | None = None, description: str | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardcolumns", operation="update", is_mutation=True, resource_id=column_id),
            "PUT",
            f"/card_tables/columns/{column_id}",
            json_body=self._compact(title=title, description=description),
            operation="UpdateCardColumn",
        )

    def set_color(self, *, column_id: int | str, color: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardcolumns", operation="set_color", is_mutation=True, resource_id=column_id),
            "PUT",
            f"/card_tables/columns/{column_id}/color.json",
            json_body=self._compact(color=color),
            operation="SetCardColumnColor",
        )

    def enable_on_hold(self, *, column_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardcolumns", operation="enable_on_hold", is_mutation=True, resource_id=column_id),
            "POST",
            f"/card_tables/columns/{column_id}/on_hold.json",
            operation="EnableCardColumnOnHold",
        )

    def disable_on_hold(self, *, column_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardcolumns", operation="disable_on_hold", is_mutation=True, resource_id=column_id),
            "DELETE",
            f"/card_tables/columns/{column_id}/on_hold.json",
            operation="DisableCardColumnOnHold",
        )

    def subscribe_to_column(self, *, column_id: int | str) -> None:
        self._request_void(
            OperationInfo(
                service="cardcolumns", operation="subscribe_to_column", is_mutation=True, resource_id=column_id
            ),
            "POST",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="SubscribeToCardColumn",
        )

    def unsubscribe_from_column(self, *, column_id: int | str) -> None:
        self._request_void(
            OperationInfo(
                service="cardcolumns", operation="unsubscribe_from_column", is_mutation=True, resource_id=column_id
            ),
            "DELETE",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="UnsubscribeFromCardColumn",
        )

    def create(self, *, card_table_id: int | str, title: str, description: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardcolumns", operation="create", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/columns.json",
            json_body=self._compact(title=title, description=description),
            operation="CreateCardColumn",
        )

    def move(self, *, card_table_id: int | str, source_id: int, target_id: int, position: int | None = None) -> None:
        self._request_void(
            OperationInfo(service="cardcolumns", operation="move", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/moves.json",
            json_body=self._compact(source_id=source_id, target_id=target_id, position=position),
            operation="MoveCardColumn",
        )


class AsyncCardColumnsService(AsyncBaseService):
    async def get(self, *, column_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardcolumns", operation="get", is_mutation=False, resource_id=column_id),
            "GET",
            f"/card_tables/columns/{column_id}",
        )

    async def update(
        self, *, column_id: int | str, title: str | None = None, description: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardcolumns", operation="update", is_mutation=True, resource_id=column_id),
            "PUT",
            f"/card_tables/columns/{column_id}",
            json_body=self._compact(title=title, description=description),
            operation="UpdateCardColumn",
        )

    async def set_color(self, *, column_id: int | str, color: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardcolumns", operation="set_color", is_mutation=True, resource_id=column_id),
            "PUT",
            f"/card_tables/columns/{column_id}/color.json",
            json_body=self._compact(color=color),
            operation="SetCardColumnColor",
        )

    async def enable_on_hold(self, *, column_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardcolumns", operation="enable_on_hold", is_mutation=True, resource_id=column_id),
            "POST",
            f"/card_tables/columns/{column_id}/on_hold.json",
            operation="EnableCardColumnOnHold",
        )

    async def disable_on_hold(self, *, column_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardcolumns", operation="disable_on_hold", is_mutation=True, resource_id=column_id),
            "DELETE",
            f"/card_tables/columns/{column_id}/on_hold.json",
            operation="DisableCardColumnOnHold",
        )

    async def subscribe_to_column(self, *, column_id: int | str) -> None:
        await self._request_void(
            OperationInfo(
                service="cardcolumns", operation="subscribe_to_column", is_mutation=True, resource_id=column_id
            ),
            "POST",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="SubscribeToCardColumn",
        )

    async def unsubscribe_from_column(self, *, column_id: int | str) -> None:
        await self._request_void(
            OperationInfo(
                service="cardcolumns", operation="unsubscribe_from_column", is_mutation=True, resource_id=column_id
            ),
            "DELETE",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="UnsubscribeFromCardColumn",
        )

    async def create(self, *, card_table_id: int | str, title: str, description: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardcolumns", operation="create", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/columns.json",
            json_body=self._compact(title=title, description=description),
            operation="CreateCardColumn",
        )

    async def move(
        self, *, card_table_id: int | str, source_id: int, target_id: int, position: int | None = None
    ) -> None:
        await self._request_void(
            OperationInfo(service="cardcolumns", operation="move", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/moves.json",
            json_body=self._compact(source_id=source_id, target_id=target_id, position=position),
            operation="MoveCardColumn",
        )
