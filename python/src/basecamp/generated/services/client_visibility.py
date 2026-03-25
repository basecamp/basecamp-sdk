# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ClientVisibilityService(BaseService):
    def set_visibility(self, *, recording_id: int, visible_to_clients: bool) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="clientvisibility", operation="set_visibility", is_mutation=True, resource_id=recording_id
            ),
            "PUT",
            f"/recordings/{recording_id}/client_visibility.json",
            json_body=self._compact(visible_to_clients=visible_to_clients),
            operation="SetClientVisibility",
        )


class AsyncClientVisibilityService(AsyncBaseService):
    async def set_visibility(self, *, recording_id: int, visible_to_clients: bool) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="clientvisibility", operation="set_visibility", is_mutation=True, resource_id=recording_id
            ),
            "PUT",
            f"/recordings/{recording_id}/client_visibility.json",
            json_body=self._compact(visible_to_clients=visible_to_clients),
            operation="SetClientVisibility",
        )
