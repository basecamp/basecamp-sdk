# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class EmbedsService(BaseService):
    def create(self, *, url: str) -> dict[str, Any]:
        """Create an embed for a URL, or return the one Basecamp already has for it.

        Answers `200 OK` with the embed either way. A new embed's status is
        `pending`; get it about once a second until its status is `active` or
        `not_found`.

        Answers `422` (`{"error": "Basecamp can't embed this URL. Link to it
        instead."}`) for a URL no provider embeds. Past 100 new embeds in an hour,
        creating another answers `429` (`{"error": "Too many new embeds. Try again
        later."}`) with no Retry-After; asking for an embed Basecamp already has
        still works. That 429 is a verdict for the rest of the hour, not
        throttling, so this operation declares `retryOn: [503]` and the 429
        surfaces on the first attempt.

        Args:
            url: The URL of the content to embed.
        """
        return self._request(
            OperationInfo(service="embeds", operation="create", is_mutation=True),
            "POST",
            "/embeds.json",
            json_body=self._compact(url=url),
            operation="CreateEmbed",
        )

    def get(self, *, sgid: str) -> dict[str, Any]:
        """Get an embed by its sgid.

        Answers `404 Not Found` for an sgid that does not name an embed.

        Args:
            sgid: An embed's signed global id, an opaque string.
        """
        return self._request(
            OperationInfo(service="embeds", operation="get", is_mutation=False),
            "GET",
            f"/embeds/{self._path_segment(sgid)}",
            operation="GetEmbed",
        )


class AsyncEmbedsService(AsyncBaseService):
    async def create(self, *, url: str) -> dict[str, Any]:
        """Create an embed for a URL, or return the one Basecamp already has for it.

        Answers `200 OK` with the embed either way. A new embed's status is
        `pending`; get it about once a second until its status is `active` or
        `not_found`.

        Answers `422` (`{"error": "Basecamp can't embed this URL. Link to it
        instead."}`) for a URL no provider embeds. Past 100 new embeds in an hour,
        creating another answers `429` (`{"error": "Too many new embeds. Try again
        later."}`) with no Retry-After; asking for an embed Basecamp already has
        still works. That 429 is a verdict for the rest of the hour, not
        throttling, so this operation declares `retryOn: [503]` and the 429
        surfaces on the first attempt.

        Args:
            url: The URL of the content to embed.
        """
        return await self._request(
            OperationInfo(service="embeds", operation="create", is_mutation=True),
            "POST",
            "/embeds.json",
            json_body=self._compact(url=url),
            operation="CreateEmbed",
        )

    async def get(self, *, sgid: str) -> dict[str, Any]:
        """Get an embed by its sgid.

        Answers `404 Not Found` for an sgid that does not name an embed.

        Args:
            sgid: An embed's signed global id, an opaque string.
        """
        return await self._request(
            OperationInfo(service="embeds", operation="get", is_mutation=False),
            "GET",
            f"/embeds/{self._path_segment(sgid)}",
            operation="GetEmbed",
        )
