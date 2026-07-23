# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class WormholesService(BaseService):
    def update(self, *, bucket_id: int, wormhole_id: int, destination_recording_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="wormholes", operation="update", is_mutation=True, resource_id=wormhole_id),
            "PUT",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="UpdateWormhole",
        )

    def delete(self, *, bucket_id: int, wormhole_id: int) -> None:
        self._request_void(
            OperationInfo(service="wormholes", operation="delete", is_mutation=True, resource_id=wormhole_id),
            "DELETE",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            operation="DeleteWormhole",
        )

    def create(self, *, bucket_id: int, card_table_id: int, destination_recording_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="wormholes", operation="create", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/buckets/{bucket_id}/card_tables/{card_table_id}/wormholes.json",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="CreateWormhole",
        )


class AsyncWormholesService(AsyncBaseService):
    async def update(self, *, bucket_id: int, wormhole_id: int, destination_recording_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="wormholes", operation="update", is_mutation=True, resource_id=wormhole_id),
            "PUT",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="UpdateWormhole",
        )

    async def delete(self, *, bucket_id: int, wormhole_id: int) -> None:
        await self._request_void(
            OperationInfo(service="wormholes", operation="delete", is_mutation=True, resource_id=wormhole_id),
            "DELETE",
            f"/buckets/{bucket_id}/card_tables/wormholes/{wormhole_id}",
            operation="DeleteWormhole",
        )

    async def create(self, *, bucket_id: int, card_table_id: int, destination_recording_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="wormholes", operation="create", is_mutation=True, resource_id=card_table_id),
            "POST",
            f"/buckets/{bucket_id}/card_tables/{card_table_id}/wormholes.json",
            json_body=self._compact(destination_recording_id=destination_recording_id),
            operation="CreateWormhole",
        )
