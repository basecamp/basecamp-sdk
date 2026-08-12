# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CampfiresService(BaseService):
    def list_chatbots(self, *, bucket_id: int, campfire_id: int, max_items: int | None = None) -> ListResult:
        """List all chatbots for a campfire.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages.
        """
        return self._request_paginated(
            OperationInfo(
                service="campfires",
                operation="list_chatbots",
                is_mutation=False,
                project_id=bucket_id,
                resource_id=campfire_id,
            ),
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations.json",
            max_items=max_items,
            operation="ListChatbots",
        )

    def create_chatbot(
        self, *, bucket_id: int, campfire_id: int, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        """Create a new chatbot for a campfire.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            service_name: The service name.
            command_url: The command url.
        """
        return self._request(
            OperationInfo(
                service="campfires",
                operation="create_chatbot",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=campfire_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations.json",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="CreateChatbot",
        )

    def get_chatbot(self, *, bucket_id: int, campfire_id: int, chatbot_id: int) -> dict[str, Any]:
        """Get a chatbot by ID.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            chatbot_id: The chatbot id.
        """
        return self._request(
            OperationInfo(
                service="campfires",
                operation="get_chatbot",
                is_mutation=False,
                project_id=bucket_id,
                resource_id=chatbot_id,
            ),
            "GET",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations/{chatbot_id}",
            operation="GetChatbot",
        )

    def update_chatbot(
        self, *, bucket_id: int, campfire_id: int, chatbot_id: int, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        """Update an existing chatbot.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            chatbot_id: The chatbot id.
            service_name: The service name.
            command_url: The command url.
        """
        return self._request(
            OperationInfo(
                service="campfires",
                operation="update_chatbot",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=chatbot_id,
            ),
            "PUT",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations/{chatbot_id}",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="UpdateChatbot",
        )

    def delete_chatbot(self, *, bucket_id: int, campfire_id: int, chatbot_id: int) -> None:
        """Delete a chatbot.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            chatbot_id: The chatbot id.
        """
        self._request_void(
            OperationInfo(
                service="campfires",
                operation="delete_chatbot",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=chatbot_id,
            ),
            "DELETE",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations/{chatbot_id}",
            operation="DeleteChatbot",
        )

    def list(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all campfires across the account.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="campfires", operation="list", is_mutation=False),
            "/chats.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListCampfires",
        )

    def get(self, *, campfire_id: int) -> dict[str, Any]:
        """Get a campfire by ID.

        Args:
            campfire_id: The campfire id.
        """
        return self._request(
            OperationInfo(service="campfires", operation="get", is_mutation=False, resource_id=campfire_id),
            "GET",
            f"/chats/{campfire_id}",
            operation="GetCampfire",
        )

    def list_lines(
        self,
        *,
        campfire_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List all lines (messages) in a campfire.

        Args:
            campfire_id: The campfire id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="campfires", operation="list_lines", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/lines.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListCampfireLines",
        )

    def create_line(self, *, campfire_id: int, content: str, content_type: str | None = None) -> dict[str, Any]:
        """Create a new line (message) in a campfire.

        Args:
            campfire_id: The campfire id.
            content: The content.
            content_type: The content type.
        """
        return self._request(
            OperationInfo(service="campfires", operation="create_line", is_mutation=True, resource_id=campfire_id),
            "POST",
            f"/chats/{campfire_id}/lines.json",
            json_body=self._compact(content=content, content_type=content_type),
            operation="CreateCampfireLine",
        )

    def get_line(self, *, campfire_id: int, line_id: int) -> dict[str, Any]:
        """Get a campfire line by ID.

        Args:
            campfire_id: The campfire id.
            line_id: The line id.
        """
        return self._request(
            OperationInfo(service="campfires", operation="get_line", is_mutation=False, resource_id=line_id),
            "GET",
            f"/chats/{campfire_id}/lines/{line_id}",
            operation="GetCampfireLine",
        )

    def update_line(self, *, campfire_id: int, line_id: int, content: str) -> None:
        """Update an existing campfire line; the content is always treated as rich text (HTML).
        The server coerces every edited line to rich text and ignores any content
        type hint. Only the line's creator may edit it, and only text and
        rich-text lines are editable.

        Args:
            campfire_id: The campfire id.
            line_id: The line id.
            content: The new line content, interpreted as rich text (HTML)
        """
        self._request_void(
            OperationInfo(service="campfires", operation="update_line", is_mutation=True, resource_id=line_id),
            "PUT",
            f"/chats/{campfire_id}/lines/{line_id}",
            json_body=self._compact(content=content),
            operation="UpdateCampfireLine",
        )

    def delete_line(self, *, campfire_id: int, line_id: int) -> None:
        """Delete a campfire line; allowed for the line's creator or an admin.
        The API responds 403 Forbidden otherwise.

        Args:
            campfire_id: The campfire id.
            line_id: The line id.
        """
        self._request_void(
            OperationInfo(service="campfires", operation="delete_line", is_mutation=True, resource_id=line_id),
            "DELETE",
            f"/chats/{campfire_id}/lines/{line_id}",
            operation="DeleteCampfireLine",
        )

    def list_uploads(
        self,
        *,
        campfire_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List uploaded files in a campfire.

        Args:
            campfire_id: The campfire id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="campfires", operation="list_uploads", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListCampfireUploads",
        )

    def create_upload(self, *, campfire_id: int, content: bytes, content_type: str, name: str) -> dict[str, Any]:
        """Upload a file to a campfire.

        Args:
            campfire_id: The campfire id.
            content: Raw bytes of the file to upload.
            content_type: MIME content type of the upload (e.g. "image/png").
            name: Filename for the uploaded file (e.g. "report.pdf").
        """
        return self._request_raw(
            OperationInfo(service="campfires", operation="create_upload", is_mutation=True, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            content=content,
            content_type=content_type,
            params=self._compact(name=name),
            operation="CreateCampfireUpload",
        )


class AsyncCampfiresService(AsyncBaseService):
    async def list_chatbots(self, *, bucket_id: int, campfire_id: int, max_items: int | None = None) -> ListResult:
        """List all chatbots for a campfire.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages.
        """
        return await self._request_paginated(
            OperationInfo(
                service="campfires",
                operation="list_chatbots",
                is_mutation=False,
                project_id=bucket_id,
                resource_id=campfire_id,
            ),
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations.json",
            max_items=max_items,
            operation="ListChatbots",
        )

    async def create_chatbot(
        self, *, bucket_id: int, campfire_id: int, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        """Create a new chatbot for a campfire.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            service_name: The service name.
            command_url: The command url.
        """
        return await self._request(
            OperationInfo(
                service="campfires",
                operation="create_chatbot",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=campfire_id,
            ),
            "POST",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations.json",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="CreateChatbot",
        )

    async def get_chatbot(self, *, bucket_id: int, campfire_id: int, chatbot_id: int) -> dict[str, Any]:
        """Get a chatbot by ID.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            chatbot_id: The chatbot id.
        """
        return await self._request(
            OperationInfo(
                service="campfires",
                operation="get_chatbot",
                is_mutation=False,
                project_id=bucket_id,
                resource_id=chatbot_id,
            ),
            "GET",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations/{chatbot_id}",
            operation="GetChatbot",
        )

    async def update_chatbot(
        self, *, bucket_id: int, campfire_id: int, chatbot_id: int, service_name: str, command_url: str | None = None
    ) -> dict[str, Any]:
        """Update an existing chatbot.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            chatbot_id: The chatbot id.
            service_name: The service name.
            command_url: The command url.
        """
        return await self._request(
            OperationInfo(
                service="campfires",
                operation="update_chatbot",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=chatbot_id,
            ),
            "PUT",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations/{chatbot_id}",
            json_body=self._compact(service_name=service_name, command_url=command_url),
            operation="UpdateChatbot",
        )

    async def delete_chatbot(self, *, bucket_id: int, campfire_id: int, chatbot_id: int) -> None:
        """Delete a chatbot.

        Args:
            bucket_id: The bucket id.
            campfire_id: The campfire id.
            chatbot_id: The chatbot id.
        """
        await self._request_void(
            OperationInfo(
                service="campfires",
                operation="delete_chatbot",
                is_mutation=True,
                project_id=bucket_id,
                resource_id=chatbot_id,
            ),
            "DELETE",
            f"/buckets/{bucket_id}/chats/{campfire_id}/integrations/{chatbot_id}",
            operation="DeleteChatbot",
        )

    async def list(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all campfires across the account.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="campfires", operation="list", is_mutation=False),
            "/chats.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListCampfires",
        )

    async def get(self, *, campfire_id: int) -> dict[str, Any]:
        """Get a campfire by ID.

        Args:
            campfire_id: The campfire id.
        """
        return await self._request(
            OperationInfo(service="campfires", operation="get", is_mutation=False, resource_id=campfire_id),
            "GET",
            f"/chats/{campfire_id}",
            operation="GetCampfire",
        )

    async def list_lines(
        self,
        *,
        campfire_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List all lines (messages) in a campfire.

        Args:
            campfire_id: The campfire id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="campfires", operation="list_lines", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/lines.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListCampfireLines",
        )

    async def create_line(self, *, campfire_id: int, content: str, content_type: str | None = None) -> dict[str, Any]:
        """Create a new line (message) in a campfire.

        Args:
            campfire_id: The campfire id.
            content: The content.
            content_type: The content type.
        """
        return await self._request(
            OperationInfo(service="campfires", operation="create_line", is_mutation=True, resource_id=campfire_id),
            "POST",
            f"/chats/{campfire_id}/lines.json",
            json_body=self._compact(content=content, content_type=content_type),
            operation="CreateCampfireLine",
        )

    async def get_line(self, *, campfire_id: int, line_id: int) -> dict[str, Any]:
        """Get a campfire line by ID.

        Args:
            campfire_id: The campfire id.
            line_id: The line id.
        """
        return await self._request(
            OperationInfo(service="campfires", operation="get_line", is_mutation=False, resource_id=line_id),
            "GET",
            f"/chats/{campfire_id}/lines/{line_id}",
            operation="GetCampfireLine",
        )

    async def update_line(self, *, campfire_id: int, line_id: int, content: str) -> None:
        """Update an existing campfire line; the content is always treated as rich text (HTML).
        The server coerces every edited line to rich text and ignores any content
        type hint. Only the line's creator may edit it, and only text and
        rich-text lines are editable.

        Args:
            campfire_id: The campfire id.
            line_id: The line id.
            content: The new line content, interpreted as rich text (HTML)
        """
        await self._request_void(
            OperationInfo(service="campfires", operation="update_line", is_mutation=True, resource_id=line_id),
            "PUT",
            f"/chats/{campfire_id}/lines/{line_id}",
            json_body=self._compact(content=content),
            operation="UpdateCampfireLine",
        )

    async def delete_line(self, *, campfire_id: int, line_id: int) -> None:
        """Delete a campfire line; allowed for the line's creator or an admin.
        The API responds 403 Forbidden otherwise.

        Args:
            campfire_id: The campfire id.
            line_id: The line id.
        """
        await self._request_void(
            OperationInfo(service="campfires", operation="delete_line", is_mutation=True, resource_id=line_id),
            "DELETE",
            f"/chats/{campfire_id}/lines/{line_id}",
            operation="DeleteCampfireLine",
        )

    async def list_uploads(
        self,
        *,
        campfire_id: int,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List uploaded files in a campfire.

        Args:
            campfire_id: The campfire id.
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="campfires", operation="list_uploads", is_mutation=False, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            params=self._compact(sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListCampfireUploads",
        )

    async def create_upload(self, *, campfire_id: int, content: bytes, content_type: str, name: str) -> dict[str, Any]:
        """Upload a file to a campfire.

        Args:
            campfire_id: The campfire id.
            content: Raw bytes of the file to upload.
            content_type: MIME content type of the upload (e.g. "image/png").
            name: Filename for the uploaded file (e.g. "report.pdf").
        """
        return await self._request_raw(
            OperationInfo(service="campfires", operation="create_upload", is_mutation=True, resource_id=campfire_id),
            f"/chats/{campfire_id}/uploads.json",
            content=content,
            content_type=content_type,
            params=self._compact(name=name),
            operation="CreateCampfireUpload",
        )
