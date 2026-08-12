# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class EverythingService(BaseService):
    def get_everything_completed_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Completed cards across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_completed_cards", is_mutation=False),
            "/cards/completed.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingCompletedCards",
        )

    def get_everything_no_due_date_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open cards with no due date across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_no_due_date_cards", is_mutation=False),
            "/cards/no_due_date.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingNoDueDateCards",
        )

    def get_everything_not_now_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Cards parked in a project's "Not now" column across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_not_now_cards", is_mutation=False),
            "/cards/not_now.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingNotNowCards",
        )

    def get_everything_open_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Incomplete cards in active columns across all accessible projects, grouped by project (paginated).
        Each bucket entry carries the matching cards and their steps.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_open_cards", is_mutation=False),
            "/cards/open.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingOpenCards",
        )

    def get_everything_overdue_cards(
        self, *, assignee_ids: list[int] | None = None, due: str | None = None
    ) -> ListResult:
        """Get every overdue card across all accessible projects, oldest-due-date-first.
        A complete, unpaginated array; each item embeds its `bucket`.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
        """
        return self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_cards", is_mutation=False),
            "/cards/overdue.json",
            params={k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due}.items() if v is not None},
            operation="GetEverythingOverdueCards",
        )

    def get_everything_unassigned_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open, unassigned cards across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_unassigned_cards", is_mutation=False),
            "/cards/unassigned.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingUnassignedCards",
        )

    def get_everything_checkins(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every automatic check-in answer across all accessible projects, newest-first.
        Paginated; each item embeds its `bucket`.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_checkins", is_mutation=False),
            "/checkins.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingCheckins",
        )

    def get_everything_comments(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every comment across all accessible projects, newest-first (paginated).
        Each item embeds its `bucket`.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_comments", is_mutation=False),
            "/comments.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingComments",
        )

    def get_everything_files(
        self,
        *,
        kind: str | None = None,
        people_ids: list[int] | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get every file recording across all accessible projects, newest-first (paginated).
        Heterogeneous: uploads and Basecamp documents carry their
        standard recording shapes, while rich-text attachments are wrapped in a
        recording envelope plus an `attachable_sgid` and blob metadata. Modeled as
        an optional-field superset (EverythingFile) so one element type decodes any
        variant.

        Args:
            kind: Filter by file kind: all (default), images, pdfs, documents, or videos.
            people_ids: Restrict to files created by the given people (repeatable).
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_files", is_mutation=False),
            "/files.json",
            params={k: v for k, v in {"kind": kind, "people_ids[]": people_ids, "page": page}.items() if v is not None},
            max_items=max_items,
            operation="GetEverythingFiles",
        )

    def get_everything_forwards(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every inbox forward across all accessible projects, newest-first (paginated).
        Each item embeds its `bucket`.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_forwards", is_mutation=False),
            "/forwards.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingForwards",
        )

    def get_everything_messages(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every message across all accessible projects, newest-first (paginated).
        Each item embeds its `bucket` for project context.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_messages", is_mutation=False),
            "/messages.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingMessages",
        )

    def get_everything_completed_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Completed to-dos across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_completed_todos", is_mutation=False),
            "/todos/completed.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingCompletedTodos",
        )

    def get_everything_no_due_date_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open to-dos with no due date across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_no_due_date_todos", is_mutation=False),
            "/todos/no_due_date.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingNoDueDateTodos",
        )

    def get_everything_open_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Active, incomplete to-dos across all accessible projects, grouped by project (paginated).
        Each bucket entry carries the matching to-dos and their steps.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_open_todos", is_mutation=False),
            "/todos/open.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingOpenTodos",
        )

    def get_everything_overdue_todos(
        self, *, assignee_ids: list[int] | None = None, due: str | None = None
    ) -> ListResult:
        """Get every overdue to-do across all accessible projects, oldest-due-date-first.
        A complete, unpaginated array; each item embeds its `bucket`.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
        """
        return self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_todos", is_mutation=False),
            "/todos/overdue.json",
            params={k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due}.items() if v is not None},
            operation="GetEverythingOverdueTodos",
        )

    def get_everything_unassigned_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open, unassigned to-dos across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_unassigned_todos", is_mutation=False),
            "/todos/unassigned.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingUnassignedTodos",
        )


