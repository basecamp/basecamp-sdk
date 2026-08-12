# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class RecordingsService(BaseService):
    def list(
        self,
        *,
        type: str,
        bucket: str | None = None,
        status: str | None = None,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List recordings of a given type across projects.

        Args:
            type: Comment|Document|Door|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule:
                :Entry|Todo|Todolist|Upload|Vault
            bucket: The bucket.
            status: active|archived|trashed
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="recordings", operation="list", is_mutation=False),
            "/projects/recordings.json",
            params=self._compact(type=type, bucket=bucket, status=status, sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListRecordings",
        )

    def unarchive(self, *, recording_id: int) -> None:
        """Unarchive a recording (restore to active status).

        Args:
            recording_id: The recording id.
        """
        self._request_void(
            OperationInfo(service="recordings", operation="unarchive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/active.json",
            operation="UnarchiveRecording",
        )

    def archive(self, *, recording_id: int) -> None:
        """Archive a recording.

        Args:
            recording_id: The recording id.
        """
        self._request_void(
            OperationInfo(service="recordings", operation="archive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/archived.json",
            operation="ArchiveRecording",
        )

    def trash(self, *, recording_id: int) -> None:
        """Trash a recording.

        Args:
            recording_id: The recording id.
        """
        self._request_void(
            OperationInfo(service="recordings", operation="trash", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/trashed.json",
            operation="TrashRecording",
        )


class AsyncRecordingsService(AsyncBaseService):
    async def list(
        self,
        *,
        type: str,
        bucket: str | None = None,
        status: str | None = None,
        sort: str | None = None,
        direction: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """List recordings of a given type across projects.

        Args:
            type: Comment|Document|Door|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule:
                :Entry|Todo|Todolist|Upload|Vault
            bucket: The bucket.
            status: active|archived|trashed
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="recordings", operation="list", is_mutation=False),
            "/projects/recordings.json",
            params=self._compact(type=type, bucket=bucket, status=status, sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListRecordings",
        )

    async def unarchive(self, *, recording_id: int) -> None:
        """Unarchive a recording (restore to active status).

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(service="recordings", operation="unarchive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/active.json",
            operation="UnarchiveRecording",
        )

    async def archive(self, *, recording_id: int) -> None:
        """Archive a recording.

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(service="recordings", operation="archive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/archived.json",
            operation="ArchiveRecording",
        )

    async def trash(self, *, recording_id: int) -> None:
        """Trash a recording.

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(service="recordings", operation="trash", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/trashed.json",
            operation="TrashRecording",
        )
