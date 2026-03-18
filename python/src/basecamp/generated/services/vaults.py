# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class VaultsService(BaseService):
    def get(self, *, vault_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="vaults", operation="get", is_mutation=False, resource_id=vault_id),
            "GET",
            f"/vaults/{vault_id}",
        )

    def update(self, *, vault_id: int | str, title: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="vaults", operation="update", is_mutation=True, resource_id=vault_id),
            "PUT",
            f"/vaults/{vault_id}",
            json_body=self._compact(title=title),
            operation="UpdateVault",
        )

    def list(self, *, vault_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="vaults", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/vaults.json",
        )

    def create(self, *, vault_id: int | str, title: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="vaults", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/vaults.json",
            json_body=self._compact(title=title),
            operation="CreateVault",
        )


class AsyncVaultsService(AsyncBaseService):
    async def get(self, *, vault_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="vaults", operation="get", is_mutation=False, resource_id=vault_id),
            "GET",
            f"/vaults/{vault_id}",
        )

    async def update(self, *, vault_id: int | str, title: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="vaults", operation="update", is_mutation=True, resource_id=vault_id),
            "PUT",
            f"/vaults/{vault_id}",
            json_body=self._compact(title=title),
            operation="UpdateVault",
        )

    async def list(self, *, vault_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="vaults", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/vaults.json",
        )

    async def create(self, *, vault_id: int | str, title: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="vaults", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/vaults.json",
            json_body=self._compact(title=title),
            operation="CreateVault",
        )
