# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CloudFilesService(BaseService):
    def create_cloud_file(
        self,
        *,
        bucket_id: int,
        vault_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new cloud file in a vault.
        `url` is validated against the selected service's patterns, so it must be a
        real link for that service; use service "other" for anything that matches no
        recognized service. Omitting `title` is allowed and reads back as
        "Untitled".

        Args:
            bucket_id: The bucket id.
            vault_id: The vault id.
            url: The url.
            service: Short identifier for the external service — "dropbox", "google_doc", "figma",
                "other", … Derived from the CloudFile::Service subclass name, so it is always
                present. `other` accepts any well-formed HTTPS URL.
            title: The title.
            description: The description.
            subscriptions: The subscriptions.
            visible_to_clients: Whether the cloud file is visible to the project's clients. Applies
                only when creating directly in the tool's vault — an item created inside a folder
                inherits the folder's visibility and ignores this. A client caller always creates
                client-visible records regardless of what is sent.
        """
        return self._request(
            OperationInfo(
                service="cloudfiles",
                operation="create_cloud_file",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=vault_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/vaults/{vault_id}/cloud_files.json",
            json_body=self._compact(
                url=url,
                service=service,
                title=title,
                description=description,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateCloudFile",
        )

    def get_cloud_file(self, *, cloud_file_id: int) -> dict[str, Any]:
        """Get a single cloud file by id.

        Args:
            cloud_file_id: The cloud file id.
        """
        return self._request(
            OperationInfo(
                service="cloudfiles", operation="get_cloud_file", is_mutation=False, resource_id=cloud_file_id
            ),
            "GET",
            f"/cloud_files/{cloud_file_id}",
            operation="GetCloudFile",
        )

    def update_cloud_file(
        self,
        *,
        cloud_file_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Replace a cloud file with a new complete representation.
        BC3 builds a brand-new CloudFile from the permitted params and swaps the
        recordable wholesale, so the request body is the full writable state:
        omitting `title` clears it (and the cloud file then reads back as
        "Untitled"), and omitting `description` clears it. `url` and `service` carry
        presence/format validations, so they are required here rather than
        clearable — a request without them is a 422, not a silent wipe.
        Updating a drafted cloud file also publishes it.
        Subscribers are the exception to omission-clears: a drafted cloud file keeps
        its current subscribers when the request addresses neither `subscriptions`
        nor `notify`. The creator is always on the list.
        The legacy bucket-scoped path PUT /buckets/{bucketId}/cloud_files/{id}.json
        is also accepted by BC3; this flat spelling is the documented one.

        Args:
            cloud_file_id: The cloud file id.
            url: The url.
            service: Short identifier for the external service — "dropbox", "google_doc", "figma",
                "other", … Derived from the CloudFile::Service subclass name, so it is always
                present. `other` accepts any well-formed HTTPS URL.
            title: The title.
            description: The description.
            subscriptions: The subscriptions.
        """
        return self._request(
            OperationInfo(
                service="cloudfiles", operation="update_cloud_file", is_mutation=True, resource_id=cloud_file_id
            ),
            "PUT",
            f"/cloud_files/{cloud_file_id}",
            json_body=self._compact(
                url=url, service=service, title=title, description=description, subscriptions=subscriptions
            ),
            operation="UpdateCloudFile",
        )


class AsyncCloudFilesService(AsyncBaseService):
    async def create_cloud_file(
        self,
        *,
        bucket_id: int,
        vault_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
        visible_to_clients: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new cloud file in a vault.
        `url` is validated against the selected service's patterns, so it must be a
        real link for that service; use service "other" for anything that matches no
        recognized service. Omitting `title` is allowed and reads back as
        "Untitled".

        Args:
            bucket_id: The bucket id.
            vault_id: The vault id.
            url: The url.
            service: Short identifier for the external service — "dropbox", "google_doc", "figma",
                "other", … Derived from the CloudFile::Service subclass name, so it is always
                present. `other` accepts any well-formed HTTPS URL.
            title: The title.
            description: The description.
            subscriptions: The subscriptions.
            visible_to_clients: Whether the cloud file is visible to the project's clients. Applies
                only when creating directly in the tool's vault — an item created inside a folder
                inherits the folder's visibility and ignores this. A client caller always creates
                client-visible records regardless of what is sent.
        """
        return await self._request(
            OperationInfo(
                service="cloudfiles",
                operation="create_cloud_file",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=vault_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/vaults/{vault_id}/cloud_files.json",
            json_body=self._compact(
                url=url,
                service=service,
                title=title,
                description=description,
                subscriptions=subscriptions,
                visible_to_clients=visible_to_clients,
            ),
            operation="CreateCloudFile",
        )

    async def get_cloud_file(self, *, cloud_file_id: int) -> dict[str, Any]:
        """Get a single cloud file by id.

        Args:
            cloud_file_id: The cloud file id.
        """
        return await self._request(
            OperationInfo(
                service="cloudfiles", operation="get_cloud_file", is_mutation=False, resource_id=cloud_file_id
            ),
            "GET",
            f"/cloud_files/{cloud_file_id}",
            operation="GetCloudFile",
        )

    async def update_cloud_file(
        self,
        *,
        cloud_file_id: int,
        url: str,
        service: str,
        title: str | None = None,
        description: str | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Replace a cloud file with a new complete representation.
        BC3 builds a brand-new CloudFile from the permitted params and swaps the
        recordable wholesale, so the request body is the full writable state:
        omitting `title` clears it (and the cloud file then reads back as
        "Untitled"), and omitting `description` clears it. `url` and `service` carry
        presence/format validations, so they are required here rather than
        clearable — a request without them is a 422, not a silent wipe.
        Updating a drafted cloud file also publishes it.
        Subscribers are the exception to omission-clears: a drafted cloud file keeps
        its current subscribers when the request addresses neither `subscriptions`
        nor `notify`. The creator is always on the list.
        The legacy bucket-scoped path PUT /buckets/{bucketId}/cloud_files/{id}.json
        is also accepted by BC3; this flat spelling is the documented one.

        Args:
            cloud_file_id: The cloud file id.
            url: The url.
            service: Short identifier for the external service — "dropbox", "google_doc", "figma",
                "other", … Derived from the CloudFile::Service subclass name, so it is always
                present. `other` accepts any well-formed HTTPS URL.
            title: The title.
            description: The description.
            subscriptions: The subscriptions.
        """
        return await self._request(
            OperationInfo(
                service="cloudfiles", operation="update_cloud_file", is_mutation=True, resource_id=cloud_file_id
            ),
            "PUT",
            f"/cloud_files/{cloud_file_id}",
            json_body=self._compact(
                url=url, service=service, title=title, description=description, subscriptions=subscriptions
            ),
            operation="UpdateCloudFile",
        )
