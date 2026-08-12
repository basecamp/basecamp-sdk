# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class WormholesService(BaseService):
    def update(self, *, bucket_id: int, wormhole_id: int, destination_recording_id: int) -> dict[str, Any]:
        """Update a wormhole's destination column.

        Args:
            bucket_id: The bucket id.
            wormhole_id: The wormhole id.
            destination_recording_id: Id of the new destination column (on another accessible card
                table).
        """
        return self._request(
            OperationInfo(
                service="wormholes", operation="update", is_mutation=True, project_id=bucket_id, resource_id=wormhole_id
            ),
            "PUT",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="UpdateWormhole",
        )

    def delete(self, *, bucket_id: int, wormhole_id: int) -> None:
        """Delete a wormhole.

        Args:
            bucket_id: The bucket id.
            wormhole_id: The wormhole id.
        """
        self._request_void(
            OperationInfo(
                service="wormholes", operation="delete", is_mutation=True, project_id=bucket_id, resource_id=wormhole_id
            ),
            "DELETE",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            operation="DeleteWormhole",
        )

    def create(self, *, bucket_id: int, card_table_id: int, destination_recording_id: int) -> dict[str, Any]:
        """Create a wormhole linking this card table to a column on another card table.

        A wormhole is the only mechanism for moving a card to a different project: its
        id is a valid `column_id` for MoveCard, teleporting the card across projects.
        `destinationRecordingId` is the id of a column on another accessible card table.

        Args:
            bucket_id: The bucket id.
            card_table_id: The card table id.
            destination_recording_id: Id of the destination column (on another accessible card
                table) to link to.
        """
        return self._request(
            OperationInfo(
                service="wormholes",
                operation="create",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=card_table_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/card_tables/{card_table_id}/wormholes.json",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="CreateWormhole",
        )


class AsyncWormholesService(AsyncBaseService):
    async def update(self, *, bucket_id: int, wormhole_id: int, destination_recording_id: int) -> dict[str, Any]:
        """Update a wormhole's destination column.

        Args:
            bucket_id: The bucket id.
            wormhole_id: The wormhole id.
            destination_recording_id: Id of the new destination column (on another accessible card
                table).
        """
        return await self._request(
            OperationInfo(
                service="wormholes", operation="update", is_mutation=True, project_id=bucket_id, resource_id=wormhole_id
            ),
            "PUT",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="UpdateWormhole",
        )

    async def delete(self, *, bucket_id: int, wormhole_id: int) -> None:
        """Delete a wormhole.

        Args:
            bucket_id: The bucket id.
            wormhole_id: The wormhole id.
        """
        await self._request_void(
            OperationInfo(
                service="wormholes", operation="delete", is_mutation=True, project_id=bucket_id, resource_id=wormhole_id
            ),
            "DELETE",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            operation="DeleteWormhole",
        )

    async def create(self, *, bucket_id: int, card_table_id: int, destination_recording_id: int) -> dict[str, Any]:
        """Create a wormhole linking this card table to a column on another card table.

        A wormhole is the only mechanism for moving a card to a different project: its
        id is a valid `column_id` for MoveCard, teleporting the card across projects.
        `destinationRecordingId` is the id of a column on another accessible card table.

        Args:
            bucket_id: The bucket id.
            card_table_id: The card table id.
            destination_recording_id: Id of the destination column (on another accessible card
                table) to link to.
        """
        return await self._request(
            OperationInfo(
                service="wormholes",
                operation="create",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=card_table_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/card_tables/{card_table_id}/wormholes.json",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="CreateWormhole",
        )
