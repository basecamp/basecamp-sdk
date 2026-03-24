# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CampfiresService(BaseService):
    def list(self) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="campfires", operation="list", is_mutation=False), "/chats.json"
        )

    def get(self, *, campfire_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="campfires", operation="get", is_mutation=False, resource_id=campfire_id),
            "GET",
            f"/chats/{campfire_id}",
        )

    def list_chatbots(self, *, campfire_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="campfires", operation="list_chatbots", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/integrations.json",
        )

    def create_chatbot(
        self, *, campfire_id: int | str, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="campfires", operation="create_chatbot", is_mutation=True, resource_id=campfire_id),
            "POST",
            f"/chats/{campfire_id}/integrations.json",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="CreateChatbot",
        )

    def get_chatbot(self, *, campfire_id: int | str, chatbot_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="campfires", operation="get_chatbot", is_mutation=False, resource_id=chatbot_id),
            "GET",
            f"/chats/{campfire_id}/integrations/{chatbot_id}",
        )

    def update_chatbot(
        self, *, campfire_id: int | str, chatbot_id: int | str, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="campfires", operation="update_chatbot", is_mutation=True, resource_id=chatbot_id),
            "PUT",
            f"/chats/{campfire_id}/integrations/{chatbot_id}",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="UpdateChatbot",
        )

    def delete_chatbot(self, *, campfire_id: int | str, chatbot_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="campfires", operation="delete_chatbot", is_mutation=True, resource_id=chatbot_id),
            "DELETE",
            f"/chats/{campfire_id}/integrations/{chatbot_id}",
            operation="DeleteChatbot",
        )

    def list_lines(
        self, *, campfire_id: int | str, sort: str | None = None, direction: str | None = None
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="campfires", operation="list_lines", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/lines.json",
            params=self._compact(sort=sort, direction=direction),
        )

    def create_line(self, *, campfire_id: int | str, content: str, content_type: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="campfires", operation="create_line", is_mutation=True, resource_id=campfire_id),
            "POST",
            f"/chats/{campfire_id}/lines.json",
            json_body=self._compact(content=content, content_type=content_type),
            operation="CreateCampfireLine",
        )

    def get_line(self, *, campfire_id: int | str, line_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="campfires", operation="get_line", is_mutation=False, resource_id=line_id),
            "GET",
            f"/chats/{campfire_id}/lines/{line_id}",
        )

    def delete_line(self, *, campfire_id: int | str, line_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="campfires", operation="delete_line", is_mutation=True, resource_id=line_id),
            "DELETE",
            f"/chats/{campfire_id}/lines/{line_id}",
            operation="DeleteCampfireLine",
        )

    def list_uploads(
        self, *, campfire_id: int | str, sort: str | None = None, direction: str | None = None
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="campfires", operation="list_uploads", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            params=self._compact(sort=sort, direction=direction),
        )

    def create_upload(self, *, campfire_id: int | str, content: bytes, content_type: str, name: str) -> dict[str, Any]:
        return self._request_raw(
            OperationInfo(service="campfires", operation="create_upload", is_mutation=True, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            content=content,
            content_type=content_type,
            params=self._compact(name=name),
            operation="CreateCampfireUpload",
        )


class AsyncCampfiresService(AsyncBaseService):
    async def list(self) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="campfires", operation="list", is_mutation=False), "/chats.json"
        )

    async def get(self, *, campfire_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="campfires", operation="get", is_mutation=False, resource_id=campfire_id),
            "GET",
            f"/chats/{campfire_id}",
        )

    async def list_chatbots(self, *, campfire_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="campfires", operation="list_chatbots", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/integrations.json",
        )

    async def create_chatbot(
        self, *, campfire_id: int | str, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="campfires", operation="create_chatbot", is_mutation=True, resource_id=campfire_id),
            "POST",
            f"/chats/{campfire_id}/integrations.json",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="CreateChatbot",
        )

    async def get_chatbot(self, *, campfire_id: int | str, chatbot_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="campfires", operation="get_chatbot", is_mutation=False, resource_id=chatbot_id),
            "GET",
            f"/chats/{campfire_id}/integrations/{chatbot_id}",
        )

    async def update_chatbot(
        self, *, campfire_id: int | str, chatbot_id: int | str, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="campfires", operation="update_chatbot", is_mutation=True, resource_id=chatbot_id),
            "PUT",
            f"/chats/{campfire_id}/integrations/{chatbot_id}",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="UpdateChatbot",
        )

    async def delete_chatbot(self, *, campfire_id: int | str, chatbot_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="campfires", operation="delete_chatbot", is_mutation=True, resource_id=chatbot_id),
            "DELETE",
            f"/chats/{campfire_id}/integrations/{chatbot_id}",
            operation="DeleteChatbot",
        )

    async def list_lines(
        self, *, campfire_id: int | str, sort: str | None = None, direction: str | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="campfires", operation="list_lines", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/lines.json",
            params=self._compact(sort=sort, direction=direction),
        )

    async def create_line(
        self, *, campfire_id: int | str, content: str, content_type: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="campfires", operation="create_line", is_mutation=True, resource_id=campfire_id),
            "POST",
            f"/chats/{campfire_id}/lines.json",
            json_body=self._compact(content=content, content_type=content_type),
            operation="CreateCampfireLine",
        )

    async def get_line(self, *, campfire_id: int | str, line_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="campfires", operation="get_line", is_mutation=False, resource_id=line_id),
            "GET",
            f"/chats/{campfire_id}/lines/{line_id}",
        )

    async def delete_line(self, *, campfire_id: int | str, line_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="campfires", operation="delete_line", is_mutation=True, resource_id=line_id),
            "DELETE",
            f"/chats/{campfire_id}/lines/{line_id}",
            operation="DeleteCampfireLine",
        )

    async def list_uploads(
        self, *, campfire_id: int | str, sort: str | None = None, direction: str | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="campfires", operation="list_uploads", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            params=self._compact(sort=sort, direction=direction),
        )

    async def create_upload(
        self, *, campfire_id: int | str, content: bytes, content_type: str, name: str
    ) -> dict[str, Any]:
        return await self._request_raw(
            OperationInfo(service="campfires", operation="create_upload", is_mutation=True, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            content=content,
            content_type=content_type,
            params=self._compact(name=name),
            operation="CreateCampfireUpload",
        )
