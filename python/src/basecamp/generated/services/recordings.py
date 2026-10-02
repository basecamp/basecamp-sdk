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
            type:
                Comment|Document|Door|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
            bucket: The bucket.
            status: active|archived|trashed
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="recordings", operation="list", is_mutation=False),
            "/projects/recordings.json",
            params=self._compact(type=type, bucket=bucket, status=status, sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListRecordings",
        )

    def move_to_vault(self, *, recording_id: int, parent_id: int, position: int | None = None) -> None:
        """Move a document, upload or vault into another vault in the same project, or
        change its position within the vault it is already in. The recording moves in
        place: its id, comments, bookmarks and history stay with it, and a vault takes
        everything inside it along. A destination in another project is 404; moves to
        another project are not this operation. 404, 403 and some 422s carry no body.

        403 when the caller may not move the recording (an account can restrict moves
        to admins and creators). 422 when position is not a positive whole number, the
        destination is not a vault or is not active, the recording is not active, or a
        vault would move into one of its own vaults; only the position and vault-cycle
        refusals carry an `error` message.

        Args:
            recording_id: The recording id.
            parent_id: The destination vault. The recording's current vault keeps it where it is and
                changes only its position.
            position: 1-indexed position within the destination vault. Defaults to 1 (first); a
                position past the end places it last.
        """
        self._request_void(
            OperationInfo(service="recordings", operation="move_to_vault", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/filing.json",
            json_body=self._compact(parent_id=parent_id, position=position),
            operation="MoveRecordingToVault",
        )

    def spotlight(self, *, recording_id: int) -> dict[str, Any]:
        """Put a recording's card in the spotlight area on its project or template home page.
        Idempotent: spotlighting an already-spotlighted recording still returns 201.

        Args:
            recording_id: The recording id.
        """
        return self._request(
            OperationInfo(service="recordings", operation="spotlight", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/spotlight.json",
            operation="SpotlightRecording",
        )

    def unspotlight(self, *, recording_id: int) -> None:
        """Remove a recording from the spotlight area.
        Idempotent: removing an absent spotlight also returns 204.

        Args:
            recording_id: The recording id.
        """
        self._request_void(
            OperationInfo(service="recordings", operation="unspotlight", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/spotlight.json",
            operation="UnspotlightRecording",
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
        """Archive a recording; bc3 answers 403 for types it never lets be archived, timesheet entries among them.

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
        """Trash a recording; bc3 answers 403 for types it never lets be trashed, timesheet entries among them (DestroyTimesheetEntry removes those, permanently).

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
            type:
                Comment|Document|Door|Kanban::Card|Kanban::Step|Message|Question::Answer|Schedule::Entry|Todo|Todolist|Upload|Vault
            bucket: The bucket.
            status: active|archived|trashed
            sort: created_at|updated_at
            direction: asc|desc
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="recordings", operation="list", is_mutation=False),
            "/projects/recordings.json",
            params=self._compact(type=type, bucket=bucket, status=status, sort=sort, direction=direction, page=page),
            max_items=max_items,
            operation="ListRecordings",
        )

    async def move_to_vault(self, *, recording_id: int, parent_id: int, position: int | None = None) -> None:
        """Move a document, upload or vault into another vault in the same project, or
        change its position within the vault it is already in. The recording moves in
        place: its id, comments, bookmarks and history stay with it, and a vault takes
        everything inside it along. A destination in another project is 404; moves to
        another project are not this operation. 404, 403 and some 422s carry no body.

        403 when the caller may not move the recording (an account can restrict moves
        to admins and creators). 422 when position is not a positive whole number, the
        destination is not a vault or is not active, the recording is not active, or a
        vault would move into one of its own vaults; only the position and vault-cycle
        refusals carry an `error` message.

        Args:
            recording_id: The recording id.
            parent_id: The destination vault. The recording's current vault keeps it where it is and
                changes only its position.
            position: 1-indexed position within the destination vault. Defaults to 1 (first); a
                position past the end places it last.
        """
        await self._request_void(
            OperationInfo(service="recordings", operation="move_to_vault", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/filing.json",
            json_body=self._compact(parent_id=parent_id, position=position),
            operation="MoveRecordingToVault",
        )

    async def spotlight(self, *, recording_id: int) -> dict[str, Any]:
        """Put a recording's card in the spotlight area on its project or template home page.
        Idempotent: spotlighting an already-spotlighted recording still returns 201.

        Args:
            recording_id: The recording id.
        """
        return await self._request(
            OperationInfo(service="recordings", operation="spotlight", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/spotlight.json",
            operation="SpotlightRecording",
        )

    async def unspotlight(self, *, recording_id: int) -> None:
        """Remove a recording from the spotlight area.
        Idempotent: removing an absent spotlight also returns 204.

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(service="recordings", operation="unspotlight", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/spotlight.json",
            operation="UnspotlightRecording",
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
        """Archive a recording; bc3 answers 403 for types it never lets be archived, timesheet entries among them.

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
        """Trash a recording; bc3 answers 403 for types it never lets be trashed, timesheet entries among them (DestroyTimesheetEntry removes those, permanently).

        Args:
            recording_id: The recording id.
        """
        await self._request_void(
            OperationInfo(service="recordings", operation="trash", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/trashed.json",
            operation="TrashRecording",
        )
