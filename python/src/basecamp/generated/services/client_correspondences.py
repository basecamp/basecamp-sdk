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
        return self._request_paginated(
            OperationInfo(service="clientcorrespondences", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/client/correspondences.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListClientCorrespondences",
        )

    def get(self, *, correspondence_id: int) -> dict[str, Any]:
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
        return await self._request_paginated(
            OperationInfo(service="clientcorrespondences", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/client/correspondences.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListClientCorrespondences",
        )

    async def get(self, *, correspondence_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="clientcorrespondences", operation="get", is_mutation=False, resource_id=correspondence_id
            ),
            "GET",
            f"/client/correspondences/{correspondence_id}",
            operation="GetClientCorrespondence",
        )
