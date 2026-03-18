from __future__ import annotations

import httpx

from basecamp._security import MAX_ERROR_BODY_BYTES, check_body_size, is_localhost, require_https, truncate
from basecamp.errors import ApiError
from basecamp.oauth.config import OAuthConfig
from basecamp.oauth.errors import OAuthError

LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com"

_DISCOVERY_TIMEOUT = 10.0


def discover(base_url: str) -> OAuthConfig:
    """Fetch OAuth 2 server configuration from a well-known discovery endpoint.

    GETs ``{base_url}/.well-known/oauth-authorization-server``, parses the
    JSON response, and returns an :class:`OAuthConfig`.  HTTPS is enforced
    unless the host is localhost.
    """
    if not is_localhost(base_url):
        require_https(base_url, "discovery base URL")

    normalized = base_url.rstrip("/")
    url = f"{normalized}/.well-known/oauth-authorization-server"

    try:
        response = httpx.get(url, headers={"Accept": "application/json"}, timeout=_DISCOVERY_TIMEOUT)
    except httpx.TimeoutException as exc:
        raise OAuthError("network", "OAuth discovery timed out", retryable=True) from exc
    except httpx.HTTPError as exc:
        raise OAuthError("network", f"OAuth discovery failed: {exc}", retryable=True) from exc

    try:
        check_body_size(response.content, MAX_ERROR_BODY_BYTES, "Discovery")
    except ApiError as exc:
        raise OAuthError("api_error", str(exc)) from exc

    if not response.is_success:
        raise OAuthError(
            "network",
            f"OAuth discovery failed with status {response.status_code}: {truncate(response.text)}",
            http_status=response.status_code,
        )

    try:
        data = response.json()
    except ValueError as exc:
        raise OAuthError("api_error", f"Failed to parse discovery response: {exc}") from exc

    _validate(data)

    return OAuthConfig(
        issuer=data["issuer"],
        authorization_endpoint=data["authorization_endpoint"],
        token_endpoint=data["token_endpoint"],
        registration_endpoint=data.get("registration_endpoint"),
        scopes_supported=data.get("scopes_supported"),
    )


def discover_launchpad() -> OAuthConfig:
    """Convenience wrapper: discover configuration from Launchpad."""
    return discover(LAUNCHPAD_BASE_URL)


def _validate(data: dict) -> None:
    missing = [f for f in ("issuer", "authorization_endpoint", "token_endpoint") if not data.get(f)]
    if missing:
        raise OAuthError(
            "api_error",
            f"Invalid OAuth discovery response: missing required fields: {', '.join(missing)}",
        )
