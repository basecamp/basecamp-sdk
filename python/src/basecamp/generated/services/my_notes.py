# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MyNotesService(BaseService):
    def get_my_note(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="mynotes", operation="get_my_note", is_mutation=False),
            "GET",
            "/my/notes.json",
            operation="GetMyNote",
        )

    def update_my_note(self, *, note: dict) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="mynotes", operation="update_my_note", is_mutation=True),
            "PUT",
            "/my/notes.json",
            json_body=self._compact(note=note),
            operation="UpdateMyNote",
        )


class AsyncMyNotesService(AsyncBaseService):
    async def get_my_note(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="mynotes", operation="get_my_note", is_mutation=False),
            "GET",
            "/my/notes.json",
            operation="GetMyNote",
        )

    async def update_my_note(self, *, note: dict) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="mynotes", operation="update_my_note", is_mutation=True),
            "PUT",
            "/my/notes.json",
            json_body=self._compact(note=note),
            operation="UpdateMyNote",
        )
