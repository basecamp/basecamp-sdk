# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class HillChartsService(BaseService):
    def get(self, *, todoset_id: int) -> dict[str, Any]:
        """Get the hill chart for a todoset.

        Args:
            todoset_id: The todoset id.
        """
        return self._request(
            OperationInfo(service="hillcharts", operation="get", is_mutation=False, resource_id=todoset_id),
            "GET",
            f"/todosets/{todoset_id}/hill.json",
            operation="GetHillChart",
        )

    def update_settings(
        self, *, todoset_id: int, tracked: list[int] | None = None, untracked: list[int] | None = None
    ) -> dict[str, Any]:
        """Track or untrack todolists on a hill chart.

        Args:
            todoset_id: The todoset id.
            tracked: The tracked.
            untracked: The untracked.
        """
        return self._request(
            OperationInfo(service="hillcharts", operation="update_settings", is_mutation=True, resource_id=todoset_id),
            "PUT",
            f"/todosets/{todoset_id}/hills/settings.json",
            json_body=self._compact(tracked=tracked, untracked=untracked),
            operation="UpdateHillChartSettings",
        )

    def list_versions(self, *, todoset_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List the progress updates on a todoset's hill chart, newest first.

        Each time someone updates a hill chart, Basecamp saves a progress update:
        the note they wrote (`description`, rich text) and where each tracked
        todolist sat on the hill at that moment (`dots`). Earlier updates are kept,
        so this list is the hill chart's history. Discussion on an update is read
        through its `comments_url`.

        To collect updates across projects, read the progress report: events with
        a `kind` of `hill_version_created` have a `url` that `GetHillChartVersion`
        answers.

        Args:
            todoset_id: The todoset id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="hillcharts", operation="list_versions", is_mutation=False, resource_id=todoset_id),
            f"/todosets/{todoset_id}/hills/versions.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListHillChartVersions",
        )

    def create_version(
        self,
        *,
        todoset_id: int,
        description: str | None = None,
        dots: list[dict] | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Post a progress update on a todoset's hill chart.

        Moves the todolists named in `dots` and saves a progress update, as
        updating the hill chart in Basecamp does. Todolists left out of `dots` keep
        their current position, so one dot can move without sending the others.
        Every field is optional: with none, the update records the chart as it
        stands. To track or untrack a todolist, use `UpdateHillChartSettings`.

        Answers 201 with the new progress update, in the shape
        `GetHillChartVersion` returns, including where every tracked todolist now
        sits. Answers 422, with an `error` message and nothing saved, when `dots`
        isn't a list of objects, a todolist appears in it more than once or isn't
        tracked on the chart, a `position` isn't a whole number from 0 to 100, or
        the chart tracks no todolists.

        Args:
            todoset_id: The todoset id.
            description: A note about the update, as rich text (HTML).
            dots: Todolists to move, each with its ID and new position. Todolists left out keep
                their current position.
            subscriptions: People to notify of the update and subscribe to it. If absent, nobody
                else is notified.
        """
        return self._request(
            OperationInfo(service="hillcharts", operation="create_version", is_mutation=True, resource_id=todoset_id),
            "POST",
            f"/todosets/{todoset_id}/hills/versions.json",
            json_body=self._compact(description=description, dots=dots, subscriptions=subscriptions),
            operation="CreateHillChartVersion",
        )

    def get_version(self, *, todoset_id: int, version_id: int) -> dict[str, Any]:
        """Get one progress update from a todoset's hill chart.

        Args:
            todoset_id: The todoset id.
            version_id: The version id.
        """
        return self._request(
            OperationInfo(service="hillcharts", operation="get_version", is_mutation=False, resource_id=version_id),
            "GET",
            f"/todosets/{todoset_id}/hills/versions/{version_id}",
            operation="GetHillChartVersion",
        )


class AsyncHillChartsService(AsyncBaseService):
    async def get(self, *, todoset_id: int) -> dict[str, Any]:
        """Get the hill chart for a todoset.

        Args:
            todoset_id: The todoset id.
        """
        return await self._request(
            OperationInfo(service="hillcharts", operation="get", is_mutation=False, resource_id=todoset_id),
            "GET",
            f"/todosets/{todoset_id}/hill.json",
            operation="GetHillChart",
        )

    async def update_settings(
        self, *, todoset_id: int, tracked: list[int] | None = None, untracked: list[int] | None = None
    ) -> dict[str, Any]:
        """Track or untrack todolists on a hill chart.

        Args:
            todoset_id: The todoset id.
            tracked: The tracked.
            untracked: The untracked.
        """
        return await self._request(
            OperationInfo(service="hillcharts", operation="update_settings", is_mutation=True, resource_id=todoset_id),
            "PUT",
            f"/todosets/{todoset_id}/hills/settings.json",
            json_body=self._compact(tracked=tracked, untracked=untracked),
            operation="UpdateHillChartSettings",
        )

    async def list_versions(
        self, *, todoset_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List the progress updates on a todoset's hill chart, newest first.

        Each time someone updates a hill chart, Basecamp saves a progress update:
        the note they wrote (`description`, rich text) and where each tracked
        todolist sat on the hill at that moment (`dots`). Earlier updates are kept,
        so this list is the hill chart's history. Discussion on an update is read
        through its `comments_url`.

        To collect updates across projects, read the progress report: events with
        a `kind` of `hill_version_created` have a `url` that `GetHillChartVersion`
        answers.

        Args:
            todoset_id: The todoset id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="hillcharts", operation="list_versions", is_mutation=False, resource_id=todoset_id),
            f"/todosets/{todoset_id}/hills/versions.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListHillChartVersions",
        )

    async def create_version(
        self,
        *,
        todoset_id: int,
        description: str | None = None,
        dots: list[dict] | None = None,
        subscriptions: list[int] | None = None,
    ) -> dict[str, Any]:
        """Post a progress update on a todoset's hill chart.

        Moves the todolists named in `dots` and saves a progress update, as
        updating the hill chart in Basecamp does. Todolists left out of `dots` keep
        their current position, so one dot can move without sending the others.
        Every field is optional: with none, the update records the chart as it
        stands. To track or untrack a todolist, use `UpdateHillChartSettings`.

        Answers 201 with the new progress update, in the shape
        `GetHillChartVersion` returns, including where every tracked todolist now
        sits. Answers 422, with an `error` message and nothing saved, when `dots`
        isn't a list of objects, a todolist appears in it more than once or isn't
        tracked on the chart, a `position` isn't a whole number from 0 to 100, or
        the chart tracks no todolists.

        Args:
            todoset_id: The todoset id.
            description: A note about the update, as rich text (HTML).
            dots: Todolists to move, each with its ID and new position. Todolists left out keep
                their current position.
            subscriptions: People to notify of the update and subscribe to it. If absent, nobody
                else is notified.
        """
        return await self._request(
            OperationInfo(service="hillcharts", operation="create_version", is_mutation=True, resource_id=todoset_id),
            "POST",
            f"/todosets/{todoset_id}/hills/versions.json",
            json_body=self._compact(description=description, dots=dots, subscriptions=subscriptions),
            operation="CreateHillChartVersion",
        )

    async def get_version(self, *, todoset_id: int, version_id: int) -> dict[str, Any]:
        """Get one progress update from a todoset's hill chart.

        Args:
            todoset_id: The todoset id.
            version_id: The version id.
        """
        return await self._request(
            OperationInfo(service="hillcharts", operation="get_version", is_mutation=False, resource_id=version_id),
            "GET",
            f"/todosets/{todoset_id}/hills/versions/{version_id}",
            operation="GetHillChartVersion",
        )
