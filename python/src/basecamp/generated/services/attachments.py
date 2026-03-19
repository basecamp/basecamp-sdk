# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class AttachmentsService(BaseService):
    def create(self, *, content: bytes, content_type: str, name: str) -> dict[str, Any]:
        return self._request_raw(
            OperationInfo(service="attachments", operation="create", is_mutation=True),
            "/attachments.json",
            content=content,
            content_type=content_type,
            params=self._compact(name=name),
            operation="CreateAttachment",
        )


class AsyncAttachmentsService(AsyncBaseService):
    async def create(self, *, content: bytes, content_type: str, name: str) -> dict[str, Any]:
        return await self._request_raw(
            OperationInfo(service="attachments", operation="create", is_mutation=True),
            "/attachments.json",
            content=content,
            content_type=content_type,
            params=self._compact(name=name),
            operation="CreateAttachment",
        )
