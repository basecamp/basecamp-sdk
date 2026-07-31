# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class DraftsService(BaseService):
    def list_my_drafts(self, *, page: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="drafts", operation="list_my_drafts", is_mutation=False),
            "/my/drafts.json",
            params=self._compact(page=page),
            operation="ListMyDrafts",
        )


class AsyncDraftsService(AsyncBaseService):
    async def list_my_drafts(self, *, page: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="drafts", operation="list_my_drafts", is_mutation=False),
            "/my/drafts.json",
            params=self._compact(page=page),
            operation="ListMyDrafts",
        )
