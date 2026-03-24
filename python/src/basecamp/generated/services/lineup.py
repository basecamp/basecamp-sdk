# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class LineupService(BaseService):
    def create(self, *, name: str, date: str) -> None:
        self._request_void(
            OperationInfo(service="lineup", operation="create", is_mutation=True),
            "POST",
            "/lineup/markers.json",
            json_body=self._compact(name=name, date=date),
            operation="CreateLineupMarker",
        )

    def update(self, *, marker_id: int, name: str | None = None, date: str | None = None) -> None:
        self._request_void(
            OperationInfo(service="lineup", operation="update", is_mutation=True, resource_id=marker_id),
            "PUT",
            f"/lineup/markers/{marker_id}",
            json_body=self._compact(name=name, date=date),
            operation="UpdateLineupMarker",
        )

    def delete(self, *, marker_id: int) -> None:
        self._request_void(
            OperationInfo(service="lineup", operation="delete", is_mutation=True, resource_id=marker_id),
            "DELETE",
            f"/lineup/markers/{marker_id}",
            operation="DeleteLineupMarker",
        )


class AsyncLineupService(AsyncBaseService):
    async def create(self, *, name: str, date: str) -> None:
        await self._request_void(
            OperationInfo(service="lineup", operation="create", is_mutation=True),
            "POST",
            "/lineup/markers.json",
            json_body=self._compact(name=name, date=date),
            operation="CreateLineupMarker",
        )

    async def update(self, *, marker_id: int, name: str | None = None, date: str | None = None) -> None:
        await self._request_void(
            OperationInfo(service="lineup", operation="update", is_mutation=True, resource_id=marker_id),
            "PUT",
            f"/lineup/markers/{marker_id}",
            json_body=self._compact(name=name, date=date),
            operation="UpdateLineupMarker",
        )

    async def delete(self, *, marker_id: int) -> None:
        await self._request_void(
            OperationInfo(service="lineup", operation="delete", is_mutation=True, resource_id=marker_id),
            "DELETE",
            f"/lineup/markers/{marker_id}",
            operation="DeleteLineupMarker",
        )
