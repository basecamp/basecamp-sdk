# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class FoldersService(BaseService):
    def list_folders(self) -> ListResult:
        """List the authenticated user's folders in home-screen order.

        Returns a bare array with no pagination envelope. Items are the base folder
        shape: they carry `bucket_ids` but **not** the expanded `projects`, which
        only the single-folder operations return.
        """
        return self._request_list(
            OperationInfo(service="folders", operation="list_folders", is_mutation=False),
            "/stacks.json",
            operation="ListFolders",
        )

    def create_folder(self, *, name: str | None = None, project_ids: list[int] | None = None) -> dict[str, Any]:
        """Create a folder for the authenticated user and file the given projects into it.

        Returns 201 with the new folder and its expanded `projects`, placed at the
        top of the home screen. Filing an all-access project the user has not joined
        **grants** them access to it. Every id is preflighted: if any is archived,
        trashed, or an invitation-only project the user is not on, the whole request
        fails with 404 and nothing is created — there is no partial success.

        Args:
            name: The folder's name. Defaults to `New folder` when blank, null, or omitted.
            project_ids: IDs of the projects to file into the folder — the same ids the folder
                reports back as `bucket_ids` and expands as `projects`. This does not round-trip
                under its own name. Omit it, or send null or an empty array, for an empty folder.
        """
        return self._request(
            OperationInfo(service="folders", operation="create_folder", is_mutation=True),
            "POST",
            "/stacks.json",
            json_body=self._compact(name=name, project_ids=project_ids),
            operation="CreateFolder",
        )

    def get_folder(self, *, folder_id: int) -> dict[str, Any]:
        """Get one folder, with the projects grouped inside it expanded under `projects`.

        Args:
            folder_id: The folder id.
        """
        return self._request(
            OperationInfo(service="folders", operation="get_folder", is_mutation=False, resource_id=folder_id),
            "GET",
            f"/stacks/{folder_id}",
            operation="GetFolder",
        )

    def update_folder(self, *, folder_id: int, name: str) -> dict[str, Any]:
        """Rename a folder.

        `name` is the only writable attribute; a folder's projects, ordering, and
        image are managed elsewhere and an image parameter sent here is ignored.

        Args:
            folder_id: The folder id.
            name: The folder's new name. Blank is rejected with 422 — unlike create, update does not
                fall back to a default name.
        """
        return self._request(
            OperationInfo(service="folders", operation="update_folder", is_mutation=True, resource_id=folder_id),
            "PUT",
            f"/stacks/{folder_id}",
            json_body=self._compact(name=name),
            operation="UpdateFolder",
        )

    def delete_folder(self, *, folder_id: int) -> None:
        """Delete a folder and unpin its projects from the home screen (returns 204 No Content).

        The projects themselves are not deleted and are not moved back out onto the
        home screen; they simply stop appearing there until pinned again.

        Args:
            folder_id: The folder id.
        """
        self._request_void(
            OperationInfo(service="folders", operation="delete_folder", is_mutation=True, resource_id=folder_id),
            "DELETE",
            f"/stacks/{folder_id}",
            operation="DeleteFolder",
        )


class AsyncFoldersService(AsyncBaseService):
    async def list_folders(self) -> ListResult:
        """List the authenticated user's folders in home-screen order.

        Returns a bare array with no pagination envelope. Items are the base folder
        shape: they carry `bucket_ids` but **not** the expanded `projects`, which
        only the single-folder operations return.
        """
        return await self._request_list(
            OperationInfo(service="folders", operation="list_folders", is_mutation=False),
            "/stacks.json",
            operation="ListFolders",
        )

    async def create_folder(self, *, name: str | None = None, project_ids: list[int] | None = None) -> dict[str, Any]:
        """Create a folder for the authenticated user and file the given projects into it.

        Returns 201 with the new folder and its expanded `projects`, placed at the
        top of the home screen. Filing an all-access project the user has not joined
        **grants** them access to it. Every id is preflighted: if any is archived,
        trashed, or an invitation-only project the user is not on, the whole request
        fails with 404 and nothing is created — there is no partial success.

        Args:
            name: The folder's name. Defaults to `New folder` when blank, null, or omitted.
            project_ids: IDs of the projects to file into the folder — the same ids the folder
                reports back as `bucket_ids` and expands as `projects`. This does not round-trip
                under its own name. Omit it, or send null or an empty array, for an empty folder.
        """
        return await self._request(
            OperationInfo(service="folders", operation="create_folder", is_mutation=True),
            "POST",
            "/stacks.json",
            json_body=self._compact(name=name, project_ids=project_ids),
            operation="CreateFolder",
        )

    async def get_folder(self, *, folder_id: int) -> dict[str, Any]:
        """Get one folder, with the projects grouped inside it expanded under `projects`.

        Args:
            folder_id: The folder id.
        """
        return await self._request(
            OperationInfo(service="folders", operation="get_folder", is_mutation=False, resource_id=folder_id),
            "GET",
            f"/stacks/{folder_id}",
            operation="GetFolder",
        )

    async def update_folder(self, *, folder_id: int, name: str) -> dict[str, Any]:
        """Rename a folder.

        `name` is the only writable attribute; a folder's projects, ordering, and
        image are managed elsewhere and an image parameter sent here is ignored.

        Args:
            folder_id: The folder id.
            name: The folder's new name. Blank is rejected with 422 — unlike create, update does not
                fall back to a default name.
        """
        return await self._request(
            OperationInfo(service="folders", operation="update_folder", is_mutation=True, resource_id=folder_id),
            "PUT",
            f"/stacks/{folder_id}",
            json_body=self._compact(name=name),
            operation="UpdateFolder",
        )

    async def delete_folder(self, *, folder_id: int) -> None:
        """Delete a folder and unpin its projects from the home screen (returns 204 No Content).

        The projects themselves are not deleted and are not moved back out onto the
        home screen; they simply stop appearing there until pinned again.

        Args:
            folder_id: The folder id.
        """
        await self._request_void(
            OperationInfo(service="folders", operation="delete_folder", is_mutation=True, resource_id=folder_id),
            "DELETE",
            f"/stacks/{folder_id}",
            operation="DeleteFolder",
        )
