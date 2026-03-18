# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ToolsService(BaseService):
    def clone(self, *, source_recording_id: int, title: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="tools", operation="clone", is_mutation=True),
            "POST",
            "/dock/tools.json",
            json_body=self._compact(source_recording_id=source_recording_id, title=title),
            operation="CloneTool",
        )

    def get(self, *, tool_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="tools", operation="get", is_mutation=False, resource_id=tool_id),
            "GET",
            f"/dock/tools/{tool_id}",
        )

    def update(self, *, tool_id: int | str, title: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="tools", operation="update", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/dock/tools/{tool_id}",
            json_body=self._compact(title=title),
            operation="UpdateTool",
        )

    def delete(self, *, tool_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="tools", operation="delete", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/dock/tools/{tool_id}",
            operation="DeleteTool",
        )

    def enable(self, *, tool_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="tools", operation="enable", is_mutation=True, resource_id=tool_id),
            "POST",
            f"/recordings/{tool_id}/position.json",
            operation="EnableTool",
        )

    def reposition(self, *, tool_id: int | str, position: int) -> None:
        self._request_void(
            OperationInfo(service="tools", operation="reposition", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/recordings/{tool_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTool",
        )

    def disable(self, *, tool_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="tools", operation="disable", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/recordings/{tool_id}/position.json",
            operation="DisableTool",
        )


class AsyncToolsService(AsyncBaseService):
    async def clone(self, *, source_recording_id: int, title: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="tools", operation="clone", is_mutation=True),
            "POST",
            "/dock/tools.json",
            json_body=self._compact(source_recording_id=source_recording_id, title=title),
            operation="CloneTool",
        )

    async def get(self, *, tool_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="tools", operation="get", is_mutation=False, resource_id=tool_id),
            "GET",
            f"/dock/tools/{tool_id}",
        )

    async def update(self, *, tool_id: int | str, title: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="tools", operation="update", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/dock/tools/{tool_id}",
            json_body=self._compact(title=title),
            operation="UpdateTool",
        )

    async def delete(self, *, tool_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="tools", operation="delete", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/dock/tools/{tool_id}",
            operation="DeleteTool",
        )

    async def enable(self, *, tool_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="tools", operation="enable", is_mutation=True, resource_id=tool_id),
            "POST",
            f"/recordings/{tool_id}/position.json",
            operation="EnableTool",
        )

    async def reposition(self, *, tool_id: int | str, position: int) -> None:
        await self._request_void(
            OperationInfo(service="tools", operation="reposition", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/recordings/{tool_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTool",
        )

    async def disable(self, *, tool_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="tools", operation="disable", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/recordings/{tool_id}/position.json",
            operation="DisableTool",
        )
