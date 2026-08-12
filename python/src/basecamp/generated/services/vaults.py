# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class VaultsService(BaseService):
    def get(self, *, vault_id: int) -> dict[str, Any]:
        """Get a single vault by id.

        Args:
            vault_id: The vault id.
        """
        return self._request(
            OperationInfo(service="vaults", operation="get", is_mutation=False, resource_id=vault_id),
            "GET",
            f"/vaults/{vault_id}",
            operation="GetVault",
        )

    def update(self, *, vault_id: int, title: str | None = None) -> dict[str, Any]:
        """Update an existing vault.

        Args:
            vault_id: The vault id.
            title: The title.
        """
        return self._request(
            OperationInfo(service="vaults", operation="update", is_mutation=True, resource_id=vault_id),
            "PUT",
            f"/vaults/{vault_id}",
            json_body=self._compact(title=title),
            operation="UpdateVault",
        )

    def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List vaults (subfolders) in a vault.

        Args:
            vault_id: The vault id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="vaults", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/vaults.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListVaults",
        )

    def create(self, *, vault_id: int, title: str) -> dict[str, Any]:
        """Create a new vault (subfolder) in a vault.

        Args:
            vault_id: The vault id.
            title: The title.
        """
        return self._request(
            OperationInfo(service="vaults", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/vaults.json",
            json_body=self._compact(title=title),
            operation="CreateVault",
        )


class AsyncVaultsService(AsyncBaseService):
    async def get(self, *, vault_id: int) -> dict[str, Any]:
        """Get a single vault by id.

        Args:
            vault_id: The vault id.
        """
        return await self._request(
            OperationInfo(service="vaults", operation="get", is_mutation=False, resource_id=vault_id),
            "GET",
            f"/vaults/{vault_id}",
            operation="GetVault",
        )

    async def update(self, *, vault_id: int, title: str | None = None) -> dict[str, Any]:
        """Update an existing vault.

        Args:
            vault_id: The vault id.
            title: The title.
        """
        return await self._request(
            OperationInfo(service="vaults", operation="update", is_mutation=True, resource_id=vault_id),
            "PUT",
            f"/vaults/{vault_id}",
            json_body=self._compact(title=title),
            operation="UpdateVault",
        )

    async def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List vaults (subfolders) in a vault.

        Args:
            vault_id: The vault id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="vaults", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/vaults.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListVaults",
        )

    async def create(self, *, vault_id: int, title: str) -> dict[str, Any]:
        """Create a new vault (subfolder) in a vault.

        Args:
            vault_id: The vault id.
            title: The title.
        """
        return await self._request(
            OperationInfo(service="vaults", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/vaults.json",
            json_body=self._compact(title=title),
            operation="CreateVault",
        )
