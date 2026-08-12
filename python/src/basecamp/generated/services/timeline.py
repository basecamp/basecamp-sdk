# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TimelineService(BaseService):
    def get_project_timeline(
        self, *, project_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """Get project timeline.

        Args:
            project_id: The project id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(
                service="timeline", operation="get_project_timeline", is_mutation=False, project_id=project_id
            ),
            f"/projects/{project_id}/timeline.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetProjectTimeline",
        )


class AsyncTimelineService(AsyncBaseService):
    async def get_project_timeline(
        self, *, project_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """Get project timeline.

        Args:
            project_id: The project id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(
                service="timeline", operation="get_project_timeline", is_mutation=False, project_id=project_id
            ),
            f"/projects/{project_id}/timeline.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="GetProjectTimeline",
        )
