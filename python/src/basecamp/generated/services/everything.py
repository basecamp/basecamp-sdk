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
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_checkins", is_mutation=False),
            "/checkins.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingCheckins",
        )

    def get_everything_comments(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
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
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_files", is_mutation=False),
            "/files.json",
            params={k: v for k, v in {"kind": kind, "people_ids[]": people_ids, "page": page}.items() if v is not None},
            max_items=max_items,
            operation="GetEverythingFiles",
        )

    def get_everything_forwards(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_forwards", is_mutation=False),
            "/forwards.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingForwards",
        )

    def get_everything_messages(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
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
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_checkins", is_mutation=False),
            "/checkins.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingCheckins",
        )

    async def get_everything_comments(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
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
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_files", is_mutation=False),
            "/files.json",
            params={k: v for k, v in {"kind": kind, "people_ids[]": people_ids, "page": page}.items() if v is not None},
            max_items=max_items,
            operation="GetEverythingFiles",
        )

    async def get_everything_forwards(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_forwards", is_mutation=False),
            "/forwards.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetEverythingForwards",
        )

    async def get_everything_messages(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
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
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_unassigned_todos", is_mutation=False),
            "/todos/unassigned.json",
            params={
                k: v for k, v in {"assignee_ids[]": assignee_ids, "due": due, "page": page}.items() if v is not None
            },
            max_items=max_items,
            operation="GetEverythingUnassignedTodos",
        )
