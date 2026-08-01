# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class EventsService(BaseService):
    def list(self, *, recording_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="events", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/events.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListEvents",
        )


class AsyncEventsService(AsyncBaseService):
    async def list(self, *, recording_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="events", operation="list", is_mutation=False, resource_id=recording_id),
            f"/recordings/{recording_id}/events.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListEvents",
        )
