# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TimelineService(BaseService):
    def get_project_timeline(self, *, project_id: int) -> ListResult:
        return self._request_paginated(
            OperationInfo(
                service="timeline", operation="get_project_timeline", is_mutation=False, project_id=project_id
            ),
            f"/projects/{project_id}/timeline.json",
        )


class AsyncTimelineService(AsyncBaseService):
    async def get_project_timeline(self, *, project_id: int) -> ListResult:
        return await self._request_paginated(
            OperationInfo(
                service="timeline", operation="get_project_timeline", is_mutation=False, project_id=project_id
            ),
            f"/projects/{project_id}/timeline.json",
        )
