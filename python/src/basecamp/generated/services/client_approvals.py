# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ClientApprovalsService(BaseService):
    def list(self, *, sort: str | None = None, direction: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="clientapprovals", operation="list", is_mutation=False),
            "/client/approvals.json",
            params=self._compact(sort=sort, direction=direction),
        )

    def get(self, *, approval_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="clientapprovals", operation="get", is_mutation=False, resource_id=approval_id),
            "GET",
            f"/client/approvals/{approval_id}",
        )


class AsyncClientApprovalsService(AsyncBaseService):
    async def list(self, *, sort: str | None = None, direction: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="clientapprovals", operation="list", is_mutation=False),
            "/client/approvals.json",
            params=self._compact(sort=sort, direction=direction),
        )

    async def get(self, *, approval_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="clientapprovals", operation="get", is_mutation=False, resource_id=approval_id),
            "GET",
            f"/client/approvals/{approval_id}",
        )
