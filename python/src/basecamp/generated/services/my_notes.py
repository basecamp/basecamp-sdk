# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class MyNotesService(BaseService):
    def get_my_note(self) -> dict[str, Any]:
        """Get the authenticated user's note — a per-person notebook singleton at
        /my/notes.json. If the user has not yet written anything, the shape is the
        same with empty content and null id/created_at/updated_at; the record is
        created on first update.
        """
        return self._request(
            OperationInfo(service="mynotes", operation="get_my_note", is_mutation=False),
            "GET",
            "/my/notes.json",
            operation="GetMyNote",
        )

    def update_my_note(self, *, note: dict) -> dict[str, Any]:
        """Replace the note's content, recording a new revision server-side.
        The first update also creates the underlying notebook if the user did not
        have one yet. Returns the updated note. Rejections arrive as a field-keyed
        422 ({"errors": {"content": ["can't be blank"]}}), not the flat {error}
        body.

        Args:
            note: The writable note payload — the wire body is the nested {note: {content}}
                envelope, the ProjectConstructionAttributes treatment.
        """
        return self._request(
            OperationInfo(service="mynotes", operation="update_my_note", is_mutation=True),
            "PUT",
            "/my/notes.json",
            json_body=self._compact(note=note),
            operation="UpdateMyNote",
        )


class AsyncMyNotesService(AsyncBaseService):
    async def get_my_note(self) -> dict[str, Any]:
        """Get the authenticated user's note — a per-person notebook singleton at
        /my/notes.json. If the user has not yet written anything, the shape is the
        same with empty content and null id/created_at/updated_at; the record is
        created on first update.
        """
        return await self._request(
            OperationInfo(service="mynotes", operation="get_my_note", is_mutation=False),
            "GET",
            "/my/notes.json",
            operation="GetMyNote",
        )

    async def update_my_note(self, *, note: dict) -> dict[str, Any]:
        """Replace the note's content, recording a new revision server-side.
        The first update also creates the underlying notebook if the user did not
        have one yet. Returns the updated note. Rejections arrive as a field-keyed
        422 ({"errors": {"content": ["can't be blank"]}}), not the flat {error}
        body.

        Args:
            note: The writable note payload — the wire body is the nested {note: {content}}
                envelope, the ProjectConstructionAttributes treatment.
        """
        return await self._request(
            OperationInfo(service="mynotes", operation="update_my_note", is_mutation=True),
            "PUT",
            "/my/notes.json",
            json_body=self._compact(note=note),
            operation="UpdateMyNote",
        )
