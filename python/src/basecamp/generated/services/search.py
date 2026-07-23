# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class SearchService(BaseService):
    def search(
        self,
        *,
        q: str,
        type_names: list[str] | None = None,
        bucket_ids: list[int] | None = None,
        creator_ids: list[int] | None = None,
        file_type: str | None = None,
        exclude_chat: bool | None = None,
        since: str | None = None,
        sort: str | None = None,
        type: str | None = None,
        bucket_id: int | None = None,
        creator_id: int | None = None,
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="search", operation="search", is_mutation=False),
            "/search.json",
            params={
                k: v
                for k, v in {
                    "q": q,
                    "type_names[]": type_names,
                    "bucket_ids[]": bucket_ids,
                    "creator_ids[]": creator_ids,
                    "file_type": file_type,
                    "exclude_chat": exclude_chat,
                    "since": since,
                    "sort": sort,
                    "type": type,
                    "bucket_id": bucket_id,
                    "creator_id": creator_id,
                }.items()
                if v is not None
            },
        )

    def metadata(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="search", operation="metadata", is_mutation=False), "GET", "/searches/metadata.json"
        )


class AsyncSearchService(AsyncBaseService):
    async def search(
        self,
        *,
        q: str,
        type_names: list[str] | None = None,
        bucket_ids: list[int] | None = None,
        creator_ids: list[int] | None = None,
        file_type: str | None = None,
        exclude_chat: bool | None = None,
        since: str | None = None,
        sort: str | None = None,
        type: str | None = None,
        bucket_id: int | None = None,
        creator_id: int | None = None,
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="search", operation="search", is_mutation=False),
            "/search.json",
            params={
                k: v
                for k, v in {
                    "q": q,
                    "type_names[]": type_names,
                    "bucket_ids[]": bucket_ids,
                    "creator_ids[]": creator_ids,
                    "file_type": file_type,
                    "exclude_chat": exclude_chat,
                    "since": since,
                    "sort": sort,
                    "type": type,
                    "bucket_id": bucket_id,
                    "creator_id": creator_id,
                }.items()
                if v is not None
            },
        )

    async def metadata(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="search", operation="metadata", is_mutation=False), "GET", "/searches/metadata.json"
        )
