# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class UploadsService(BaseService):
    def get(self, *, upload_id: int) -> dict[str, Any]:
        """Get a single upload by id.

        Args:
            upload_id: The upload id.
        """
        return self._request(
            OperationInfo(service="uploads", operation="get", is_mutation=False, resource_id=upload_id),
            "GET",
            f"/uploads/{upload_id}",
            operation="GetUpload",
        )

    def update(self, *, upload_id: int, description: str | None = None, base_name: str | None = None) -> dict[str, Any]:
        """Update an existing upload.

        Args:
            upload_id: The upload id.
            description: The description.
            base_name: The base name.
        """
        return self._request(
            OperationInfo(service="uploads", operation="update", is_mutation=True, resource_id=upload_id),
            "PUT",
            f"/uploads/{upload_id}",
            json_body=self._compact(description=description, base_name=base_name),
            operation="UpdateUpload",
        )

    def list_versions(self, *, upload_id: int, max_items: int | None = None) -> ListResult:
        """List versions of an upload.

        Args:
            upload_id: The upload id.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="uploads", operation="list_versions", is_mutation=False, resource_id=upload_id),
            f"/uploads/{upload_id}/versions.json",
            max_items=max_items,
            operation="ListUploadVersions",
        )

    def create_version(
        self,
        *,
        upload_id: int,
        attachable_sgid: str,
        base_name: str | None = None,
        description: str | None = None,
        notify: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Replace an upload's file with a new version.

        The recording keeps its id, its URL and its comments; the previous file becomes a
        past version. Use this instead of CreateUpload when publishing a new release of the
        same file, so its published link keeps working.

        Args:
            upload_id: The upload id.
            attachable_sgid: The attachable sgid.
            base_name: Omit to keep the uploaded file's own name. Sending "" also keeps it.
            description: Presence-aware: omit to carry the previous version's description forward,
                send "" to clear it, send a value to set it.
            notify: Who to notify: "default", "everyone", or "custom" (the people in subscriptions).
                Omit both this and subscriptions to notify nobody. A subscriptions array sent
                without notify is read as "custom".
            subscriptions: People to notify about the replacement and subscribe to the upload.
        """
        return self._request(
            OperationInfo(service="uploads", operation="create_version", is_mutation=True, resource_id=upload_id),
            "POST",
            f"/uploads/{upload_id}/versions.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                base_name=base_name,
                description=description,
                notify=notify,
                subscriptions=subscriptions,
            ),
            operation="CreateUploadVersion",
        )

    def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List uploads in a vault.

        Args:
            vault_id: The vault id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="uploads", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/uploads.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListUploads",
        )

    def create(
        self,
        *,
        vault_id: int,
        attachable_sgid: str,
        description: str | None = None,
        base_name: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new upload in a vault.

        Args:
            vault_id: The vault id.
            attachable_sgid: The attachable sgid.
            description: The description.
            base_name: The base name.
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return self._request(
            OperationInfo(service="uploads", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/uploads.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                description=description,
                base_name=base_name,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateUpload",
        )


class AsyncUploadsService(AsyncBaseService):
    async def get(self, *, upload_id: int) -> dict[str, Any]:
        """Get a single upload by id.

        Args:
            upload_id: The upload id.
        """
        return await self._request(
            OperationInfo(service="uploads", operation="get", is_mutation=False, resource_id=upload_id),
            "GET",
            f"/uploads/{upload_id}",
            operation="GetUpload",
        )

    async def update(
        self, *, upload_id: int, description: str | None = None, base_name: str | None = None
    ) -> dict[str, Any]:
        """Update an existing upload.

        Args:
            upload_id: The upload id.
            description: The description.
            base_name: The base name.
        """
        return await self._request(
            OperationInfo(service="uploads", operation="update", is_mutation=True, resource_id=upload_id),
            "PUT",
            f"/uploads/{upload_id}",
            json_body=self._compact(description=description, base_name=base_name),
            operation="UpdateUpload",
        )

    async def list_versions(self, *, upload_id: int, max_items: int | None = None) -> ListResult:
        """List versions of an upload.

        Args:
            upload_id: The upload id.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="uploads", operation="list_versions", is_mutation=False, resource_id=upload_id),
            f"/uploads/{upload_id}/versions.json",
            max_items=max_items,
            operation="ListUploadVersions",
        )

    async def create_version(
        self,
        *,
        upload_id: int,
        attachable_sgid: str,
        base_name: str | None = None,
        description: str | None = None,
        notify: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Replace an upload's file with a new version.

        The recording keeps its id, its URL and its comments; the previous file becomes a
        past version. Use this instead of CreateUpload when publishing a new release of the
        same file, so its published link keeps working.

        Args:
            upload_id: The upload id.
            attachable_sgid: The attachable sgid.
            base_name: Omit to keep the uploaded file's own name. Sending "" also keeps it.
            description: Presence-aware: omit to carry the previous version's description forward,
                send "" to clear it, send a value to set it.
            notify: Who to notify: "default", "everyone", or "custom" (the people in subscriptions).
                Omit both this and subscriptions to notify nobody. A subscriptions array sent
                without notify is read as "custom".
            subscriptions: People to notify about the replacement and subscribe to the upload.
        """
        return await self._request(
            OperationInfo(service="uploads", operation="create_version", is_mutation=True, resource_id=upload_id),
            "POST",
            f"/uploads/{upload_id}/versions.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                base_name=base_name,
                description=description,
                notify=notify,
                subscriptions=subscriptions,
            ),
            operation="CreateUploadVersion",
        )

    async def list(self, *, vault_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List uploads in a vault.

        Args:
            vault_id: The vault id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="uploads", operation="list", is_mutation=False, resource_id=vault_id),
            f"/vaults/{vault_id}/uploads.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListUploads",
        )

    async def create(
        self,
        *,
        vault_id: int,
        attachable_sgid: str,
        description: str | None = None,
        base_name: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new upload in a vault.

        Args:
            vault_id: The vault id.
            attachable_sgid: The attachable sgid.
            description: The description.
            base_name: The base name.
            subscriptions: The subscriptions.
            visible_to_clients: The visible to clients.
        """
        return await self._request(
            OperationInfo(service="uploads", operation="create", is_mutation=True, resource_id=vault_id),
            "POST",
            f"/vaults/{vault_id}/uploads.json",
            json_body=self._compact(
                attachable_sgid=attachable_sgid,
                description=description,
                base_name=base_name,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateUpload",
        )