class AsyncEverythingService(AsyncBaseService):
    async def get_everything_completed_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Completed cards across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_completed_cards", is_mutation=False),
            "/cards/completed.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingCompletedCards",
        )

    async def get_everything_no_due_date_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open cards with no due date across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_no_due_date_cards", is_mutation=False),
            "/cards/no_due_date.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingNoDueDateCards",
        )

    async def get_everything_not_now_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Cards parked in a project's "Not now" column across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_not_now_cards", is_mutation=False),
            "/cards/not_now.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingNotNowCards",
        )

    async def get_everything_open_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Incomplete cards in active columns across all accessible projects, grouped by project (paginated).
        Each bucket entry carries the matching cards and their steps.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_open_cards", is_mutation=False),
            "/cards/open.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingOpenCards",
        )

    async def get_everything_overdue_cards(
        self, *, assignee_ids: list[int] | None = None, due: str | None = None
    ) -> ListResult:
        """Get every overdue card across all accessible projects, oldest-due-date-first.
        A complete, unpaginated array; each item embeds its `bucket`.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
        """
        return await self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_cards", is_mutation=False),
            "/cards/overdue.json",
            params={k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due}.items() if v is not None},
            operation="GetEverythingOverdueCards",
        )

    async def get_everything_unassigned_cards(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open, unassigned cards across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_unassigned_cards", is_mutation=False),
            "/cards/unassigned.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingUnassignedCards",
        )

    async def get_everything_checkins(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every automatic check-in answer across all accessible projects, newest-first.
        Paginated; each item embeds its `bucket`.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_checkins", is_mutation=False),
            "/checkins.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingCheckins",
        )

    async def get_everything_comments(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every comment across all accessible projects, newest-first (paginated).
        Each item embeds its `bucket`.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_comments", is_mutation=False),
            "/comments.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingComments",
        )

    async def get_everything_files(
        self,
        *,
        kind: str | None = None,
        people_ids: list[int] | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Get every file recording across all accessible projects, newest-first (paginated).
        Heterogeneous: uploads and Basecamp documents carry their
        standard recording shapes, while rich-text attachments are wrapped in a
        recording envelope plus an `attachable_sgid` and blob metadata. Modeled as
        an optional-field superset (EverythingFile) so one element type decodes any
        variant.

        Args:
            kind: Filter by file kind: all (default), images, pdfs, documents, or videos.
            people_ids: Restrict to files created by the given people (repeatable).
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_files", is_mutation=False),
            "/files.json",
            params={k: v for k, v in {"kind": kind, "people_ids[]": people_ids, "page": page}.items() if v is not None},
            max_items=max_items,
            operation="GetEverythingFiles",
        )

    async def get_everything_forwards(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every inbox forward across all accessible projects, newest-first (paginated).
        Each item embeds its `bucket`.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_forwards", is_mutation=False),
            "/forwards.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingForwards",
        )

    async def get_everything_messages(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """Get every message across all accessible projects, newest-first (paginated).
        Each item embeds its `bucket` for project context.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_messages", is_mutation=False),
            "/messages.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingMessages",
        )

    async def get_everything_completed_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Completed to-dos across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_completed_todos", is_mutation=False),
            "/todos/completed.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingCompletedTodos",
        )

    async def get_everything_no_due_date_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open to-dos with no due date across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_no_due_date_todos", is_mutation=False),
            "/todos/no_due_date.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingNoDueDateTodos",
        )

    async def get_everything_open_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Active, incomplete to-dos across all accessible projects, grouped by project (paginated).
        Each bucket entry carries the matching to-dos and their steps.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_open_todos", is_mutation=False),
            "/todos/open.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingOpenTodos",
        )

    async def get_everything_overdue_todos(
        self, *, assignee_ids: list[int] | None = None, due: str | None = None
    ) -> ListResult:
        """Get every overdue to-do across all accessible projects, oldest-due-date-first.
        A complete, unpaginated array; each item embeds its `bucket`.

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
        """
        return await self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_todos", is_mutation=False),
            "/todos/overdue.json",
            params={k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due}.items() if v is not None},
            operation="GetEverythingOverdueTodos",
        )

    async def get_everything_unassigned_todos(
        self,
        *,
        assignee_ids: list[int] | None = None,
        due: str | None = None,
        page: int | None = None,
        max_items: int | None = None,
    ) -> ListResult:
        """Open, unassigned to-dos across all accessible projects, grouped by project (paginated).

        Args:
            assignee_ids: Restrict to tasks assigned to at least one of the given people
                (repeatable). Assignees on nested steps are not considered.
            due: Filter by due date: with, without, or overdue. Unrecognized values are ignored.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_unassigned_todos", is_mutation=False),
            "/todos/unassigned.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingUnassignedTodos",
        )
