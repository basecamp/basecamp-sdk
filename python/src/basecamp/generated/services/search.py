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
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Search for content across the account.

        Deprecated parameters (prefer the replacement):

        - type: prefer type_names[].
        - bucket_id: prefer bucket_ids[].
        - creator_id: prefer creator_ids[].

        Args:
            q: The q.
            type_names: Recording types to include. Use `key` values from the metadata endpoint's
                `recording_search_types`. Available since Basecamp 5.
            bucket_ids: Project IDs to filter by. Available since Basecamp 5.
            creator_ids: Creator person IDs to filter by. Available since Basecamp 5.
            file_type: Filter attachments by type. Use `key` values from the metadata endpoint's
                `file_search_types`.
            exclude_chat: Set to true to exclude chat results.
            since: last_7_days|last_30_days|last_90_days|last_12_months|forever
            sort: best_match|recency
            type: Deprecated: prefer type_names[].
            bucket_id: Deprecated: prefer bucket_ids[].
            creator_id: Deprecated: prefer creator_ids[].
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
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
                    "page": page,
                }.items()
                if v is not None
            },
            max_items=max_items,
            operation="Search",
        )

    def metadata(self) -> dict[str, Any]:
        """Get search metadata (available filter options)."""
        return self._request(
            OperationInfo(service="search", operation="metadata", is_mutation=False),
            "GET",
            "/searches/metadata.json",
            operation="GetSearchMetadata",
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
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Search for content across the account.

        Deprecated parameters (prefer the replacement):

        - type: prefer type_names[].
        - bucket_id: prefer bucket_ids[].
        - creator_id: prefer creator_ids[].

        Args:
            q: The q.
            type_names: Recording types to include. Use `key` values from the metadata endpoint's
                `recording_search_types`. Available since Basecamp 5.
            bucket_ids: Project IDs to filter by. Available since Basecamp 5.
            creator_ids: Creator person IDs to filter by. Available since Basecamp 5.
            file_type: Filter attachments by type. Use `key` values from the metadata endpoint's
                `file_search_types`.
            exclude_chat: Set to true to exclude chat results.
            since: last_7_days|last_30_days|last_90_days|last_12_months|forever
            sort: best_match|recency
            type: Deprecated: prefer type_names[].
            bucket_id: Deprecated: prefer bucket_ids[].
            creator_id: Deprecated: prefer creator_ids[].
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None means no
                item cap. Collection is always bounded by config.max_pages. A positive page argument
                fetches exactly that one page.
        """
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
                    "page": page,
                }.items()
                if v is not None
            },
            max_items=max_items,
            operation="Search",
        )

    async def metadata(self) -> dict[str, Any]:
        """Get search metadata (available filter options)."""
        return await self._request(
            OperationInfo(service="search", operation="metadata", is_mutation=False),
            "GET",
            "/searches/metadata.json",
            operation="GetSearchMetadata",
        )
