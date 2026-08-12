# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ToolsService(BaseService):
    def create(
        self, *, bucket_id: int, tool_type: str, title: str | None = None, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Create a tool in a project dock.

        Args:
            bucket_id: The bucket id.
            tool_type: Tool type to add to the project dock. Values:
                Chat::Transcript|Inbox|Kanban::Board|Message::Board|Questionnaire|Schedule|Todoset|Vault.
            title: Title for the new tool. When omitted, Basecamp assigns the next available default
                title for the tool type.
            visible_to_clients: Create the tool already visible to clients. Honored only for tool
                types that manage their own client visibility (Chat::Transcript, Kanban::Board),
                which otherwise start hidden; every other tool type ignores it and inherits the
                project default.
        """
        return self._request(
            OperationInfo(service="tools", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/dock/tools.json",
            json_body=self._compact(tool_type=tool_type, title=title, visible_to_clients=visible_to_clients),
            operation="CreateTool",
        )

    def get(self, *, tool_id: int) -> dict[str, Any]:
        """Get a dock tool by id.

        Args:
            tool_id: The tool id.
        """
        return self._request(
            OperationInfo(service="tools", operation="get", is_mutation=False, resource_id=tool_id),
            "GET",
            f"/dock/tools/{tool_id}",
            operation="GetTool",
        )

    def update(self, *, tool_id: int, title: str) -> dict[str, Any]:
        """Update (rename) an existing tool.

        Args:
            tool_id: The tool id.
            title: The title.
        """
        return self._request(
            OperationInfo(service="tools", operation="update", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/dock/tools/{tool_id}",
            json_body=self._compact(title=title),
            operation="UpdateTool",
        )

    def delete(self, *, tool_id: int) -> None:
        """Delete a tool (trash it).

        Args:
            tool_id: The tool id.
        """
        self._request_void(
            OperationInfo(service="tools", operation="delete", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/dock/tools/{tool_id}",
            operation="DeleteTool",
        )

    def enable(self, *, tool_id: int) -> None:
        """Enable a tool (show it on the project dock).

        Args:
            tool_id: The tool id.
        """
        self._request_void(
            OperationInfo(service="tools", operation="enable", is_mutation=True, resource_id=tool_id),
            "POST",
            f"/recordings/{tool_id}/position.json",
            operation="EnableTool",
        )

    def reposition(self, *, tool_id: int, position: int) -> None:
        """Reposition a tool on the project dock.

        Args:
            tool_id: The tool id.
            position: The position.
        """
        self._request_void(
            OperationInfo(service="tools", operation="reposition", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/recordings/{tool_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTool",
        )

    def disable(self, *, tool_id: int) -> None:
        """Disable a tool (hide it from the project dock).

        Args:
            tool_id: The tool id.
        """
        self._request_void(
            OperationInfo(service="tools", operation="disable", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/recordings/{tool_id}/position.json",
            operation="DisableTool",
        )


class AsyncToolsService(AsyncBaseService):
    async def create(
        self, *, bucket_id: int, tool_type: str, title: str | None = None, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Create a tool in a project dock.

        Args:
            bucket_id: The bucket id.
            tool_type: Tool type to add to the project dock. Values:
                Chat::Transcript|Inbox|Kanban::Board|Message::Board|Questionnaire|Schedule|Todoset|Vault.
            title: Title for the new tool. When omitted, Basecamp assigns the next available default
                title for the tool type.
            visible_to_clients: Create the tool already visible to clients. Honored only for tool
                types that manage their own client visibility (Chat::Transcript, Kanban::Board),
                which otherwise start hidden; every other tool type ignores it and inherits the
                project default.
        """
        return await self._request(
            OperationInfo(service="tools", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/dock/tools.json",
            json_body=self._compact(tool_type=tool_type, title=title, visible_to_clients=visible_to_clients),
            operation="CreateTool",
        )

    async def get(self, *, tool_id: int) -> dict[str, Any]:
        """Get a dock tool by id.

        Args:
            tool_id: The tool id.
        """
        return await self._request(
            OperationInfo(service="tools", operation="get", is_mutation=False, resource_id=tool_id),
            "GET",
            f"/dock/tools/{tool_id}",
            operation="GetTool",
        )

    async def update(self, *, tool_id: int, title: str) -> dict[str, Any]:
        """Update (rename) an existing tool.

        Args:
            tool_id: The tool id.
            title: The title.
        """
        return await self._request(
            OperationInfo(service="tools", operation="update", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/dock/tools/{tool_id}",
            json_body=self._compact(title=title),
            operation="UpdateTool",
        )

    async def delete(self, *, tool_id: int) -> None:
        """Delete a tool (trash it).

        Args:
            tool_id: The tool id.
        """
        await self._request_void(
            OperationInfo(service="tools", operation="delete", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/dock/tools/{tool_id}",
            operation="DeleteTool",
        )

    async def enable(self, *, tool_id: int) -> None:
        """Enable a tool (show it on the project dock).

        Args:
            tool_id: The tool id.
        """
        await self._request_void(
            OperationInfo(service="tools", operation="enable", is_mutation=True, resource_id=tool_id),
            "POST",
            f"/recordings/{tool_id}/position.json",
            operation="EnableTool",
        )

    async def reposition(self, *, tool_id: int, position: int) -> None:
        """Reposition a tool on the project dock.

        Args:
            tool_id: The tool id.
            position: The position.
        """
        await self._request_void(
            OperationInfo(service="tools", operation="reposition", is_mutation=True, resource_id=tool_id),
            "PUT",
            f"/recordings/{tool_id}/position.json",
            json_body=self._compact(position=position),
            operation="RepositionTool",
        )

    async def disable(self, *, tool_id: int) -> None:
        """Disable a tool (hide it from the project dock).

        Args:
            tool_id: The tool id.
        """
        await self._request_void(
            OperationInfo(service="tools", operation="disable", is_mutation=True, resource_id=tool_id),
            "DELETE",
            f"/recordings/{tool_id}/position.json",
            operation="DisableTool",
        )
