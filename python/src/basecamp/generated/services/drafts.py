# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class DraftsService(BaseService):
    def list_my_drafts(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List the current user's drafts across their active projects, most recently
        updated first (paginated, capped at 250 like /my/assignments). Five draft
        kinds are returned: messages, documents, uploads, client approvals, and
        client correspondences. Drafts under archived or trashed projects are
        excluded.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="drafts", operation="list_my_drafts", is_mutation=False),
            "/my/drafts.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListMyDrafts",
        )


class AsyncDraftsService(AsyncBaseService):
    async def list_my_drafts(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List the current user's drafts across their active projects, most recently
        updated first (paginated, capped at 250 like /my/assignments). Five draft
        kinds are returned: messages, documents, uploads, client approvals, and
        client correspondences. Drafts under archived or trashed projects are
        excluded.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="drafts", operation="list_my_drafts", is_mutation=False),
            "/my/drafts.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListMyDrafts",
        )
