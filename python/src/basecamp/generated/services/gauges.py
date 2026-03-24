# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class GaugesService(BaseService):
    def get_gauge_needle(self, *, needle_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="gauges", operation="get_gauge_needle", is_mutation=False, resource_id=needle_id),
            "GET",
            f"/gauge_needles/{needle_id}",
        )

    def update_gauge_needle(self, *, needle_id: int | str, gauge_needle: dict | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="gauges", operation="update_gauge_needle", is_mutation=True, resource_id=needle_id),
            "PUT",
            f"/gauge_needles/{needle_id}",
            json_body=self._compact(gauge_needle=gauge_needle),
            operation="UpdateGaugeNeedle",
        )

    def destroy_gauge_needle(self, *, needle_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="gauges", operation="destroy_gauge_needle", is_mutation=True, resource_id=needle_id),
            "DELETE",
            f"/gauge_needles/{needle_id}",
            operation="DestroyGaugeNeedle",
        )

    def toggle_gauge(self, *, project_id: int | str, gauge: dict) -> None:
        self._request_void(
            OperationInfo(service="gauges", operation="toggle_gauge", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/gauge.json",
            json_body=self._compact(gauge=gauge),
            operation="ToggleGauge",
        )

    def list_gauge_needles(self, *, project_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauge_needles", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/gauge/needles.json",
        )

    def create_gauge_needle(
        self, *, project_id: int | str, gauge_needle: dict, notify: str | None = None, subscriptions: list | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="gauges", operation="create_gauge_needle", is_mutation=True, project_id=project_id),
            "POST",
            f"/projects/{project_id}/gauge/needles.json",
            json_body=self._compact(gauge_needle=gauge_needle, notify=notify, subscriptions=subscriptions),
            operation="CreateGaugeNeedle",
        )

    def list_gauges(self, *, bucket_ids: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauges", is_mutation=False),
            "/reports/gauges.json",
            params=self._compact(bucket_ids=bucket_ids),
        )


class AsyncGaugesService(AsyncBaseService):
    async def get_gauge_needle(self, *, needle_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="gauges", operation="get_gauge_needle", is_mutation=False, resource_id=needle_id),
            "GET",
            f"/gauge_needles/{needle_id}",
        )

    async def update_gauge_needle(self, *, needle_id: int | str, gauge_needle: dict | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="gauges", operation="update_gauge_needle", is_mutation=True, resource_id=needle_id),
            "PUT",
            f"/gauge_needles/{needle_id}",
            json_body=self._compact(gauge_needle=gauge_needle),
            operation="UpdateGaugeNeedle",
        )

    async def destroy_gauge_needle(self, *, needle_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="gauges", operation="destroy_gauge_needle", is_mutation=True, resource_id=needle_id),
            "DELETE",
            f"/gauge_needles/{needle_id}",
            operation="DestroyGaugeNeedle",
        )

    async def toggle_gauge(self, *, project_id: int | str, gauge: dict) -> None:
        await self._request_void(
            OperationInfo(service="gauges", operation="toggle_gauge", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/gauge.json",
            json_body=self._compact(gauge=gauge),
            operation="ToggleGauge",
        )

    async def list_gauge_needles(self, *, project_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauge_needles", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/gauge/needles.json",
        )

    async def create_gauge_needle(
        self, *, project_id: int | str, gauge_needle: dict, notify: str | None = None, subscriptions: list | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="gauges", operation="create_gauge_needle", is_mutation=True, project_id=project_id),
            "POST",
            f"/projects/{project_id}/gauge/needles.json",
            json_body=self._compact(gauge_needle=gauge_needle, notify=notify, subscriptions=subscriptions),
            operation="CreateGaugeNeedle",
        )

    async def list_gauges(self, *, bucket_ids: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauges", is_mutation=False),
            "/reports/gauges.json",
            params=self._compact(bucket_ids=bucket_ids),
        )
