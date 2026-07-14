from __future__ import annotations

from typing import Any

from basecamp._security import LAUNCHPAD_AUTHORIZATION_URL

__all__ = ["AuthorizationService", "AsyncAuthorizationService", "LAUNCHPAD_AUTHORIZATION_URL"]


class AuthorizationService:
    """Service for authorization operations (account-independent)."""

    def __init__(self, client) -> None:
        self._http = client.http

    def get(self) -> dict[str, Any]:
        response = self._http.get_absolute(LAUNCHPAD_AUTHORIZATION_URL)
        return response.json()


class AsyncAuthorizationService:
    """Async service for authorization operations (account-independent)."""

    def __init__(self, client) -> None:
        self._http = client.http

    async def get(self) -> dict[str, Any]:
        response = await self._http.get_absolute(LAUNCHPAD_AUTHORIZATION_URL)
        return response.json()
