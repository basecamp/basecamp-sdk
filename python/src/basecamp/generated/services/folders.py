# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class FoldersService(BaseService):
    def list_folders(self) -> ListResult:
        return self._request_list(
            OperationInfo(service="folders", operation="list_folders", is_mutation=False),
            "/stacks.json",
            operation="ListFolders",
        )

    def create_folder(self, *, name: str | None = None, project_ids: list[int] | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="folders", operation="create_folder", is_mutation=True),
            "POST",
            "/stacks.json",
            json_body=self._compact(name=name, project_ids=project_ids),
            operation="CreateFolder",
        )

    def get_folder(self, *, folder_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="folders", operation="get_folder", is_mutation=False, resource_id=folder_id),
            "GET",
            f"/stacks/{folder_id}",
            operation="GetFolder",
        )

    def update_folder(self, *, folder_id: int, name: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="folders", operation="update_folder", is_mutation=True, resource_id=folder_id),
            "PUT",
            f"/stacks/{folder_id}",
            json_body=self._compact(name=name),
            operation="UpdateFolder",
        )

    def delete_folder(self, *, folder_id: int) -> None:
        self._request_void(
            OperationInfo(service="folders", operation="delete_folder", is_mutation=True, resource_id=folder_id),
            "DELETE",
            f"/stacks/{folder_id}",
            operation="DeleteFolder",
        )


class AsyncFoldersService(AsyncBaseService):
    async def list_folders(self) -> ListResult:
        return await self._request_list(
            OperationInfo(service="folders", operation="list_folders", is_mutation=False),
            "/stacks.json",
            operation="ListFolders",
        )

    async def create_folder(self, *, name: str | None = None, project_ids: list[int] | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="folders", operation="create_folder", is_mutation=True),
            "POST",
            "/stacks.json",
            json_body=self._compact(name=name, project_ids=project_ids),
            operation="CreateFolder",
        )

    async def get_folder(self, *, folder_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="folders", operation="get_folder", is_mutation=False, resource_id=folder_id),
            "GET",
            f"/stacks/{folder_id}",
            operation="GetFolder",
        )

    async def update_folder(self, *, folder_id: int, name: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="folders", operation="update_folder", is_mutation=True, resource_id=folder_id),
            "PUT",
            f"/stacks/{folder_id}",
            json_body=self._compact(name=name),
            operation="UpdateFolder",
        )

    async def delete_folder(self, *, folder_id: int) -> None:
        await self._request_void(
            OperationInfo(service="folders", operation="delete_folder", is_mutation=True, resource_id=folder_id),
            "DELETE",
            f"/stacks/{folder_id}",
            operation="DeleteFolder",
        )
