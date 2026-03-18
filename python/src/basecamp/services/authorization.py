from __future__ import annotations

import asyncio
from typing import Any

LAUNCHPAD_AUTHORIZATION_URL = "https://launchpad.37signals.com/authorization.json"


class AuthorizationService:
    """Service for authorization operations (account-independent)."""

    def __init__(self, client) -> None:
        self._http = client.http

    def get(self) -> dict[str, Any]:
        url = self._discover_authorization_url()
        response = self._http.get_absolute(url)
        return response.json()

    def _discover_authorization_url(self) -> str:
        try:
            from basecamp.oauth import discover

            config = discover(self._http.base_url)
            return f"{config.issuer.rstrip('/')}/authorization.json"
        except Exception:
            return LAUNCHPAD_AUTHORIZATION_URL


class AsyncAuthorizationService:
    """Async service for authorization operations (account-independent)."""

    def __init__(self, client) -> None:
        self._http = client.http

    async def get(self) -> dict[str, Any]:
        url = await self._discover_authorization_url()
        response = await self._http.get_absolute(url)
        return response.json()

    async def _discover_authorization_url(self) -> str:
        try:
            from basecamp.oauth import discover

            config = await asyncio.to_thread(discover, self._http.base_url)
            return f"{config.issuer.rstrip('/')}/authorization.json"
        except Exception:
            return LAUNCHPAD_AUTHORIZATION_URL
