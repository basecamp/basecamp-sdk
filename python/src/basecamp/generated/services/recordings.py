# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class RecordingsService(BaseService):
    def get_filing(self, *, filing_id: int) -> dict[str, Any]:
        """Get the state of a move or copy started by MoveRecording or CopyRecording.

        `status` is `pending` or `processing` while the work runs, `completed` when
        it is done, and `failed` if it could not be done. Poll until it leaves
        `pending` and `processing`. A completed filing carries the recording it made
        under `destination_recording`; for a move, that is the moved recording's new
        id. 404 for a filing someone else started, and for a completed one whose
        recording has since been permanently deleted.

        Args:
            filing_id: The filing id.
        """
        return self._request(
            OperationInfo(service="recordings", operation="get_filing", is_mutation=False, resource_id=filing_id),
            "GET",
            f"/filings/{filing_id}",
            operation="GetFiling",
        )

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

    def get(self, *, recording_id: int) -> dict[str, Any]:
        """Get any recording by its id, without knowing its type.

        Returns the recording's own JSON, as its type-specific read would, plus
        `bookmark_url` and `subscription_url`. `type` names the kind of recording,
        so a caller holding only an id can follow up with the type-specific read.
        `parent` names the recording it sits in, by id and type, and `bucket` its
        project. A dock tool itself (a message board, card table, Docs & Files,
        schedule, and so on) has no `parent`.

        404 when the recording does not exist, is deleted, or the caller cannot
        see it, and for plumbing recordings that have no URL of their own.

        Args:
            recording_id: The recording id.
        """
        return self._request(
            OperationInfo(service="recordings", operation="get", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}",
            operation="GetRecording",
        )

    def copy(
        self,
        *,
        recording_id: int,
        filing_destination_id: int | None = None,
        destination_project_id: int | None = None,
        parent_id: int | None = None,
        include_comments: bool | None = None,
    ) -> dict[str, Any]:
        """Start copying a recording into another project; poll GetFiling for the result.

        Works like MoveRecording, and takes the same destinations, except the
        original stays where it is. Name the destination with
        `filing_destination_id`, a container of the kind the recording sits in now,
        chosen as for MoveRecording. A copied card lands in Triage, or in Done if it
        is completed, as a moved one does. A to-do or to-do list group copies into a
        to-do list in its own project or another one. A dock tool is copied by
        `destination_project_id`, or by naming the destination project's dock as
        `filing_destination_id`. Poll GetFiling with the returned filing's id until
        `status` leaves `pending` and `processing`; a completed filing's
        `destination_recording.id` is the copy.

        The same recordings copy on their own as move on their own, except the
        Email Forwards tool, which can't be copied. Everything else copies only
        along with its parent and answers 403, as for MoveRecording: comments,
        attachments, card steps, check-in answers, hill chart updates, external
        links, the card table columns every card table has, and anything outside a
        tool, such as client correspondence and approvals.

        Not safe to retry blindly: each request makes another copy. Check the first
        filing's status before trying again.

        400 when neither destination is given. 403 when the recording can't be
        copied on its own. 404 when the destination is not one the caller can copy
        it into. 422 when the copy cannot start, such as when the recording is being
        moved or the destination cannot take it.

        Args:
            recording_id: The recording id.
            filing_destination_id: The container to copy into, chosen as for MoveRecording's
                filing_destination_id: a container of the kind the recording sits in now, such as a
                to-do list, in this project or another, for a to-do. For a dock tool, the
                destination project's dock. Give this or destination_project_id.
            destination_project_id: The project to copy a dock tool to. Applies to tools only, and
                wins over filing_destination_id when a request for a tool gives both.
            parent_id: When copying into a vault, a folder inside it to put the copy in.
            include_comments: Copy the recording's comments too. Defaults to false.
        """
        return self._request(
            OperationInfo(service="recordings", operation="copy", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/copies.json",
            json_body=self._compact(
                filing_destination_id=filing_destination_id,
                destination_project_id=destination_project_id,
                parent_id=parent_id,
                include_comments=include_comments,
            ),
            operation="CopyRecording",
        )

    def move_to_vault(self, *, recording_id: int, parent_id: int, position: int | None = None) -> None:
        """Move a document, upload or vault into another vault in its project, or reorder it in its own vault.

        Passing the vault it is already in changes only its position. The
        recording moves in place: its id, comments, bookmarks and history stay with
        it, and a vault takes everything inside it along. A destination in another
        project is 404; moves to another project are not this operation. 404, 403 and
        some 422s carry no body.

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

    def move(
        self,
        *,
        recording_id: int,
        filing_destination_id: int | None = None,
        destination_project_id: int | None = None,
        parent_id: int | None = None,
    ) -> dict[str, Any]:
        """Start moving a recording into another project; poll GetFiling for the result.

        The move runs in the background and this answers 201 with a filing. Poll
        GetFiling with its id until `status` leaves `pending` and `processing`. Once
        the move completes, the recording lives in the destination project under a
        new id, the completed filing's `destination_recording.id`; its old id no
        longer reaches it. A move always gives the recording a new id, even within
        its project. To move a to-do into another list in its own project and keep
        its id, use RepositionTodo with `parent_id` instead.

        Name the destination with `filing_destination_id`: the container the
        recording moves into, of the same kind it sits in now. Find tools in the
        destination project's `dock` (GetProject). A message moves into a
        `message_board`. A card moves into a card table (`kanban_board`) and lands
        in Triage, or in Done if it is completed; a card table column moves into a
        card table too. A document, upload, cloud file, Google document or folder
        moves into a `vault` (Docs & Files). A schedule entry moves into a
        `schedule`, a check-in question into a `questionnaire` (Automatic
        Check-ins), an email forward into an `inbox` (Email Forwards), and a to-do
        list into a `todoset` (To-dos). A to-do, or a to-do list group, moves into a
        to-do list, found with ListTodolists. The list may be in the to-do's own
        project or another one, but it can't be a group, an archived or trashed
        list, or the list the to-do is already in. A group moved into a list
        becomes a group in that list.

        A dock tool itself (a message board, card table, Docs & Files, schedule, and
        so on) has no container to name. Move it by `destination_project_id`, or
        name the destination project's dock as `filing_destination_id`.

        Only the recordings above, and the dock tools, move on their own. Everything
        else moves only along with its parent and answers 403: comments,
        attachments, card steps, check-in answers, hill chart updates, external
        links, the card table columns every card table has (Triage, Not now, Done
        and On hold), and anything outside a tool, such as client correspondence and
        approvals. To move one, move the recording or tool it belongs to.

        400 when neither destination is given. 403 when the recording can't be
        moved on its own, or the caller may not move it. 404 when the destination
        is not one the caller can move it into. 422 when the move cannot start,
        such as when the recording is already being moved or the destination
        cannot take it. A repeated move is harmless: while the first is pending it
        is 422, and once it is done, 404, because the recording has moved.

        Args:
            recording_id: The recording id.
            filing_destination_id: The container to move into, of the kind the recording sits in
                now: a card table for a card, a message board for a message, a vault for a document,
                upload, cloud file, Google document or folder, a schedule for a schedule entry, a
                questionnaire for a check-in question, an inbox for an email forward, a to-do set
                for a to-do list, and a to-do list, in this project or another, for a to-do or to-do
                list group. For a dock tool, the destination project's dock. Give this or
                destination_project_id.
            destination_project_id: The project to move a dock tool to. Applies to tools only, and
                wins over filing_destination_id when a request for a tool gives both.
            parent_id: When moving into a vault, a folder inside it to put the recording in.
        """
        return self._request(
            OperationInfo(service="recordings", operation="move", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/moves.json",
            json_body=self._compact(
                filing_destination_id=filing_destination_id,
                destination_project_id=destination_project_id,
                parent_id=parent_id,
            ),
            operation="MoveRecording",
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
    async def get_filing(self, *, filing_id: int) -> dict[str, Any]:
        """Get the state of a move or copy started by MoveRecording or CopyRecording.

        `status` is `pending` or `processing` while the work runs, `completed` when
        it is done, and `failed` if it could not be done. Poll until it leaves
        `pending` and `processing`. A completed filing carries the recording it made
        under `destination_recording`; for a move, that is the moved recording's new
        id. 404 for a filing someone else started, and for a completed one whose
        recording has since been permanently deleted.

        Args:
            filing_id: The filing id.
        """
        return await self._request(
            OperationInfo(service="recordings", operation="get_filing", is_mutation=False, resource_id=filing_id),
            "GET",
            f"/filings/{filing_id}",
            operation="GetFiling",
        )

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

    async def get(self, *, recording_id: int) -> dict[str, Any]:
        """Get any recording by its id, without knowing its type.

        Returns the recording's own JSON, as its type-specific read would, plus
        `bookmark_url` and `subscription_url`. `type` names the kind of recording,
        so a caller holding only an id can follow up with the type-specific read.
        `parent` names the recording it sits in, by id and type, and `bucket` its
        project. A dock tool itself (a message board, card table, Docs & Files,
        schedule, and so on) has no `parent`.

        404 when the recording does not exist, is deleted, or the caller cannot
        see it, and for plumbing recordings that have no URL of their own.

        Args:
            recording_id: The recording id.
        """
        return await self._request(
            OperationInfo(service="recordings", operation="get", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}",
            operation="GetRecording",
        )

    async def copy(
        self,
        *,
        recording_id: int,
        filing_destination_id: int | None = None,
        destination_project_id: int | None = None,
        parent_id: int | None = None,
        include_comments: bool | None = None,
    ) -> dict[str, Any]:
        """Start copying a recording into another project; poll GetFiling for the result.

        Works like MoveRecording, and takes the same destinations, except the
        original stays where it is. Name the destination with
        `filing_destination_id`, a container of the kind the recording sits in now,
        chosen as for MoveRecording. A copied card lands in Triage, or in Done if it
        is completed, as a moved one does. A to-do or to-do list group copies into a
        to-do list in its own project or another one. A dock tool is copied by
        `destination_project_id`, or by naming the destination project's dock as
        `filing_destination_id`. Poll GetFiling with the returned filing's id until
        `status` leaves `pending` and `processing`; a completed filing's
        `destination_recording.id` is the copy.

        The same recordings copy on their own as move on their own, except the
        Email Forwards tool, which can't be copied. Everything else copies only
        along with its parent and answers 403, as for MoveRecording: comments,
        attachments, card steps, check-in answers, hill chart updates, external
        links, the card table columns every card table has, and anything outside a
        tool, such as client correspondence and approvals.

        Not safe to retry blindly: each request makes another copy. Check the first
        filing's status before trying again.

        400 when neither destination is given. 403 when the recording can't be
        copied on its own. 404 when the destination is not one the caller can copy
        it into. 422 when the copy cannot start, such as when the recording is being
        moved or the destination cannot take it.

        Args:
            recording_id: The recording id.
            filing_destination_id: The container to copy into, chosen as for MoveRecording's
                filing_destination_id: a container of the kind the recording sits in now, such as a
                to-do list, in this project or another, for a to-do. For a dock tool, the
                destination project's dock. Give this or destination_project_id.
            destination_project_id: The project to copy a dock tool to. Applies to tools only, and
                wins over filing_destination_id when a request for a tool gives both.
            parent_id: When copying into a vault, a folder inside it to put the copy in.
            include_comments: Copy the recording's comments too. Defaults to false.
        """
        return await self._request(
            OperationInfo(service="recordings", operation="copy", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/copies.json",
            json_body=self._compact(
                filing_destination_id=filing_destination_id,
                destination_project_id=destination_project_id,
                parent_id=parent_id,
                include_comments=include_comments,
            ),
            operation="CopyRecording",
        )

    async def move_to_vault(self, *, recording_id: int, parent_id: int, position: int | None = None) -> None:
        """Move a document, upload or vault into another vault in its project, or reorder it in its own vault.

        Passing the vault it is already in changes only its position. The
        recording moves in place: its id, comments, bookmarks and history stay with
        it, and a vault takes everything inside it along. A destination in another
        project is 404; moves to another project are not this operation. 404, 403 and
        some 422s carry no body.

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

    async def move(
        self,
        *,
        recording_id: int,
        filing_destination_id: int | None = None,
        destination_project_id: int | None = None,
        parent_id: int | None = None,
    ) -> dict[str, Any]:
        """Start moving a recording into another project; poll GetFiling for the result.

        The move runs in the background and this answers 201 with a filing. Poll
        GetFiling with its id until `status` leaves `pending` and `processing`. Once
        the move completes, the recording lives in the destination project under a
        new id, the completed filing's `destination_recording.id`; its old id no
        longer reaches it. A move always gives the recording a new id, even within
        its project. To move a to-do into another list in its own project and keep
        its id, use RepositionTodo with `parent_id` instead.

        Name the destination with `filing_destination_id`: the container the
        recording moves into, of the same kind it sits in now. Find tools in the
        destination project's `dock` (GetProject). A message moves into a
        `message_board`. A card moves into a card table (`kanban_board`) and lands
        in Triage, or in Done if it is completed; a card table column moves into a
        card table too. A document, upload, cloud file, Google document or folder
        moves into a `vault` (Docs & Files). A schedule entry moves into a
        `schedule`, a check-in question into a `questionnaire` (Automatic
        Check-ins), an email forward into an `inbox` (Email Forwards), and a to-do
        list into a `todoset` (To-dos). A to-do, or a to-do list group, moves into a
        to-do list, found with ListTodolists. The list may be in the to-do's own
        project or another one, but it can't be a group, an archived or trashed
        list, or the list the to-do is already in. A group moved into a list
        becomes a group in that list.

        A dock tool itself (a message board, card table, Docs & Files, schedule, and
        so on) has no container to name. Move it by `destination_project_id`, or
        name the destination project's dock as `filing_destination_id`.

        Only the recordings above, and the dock tools, move on their own. Everything
        else moves only along with its parent and answers 403: comments,
        attachments, card steps, check-in answers, hill chart updates, external
        links, the card table columns every card table has (Triage, Not now, Done
        and On hold), and anything outside a tool, such as client correspondence and
        approvals. To move one, move the recording or tool it belongs to.

        400 when neither destination is given. 403 when the recording can't be
        moved on its own, or the caller may not move it. 404 when the destination
        is not one the caller can move it into. 422 when the move cannot start,
        such as when the recording is already being moved or the destination
        cannot take it. A repeated move is harmless: while the first is pending it
        is 422, and once it is done, 404, because the recording has moved.

        Args:
            recording_id: The recording id.
            filing_destination_id: The container to move into, of the kind the recording sits in
                now: a card table for a card, a message board for a message, a vault for a document,
                upload, cloud file, Google document or folder, a schedule for a schedule entry, a
                questionnaire for a check-in question, an inbox for an email forward, a to-do set
                for a to-do list, and a to-do list, in this project or another, for a to-do or to-do
                list group. For a dock tool, the destination project's dock. Give this or
                destination_project_id.
            destination_project_id: The project to move a dock tool to. Applies to tools only, and
                wins over filing_destination_id when a request for a tool gives both.
            parent_id: When moving into a vault, a folder inside it to put the recording in.
        """
        return await self._request(
            OperationInfo(service="recordings", operation="move", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/moves.json",
            json_body=self._compact(
                filing_destination_id=filing_destination_id,
                destination_project_id=destination_project_id,
                parent_id=parent_id,
            ),
            operation="MoveRecording",
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
