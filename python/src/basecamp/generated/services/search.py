# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class SearchService(BaseService):
    def search(self, *, q: str, sort: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="search", operation="search", is_mutation=False),
            "/search.json",
            params=self._compact(q=q, sort=sort),
        )

    def metadata(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="search", operation="metadata", is_mutation=False), "GET", "/searches/metadata.json"
        )


class AsyncSearchService(AsyncBaseService):
    async def search(self, *, q: str, sort: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="search", operation="search", is_mutation=False),
            "/search.json",
            params=self._compact(q=q, sort=sort),
        )

    async def metadata(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="search", operation="metadata", is_mutation=False), "GET", "/searches/metadata.json"
        )
