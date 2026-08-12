# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ClientCorrespondencesService(BaseService):
    def list(
        self,
        *,
        bucket_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List all client correspondences in a project.

        Args:
            bucket_id: The bucket id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="clientcorrespondences", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/client/correspondences.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListClientCorrespondences",
        )

    def get(self, *, correspondence_id: int) -> dict[str, Any]:
        """Get a single client correspondence by id.

        Args:
            correspondence_id: The correspondence id.
        """
        return self._request(
            OperationInfo(
                service="clientcorrespondences", operation="get", is_mutation=False, resource_id=correspondence_id
            ),
            "GET",
            f"/client/correspondences/{correspondence_id}",
            operation="GetClientCorrespondence",
        )


class AsyncClientCorrespondencesService(AsyncBaseService):
    async def list(
        self,
        *,
        bucket_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List all client correspondences in a project.

        Args:
            bucket_id: The bucket id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="clientcorrespondences", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/client/correspondences.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListClientCorrespondences",
        )

    async def get(self, *, correspondence_id: int) -> dict[str, Any]:
        """Get a single client correspondence by id.

        Args:
            correspondence_id: The correspondence id.
        """
        return await self._request(
            OperationInfo(
                service="clientcorrespondences", operation="get", is_mutation=False, resource_id=correspondence_id
            ),
            "GET",
            f"/client/correspondences/{correspondence_id}",
            operation="GetClientCorrespondence",
        )
