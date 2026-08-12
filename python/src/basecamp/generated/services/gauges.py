# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class GaugesService(BaseService):
    def get_gauge_needle(self, *, needle_id: int) -> dict[str, Any]:
        """Get a gauge needle by ID.

        Args:
            needle_id: The needle id.
        """
        return self._request(
            OperationInfo(service="gauges", operation="get_gauge_needle", is_mutation=False, resource_id=needle_id),
            "GET",
            f"/gauge_needles/{needle_id}",
            operation="GetGaugeNeedle",
        )

    def update_gauge_needle(self, *, needle_id: int, gauge_needle: dict | None = None) -> dict[str, Any]:
        """Update a gauge needle's description. Position and color are immutable.

        Args:
            needle_id: The needle id.
            gauge_needle: The gauge needle.
        """
        return self._request(
            OperationInfo(service="gauges", operation="update_gauge_needle", is_mutation=True, resource_id=needle_id),
            "PUT",
            f"/gauge_needles/{needle_id}",
            json_body=self._compact(gauge_needle=gauge_needle),
            operation="UpdateGaugeNeedle",
        )

    def destroy_gauge_needle(self, *, needle_id: int) -> None:
        """Destroy a gauge needle.

        Args:
            needle_id: The needle id.
        """
        self._request_void(
            OperationInfo(service="gauges", operation="destroy_gauge_needle", is_mutation=True, resource_id=needle_id),
            "DELETE",
            f"/gauge_needles/{needle_id}",
            operation="DestroyGaugeNeedle",
        )

    def toggle_gauge(self, *, project_id: int, gauge: dict) -> None:
        """Enable or disable the gauge for a project. Only project admins can toggle gauges.

        Args:
            project_id: The project id.
            gauge: The gauge.
        """
        self._request_void(
            OperationInfo(service="gauges", operation="toggle_gauge", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/gauge.json",
            json_body=self._compact(gauge=gauge),
            operation="ToggleGauge",
        )

    def list_gauge_needles(
        self, *, project_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List gauge needles for a project, ordered newest first.

        Args:
            project_id: The project id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauge_needles", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/gauge/needles.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListGaugeNeedles",
        )

    def create_gauge_needle(
        self, *, project_id: int, gauge_needle: dict, notify: str | None = None, subscriptions: list[int] | None = None
    ) -> dict[str, Any]:
        """Create a gauge needle (progress update) for a project.

        Args:
            project_id: The project id.
            gauge_needle: The gauge needle.
            notify: Who to notify: "everyone", "working_on", "custom", or omit for nobody
            subscriptions: Array of people IDs to notify (only used when notify is "custom")
        """
        return self._request(
            OperationInfo(service="gauges", operation="create_gauge_needle", is_mutation=True, project_id=project_id),
            "POST",
            f"/projects/{project_id}/gauge/needles.json",
            json_body=self._compact(gauge_needle=gauge_needle, notify=notify, subscriptions=subscriptions),
            operation="CreateGaugeNeedle",
        )

    def list_gauges(
        self, *, bucket_ids: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List gauges across all projects the authenticated user has access to.
        Gauges are sorted by risk level (red, yellow, green), then alphabetically.

        Args:
            bucket_ids: Comma-separated list of project IDs. When provided, results are returned in
                the order specified instead of by risk level.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauges", is_mutation=False),
            "/reports/gauges.json",
            params=self._compact(bucket_ids=bucket_ids, page=page),
            max_items=max_items,
            operation="ListGauges",
        )


class AsyncGaugesService(AsyncBaseService):
    async def get_gauge_needle(self, *, needle_id: int) -> dict[str, Any]:
        """Get a gauge needle by ID.

        Args:
            needle_id: The needle id.
        """
        return await self._request(
            OperationInfo(service="gauges", operation="get_gauge_needle", is_mutation=False, resource_id=needle_id),
            "GET",
            f"/gauge_needles/{needle_id}",
            operation="GetGaugeNeedle",
        )

    async def update_gauge_needle(self, *, needle_id: int, gauge_needle: dict | None = None) -> dict[str, Any]:
        """Update a gauge needle's description. Position and color are immutable.

        Args:
            needle_id: The needle id.
            gauge_needle: The gauge needle.
        """
        return await self._request(
            OperationInfo(service="gauges", operation="update_gauge_needle", is_mutation=True, resource_id=needle_id),
            "PUT",
            f"/gauge_needles/{needle_id}",
            json_body=self._compact(gauge_needle=gauge_needle),
            operation="UpdateGaugeNeedle",
        )

    async def destroy_gauge_needle(self, *, needle_id: int) -> None:
        """Destroy a gauge needle.

        Args:
            needle_id: The needle id.
        """
        await self._request_void(
            OperationInfo(service="gauges", operation="destroy_gauge_needle", is_mutation=True, resource_id=needle_id),
            "DELETE",
            f"/gauge_needles/{needle_id}",
            operation="DestroyGaugeNeedle",
        )

    async def toggle_gauge(self, *, project_id: int, gauge: dict) -> None:
        """Enable or disable the gauge for a project. Only project admins can toggle gauges.

        Args:
            project_id: The project id.
            gauge: The gauge.
        """
        await self._request_void(
            OperationInfo(service="gauges", operation="toggle_gauge", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/gauge.json",
            json_body=self._compact(gauge=gauge),
            operation="ToggleGauge",
        )

    async def list_gauge_needles(
        self, *, project_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List gauge needles for a project, ordered newest first.

        Args:
            project_id: The project id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauge_needles", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/gauge/needles.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListGaugeNeedles",
        )

    async def create_gauge_needle(
        self, *, project_id: int, gauge_needle: dict, notify: str | None = None, subscriptions: list[int] | None = None
    ) -> dict[str, Any]:
        """Create a gauge needle (progress update) for a project.

        Args:
            project_id: The project id.
            gauge_needle: The gauge needle.
            notify: Who to notify: "everyone", "working_on", "custom", or omit for nobody
            subscriptions: Array of people IDs to notify (only used when notify is "custom")
        """
        return await self._request(
            OperationInfo(service="gauges", operation="create_gauge_needle", is_mutation=True, project_id=project_id),
            "POST",
            f"/projects/{project_id}/gauge/needles.json",
            json_body=self._compact(gauge_needle=gauge_needle, notify=notify, subscriptions=subscriptions),
            operation="CreateGaugeNeedle",
        )

    async def list_gauges(
        self, *, bucket_ids: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List gauges across all projects the authenticated user has access to.
        Gauges are sorted by risk level (red, yellow, green), then alphabetically.

        Args:
            bucket_ids: Comma-separated list of project IDs. When provided, results are returned in
                the order specified instead of by risk level.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="gauges", operation="list_gauges", is_mutation=False),
            "/reports/gauges.json",
            params=self._compact(bucket_ids=bucket_ids, page=page),
            max_items=max_items,
            operation="ListGauges",
        )
