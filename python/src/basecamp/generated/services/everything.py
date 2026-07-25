# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class EverythingService(BaseService):
    def get_everything_boosts(self, *, page: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_boosts", is_mutation=False),
            "/boosts.json",
            params=self._compact(page=page),
        )

    def get_everything_overdue_cards(self) -> ListResult:
        return self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_cards", is_mutation=False),
            "/cards/overdue.json",
        )

    def get_everything_checkins(self, *, page: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_checkins", is_mutation=False),
            "/checkins.json",
            params=self._compact(page=page),
        )

    def get_everything_comments(self, *, page: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_comments", is_mutation=False),
            "/comments.json",
            params=self._compact(page=page),
        )

    def get_everything_files(
        self, *, kind: str | None = None, people_ids: list[int] | None = None, page: int | None = None
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_files", is_mutation=False),
            "/files.json",
            params={k: v for k, v in {"kind": kind, "people_ids[]": people_ids, "page": page}.items() if v is not None},
        )

    def get_everything_forwards(self, *, page: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_forwards", is_mutation=False),
            "/forwards.json",
            params=self._compact(page=page),
        )

    def get_everything_messages(self, *, page: int | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_messages", is_mutation=False),
            "/messages.json",
            params=self._compact(page=page),
        )

    def get_everything_overdue_todos(self) -> ListResult:
        return self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_todos", is_mutation=False),
            "/todos/overdue.json",
        )


class AsyncEverythingService(AsyncBaseService):
    async def get_everything_boosts(self, *, page: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_boosts", is_mutation=False),
            "/boosts.json",
            params=self._compact(page=page),
        )

    async def get_everything_overdue_cards(self) -> ListResult:
        return await self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_cards", is_mutation=False),
            "/cards/overdue.json",
        )

    async def get_everything_checkins(self, *, page: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_checkins", is_mutation=False),
            "/checkins.json",
            params=self._compact(page=page),
        )

    async def get_everything_comments(self, *, page: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_comments", is_mutation=False),
            "/comments.json",
            params=self._compact(page=page),
        )

    async def get_everything_files(
        self, *, kind: str | None = None, people_ids: list[int] | None = None, page: int | None = None
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_files", is_mutation=False),
            "/files.json",
            params={k: v for k, v in {"kind": kind, "people_ids[]": people_ids, "page": page}.items() if v is not None},
        )

    async def get_everything_forwards(self, *, page: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_forwards", is_mutation=False),
            "/forwards.json",
            params=self._compact(page=page),
        )

    async def get_everything_messages(self, *, page: int | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="everything", operation="get_everything_messages", is_mutation=False),
            "/messages.json",
            params=self._compact(page=page),
        )

    async def get_everything_overdue_todos(self) -> ListResult:
        return await self._request_list(
            OperationInfo(service="everything", operation="get_everything_overdue_todos", is_mutation=False),
            "/todos/overdue.json",
        )
