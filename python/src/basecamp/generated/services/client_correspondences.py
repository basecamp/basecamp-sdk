# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ClientCorrespondencesService(BaseService):
    def list(self, *, sort: str | None = None, direction: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="clientcorrespondences", operation="list", is_mutation=False),
            "/client/correspondences.json",
            params=self._compact(sort=sort, direction=direction),
        )

    def get(self, *, correspondence_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="clientcorrespondences", operation="get", is_mutation=False, resource_id=correspondence_id
            ),
            "GET",
            f"/client/correspondences/{correspondence_id}",
        )


class AsyncClientCorrespondencesService(AsyncBaseService):
    async def list(self, *, sort: str | None = None, direction: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="clientcorrespondences", operation="list", is_mutation=False),
            "/client/correspondences.json",
            params=self._compact(sort=sort, direction=direction),
        )

    async def get(self, *, correspondence_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="clientcorrespondences", operation="get", is_mutation=False, resource_id=correspondence_id
            ),
            "GET",
            f"/client/correspondences/{correspondence_id}",
        )
