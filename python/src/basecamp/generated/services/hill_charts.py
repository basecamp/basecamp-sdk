# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class HillChartsService(BaseService):
    def get(self, *, todoset_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="hillcharts", operation="get", is_mutation=False, resource_id=todoset_id),
            "GET",
            f"/todosets/{todoset_id}/hill.json",
        )

    def update_settings(
        self, *, todoset_id: int | str, tracked: list | None = None, untracked: list | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="hillcharts", operation="update_settings", is_mutation=True, resource_id=todoset_id),
            "PUT",
            f"/todosets/{todoset_id}/hills/settings.json",
            json_body=self._compact(tracked=tracked, untracked=untracked),
            operation="UpdateHillChartSettings",
        )


class AsyncHillChartsService(AsyncBaseService):
    async def get(self, *, todoset_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="hillcharts", operation="get", is_mutation=False, resource_id=todoset_id),
            "GET",
            f"/todosets/{todoset_id}/hill.json",
        )

    async def update_settings(
        self, *, todoset_id: int | str, tracked: list | None = None, untracked: list | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="hillcharts", operation="update_settings", is_mutation=True, resource_id=todoset_id),
            "PUT",
            f"/todosets/{todoset_id}/hills/settings.json",
            json_body=self._compact(tracked=tracked, untracked=untracked),
            operation="UpdateHillChartSettings",
        )
