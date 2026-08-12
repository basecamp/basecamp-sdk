# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CardColumnsService(BaseService):
    def set_color(self, *, bucket_id: int, column_id: int, color: str) -> dict[str, Any]:
        """Set the color of a column.

        Args:
            bucket_id: The bucket id.
            column_id: The column id.
            color: Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink,
                brown
        """
        return self._request(
            OperationInfo(
                service="cardcolumns",
                operation="set_color",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=column_id,
            ),
            "PUT",
            f"/buckets/{bucket_id}/card_tables/columns/{column_id}/color.json",
            json_body=self._compact(color=color),
            operation="SetCardColumnColor",
        )

    def enable_on_hold(self, *, bucket_id: int, column_id: int) -> dict[str, Any]:
        """Enable on-hold section in a column.

        Args:
            bucket_id: The bucket id.
            column_id: The column id.
        """
        return self._request(
            OperationInfo(
                service="cardcolumns",
                operation="enable_on_hold",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=column_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/card_tables/columns/{column_id}/on_hold.json",
            operation="EnableCardColumnOnHold",
        )

    def disable_on_hold(self, *, bucket_id: int, column_id: int) -> dict[str, Any]:
        """Disable on-hold section in a column.

        Args:
            bucket_id: The bucket id.
            column_id: The column id.
        """
        return self._request(
            OperationInfo(
                service="cardcolumns",
                operation="disable_on_hold",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=column_id,
            ),
            "DELETE",
            f"/buckets/{bucket_id}/card_tables/columns/{column_id}/on_hold.json",
            operation="DisableCardColumnOnHold",
        )

    def get(self, *, column_id: int) -> dict[str, Any]:
        """Get a card column by ID.

        Args:
            column_id: The column id.
        """
        return self._request(
            OperationInfo(service="cardcolumns", operation="get", is_mutation=False, resource_id=column_id),
            "GET",
            f"/card_tables/columns/{column_id}",
            operation="GetCardColumn",
        )

    def update(self, *, column_id: int, title: str | None = None, description: str | None = None) -> dict[str, Any]:
        """Update an existing column.

        Args:
            column_id: The column id.
            title: The title.
            description: The description.
        """
        return self._request(
            OperationInfo(service="cardcolumns", operation="update", is_mutation=True, resource_id=column_id),
            "PUT",
            f"/card_tables/columns/{column_id}",
            json_body=self._compact(title=title, description=description),
            operation="UpdateCardColumn",
        )

    def subscribe_to_column(self, *, column_id: int) -> None:
        """Subscribe to a card column (watch for changes).

        Args:
            column_id: The column id.
        """
        self._request_void(
            OperationInfo(
                service="cardcolumns", operation="subscribe_to_column", is_mutation=True, resource_id=column_id
            ),
            "POST",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="SubscribeToCardColumn",
        )

    def unsubscribe_from_column(self, *, column_id: int) -> None:
        """Unsubscribe from a card column (stop watching for changes).

        Args:
            column_id: The column id.
        """
        self._request_void(
            OperationInfo(
                service="cardcolumns", operation="unsubscribe_from_column", is_mutation=True, resource_id=column_id
            ),
            "DELETE",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="UnsubscribeFromCardColumn",
        )

    def create(self, *, card_table_id: int, title: str, description: str | None = None) -> dict[str, Any]:
        """Create a column in a card table.

        Args:
            card_table_id: The card table id.
            title: The title.
            description: The description.
        """
        return self._request(
            OperationInfo(service="cardcolumns", operation="create", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/columns.json",
            json_body=self._compact(title=title, description=description),
            operation="CreateCardColumn",
        )

    def move(self, *, card_table_id: int, source_id: int, target_id: int, position: int | None = None) -> None:
        """Move a column within a card table.

        Args:
            card_table_id: The card table id.
            source_id: The source id.
            target_id: The target id.
            position: The position.
        """
        self._request_void(
            OperationInfo(service="cardcolumns", operation="move", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/moves.json",
            json_body=self._compact(source_id=source_id, target_id=target_id, position=position),
            operation="MoveCardColumn",
        )


class AsyncCardColumnsService(AsyncBaseService):
    async def set_color(self, *, bucket_id: int, column_id: int, color: str) -> dict[str, Any]:
        """Set the color of a column.

        Args:
            bucket_id: The bucket id.
            column_id: The column id.
            color: Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink,
                brown
        """
        return await self._request(
            OperationInfo(
                service="cardcolumns",
                operation="set_color",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=column_id,
            ),
            "PUT",
            f"/buckets/{bucket_id}/card_tables/columns/{column_id}/color.json",
            json_body=self._compact(color=color),
            operation="SetCardColumnColor",
        )

    async def enable_on_hold(self, *, bucket_id: int, column_id: int) -> dict[str, Any]:
        """Enable on-hold section in a column.

        Args:
            bucket_id: The bucket id.
            column_id: The column id.
        """
        return await self._request(
            OperationInfo(
                service="cardcolumns",
                operation="enable_on_hold",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=column_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/card_tables/columns/{column_id}/on_hold.json",
            operation="EnableCardColumnOnHold",
        )

    async def disable_on_hold(self, *, bucket_id: int, column_id: int) -> dict[str, Any]:
        """Disable on-hold section in a column.

        Args:
            bucket_id: The bucket id.
            column_id: The column id.
        """
        return await self._request(
            OperationInfo(
                service="cardcolumns",
                operation="disable_on_hold",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=column_id,
            ),
            "DELETE",
            f"/buckets/{bucket_id}/card_tables/columns/{column_id}/on_hold.json",
            operation="DisableCardColumnOnHold",
        )

    async def get(self, *, column_id: int) -> dict[str, Any]:
        """Get a card column by ID.

        Args:
            column_id: The column id.
        """
        return await self._request(
            OperationInfo(service="cardcolumns", operation="get", is_mutation=False, resource_id=column_id),
            "GET",
            f"/card_tables/columns/{column_id}",
            operation="GetCardColumn",
        )

    async def update(
        self, *, column_id: int, title: str | None = None, description: str | None = None
    ) -> dict[str, Any]:
        """Update an existing column.

        Args:
            column_id: The column id.
            title: The title.
            description: The description.
        """
        return await self._request(
            OperationInfo(service="cardcolumns", operation="update", is_mutation=True, resource_id=column_id),
            "PUT",
            f"/card_tables/columns/{column_id}",
            json_body=self._compact(title=title, description=description),
            operation="UpdateCardColumn",
        )

    async def subscribe_to_column(self, *, column_id: int) -> None:
        """Subscribe to a card column (watch for changes).

        Args:
            column_id: The column id.
        """
        await self._request_void(
            OperationInfo(
                service="cardcolumns", operation="subscribe_to_column", is_mutation=True, resource_id=column_id
            ),
            "POST",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="SubscribeToCardColumn",
        )

    async def unsubscribe_from_column(self, *, column_id: int) -> None:
        """Unsubscribe from a card column (stop watching for changes).

        Args:
            column_id: The column id.
        """
        await self._request_void(
            OperationInfo(
                service="cardcolumns", operation="unsubscribe_from_column", is_mutation=True, resource_id=column_id
            ),
            "DELETE",
            f"/card_tables/lists/{column_id}/subscription.json",
            operation="UnsubscribeFromCardColumn",
        )

    async def create(self, *, card_table_id: int, title: str, description: str | None = None) -> dict[str, Any]:
        """Create a column in a card table.

        Args:
            card_table_id: The card table id.
            title: The title.
            description: The description.
        """
        return await self._request(
            OperationInfo(service="cardcolumns", operation="create", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/columns.json",
            json_body=self._compact(title=title, description=description),
            operation="CreateCardColumn",
        )

    async def move(self, *, card_table_id: int, source_id: int, target_id: int, position: int | None = None) -> None:
        """Move a column within a card table.

        Args:
            card_table_id: The card table id.
            source_id: The source id.
            target_id: The target id.
            position: The position.
        """
        await self._request_void(
            OperationInfo(service="cardcolumns", operation="move", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/card_tables/{card_table_id}/moves.json",
            json_body=self._compact(source_id=source_id, target_id=target_id, position=position),
            operation="MoveCardColumn",
        )
