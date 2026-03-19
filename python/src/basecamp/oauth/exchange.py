from __future__ import annotations

import httpx

from basecamp._security import MAX_ERROR_BODY_BYTES, check_body_size, is_localhost, require_https, truncate
from basecamp.oauth.errors import OAuthError
from basecamp.oauth.token import OAuthToken

_TOKEN_TIMEOUT = 30.0


def exchange_code(
    token_endpoint: str,
    code: str,
    redirect_uri: str,
    client_id: str,
    *,
    client_secret: str | None = None,
    code_verifier: str | None = None,
    use_legacy_format: bool = False,
) -> OAuthToken:
    """Exchange an authorization code for tokens.

    Set *use_legacy_format* to ``True`` for Launchpad's non-standard
    ``type=web_server`` format instead of the standard ``grant_type``.
    """
    if not token_endpoint:
        raise OAuthError("validation", "Token endpoint is required")
    if not code:
        raise OAuthError("validation", "Authorization code is required")
    if not redirect_uri:
        raise OAuthError("validation", "Redirect URI is required")
    if not client_id:
        raise OAuthError("validation", "Client ID is required")

    params: dict[str, str] = {}
    if use_legacy_format:
        params["type"] = "web_server"
    else:
        params["grant_type"] = "authorization_code"

    params["code"] = code
    params["redirect_uri"] = redirect_uri
    params["client_id"] = client_id
    if client_secret is not None:
        params["client_secret"] = client_secret
    if code_verifier is not None:
        params["code_verifier"] = code_verifier

    return _token_request(token_endpoint, params)


def refresh_token(
    token_endpoint: str,
    refresh_tok: str,
    *,
    client_id: str | None = None,
    client_secret: str | None = None,
    use_legacy_format: bool = False,
) -> OAuthToken:
    """Refresh an access token.

    Set *use_legacy_format* to ``True`` for Launchpad's non-standard
    ``type=refresh`` format instead of the standard ``grant_type``.
    """
    if not token_endpoint:
        raise OAuthError("validation", "Token endpoint is required")
    if not refresh_tok:
        raise OAuthError("validation", "Refresh token is required")

    params: dict[str, str] = {}
    if use_legacy_format:
        params["type"] = "refresh"
    else:
        params["grant_type"] = "refresh_token"

    params["refresh_token"] = refresh_tok
    if client_id is not None:
        params["client_id"] = client_id
    if client_secret is not None:
        params["client_secret"] = client_secret

    return _token_request(token_endpoint, params)


# ------------------------------------------------------------------
# Internal helpers
# ------------------------------------------------------------------


def _token_request(token_endpoint: str, params: dict[str, str]) -> OAuthToken:
    if not is_localhost(token_endpoint):
        require_https(token_endpoint, "token endpoint")

    try:
        response = httpx.post(
            token_endpoint,
            data=params,
            headers={
                "Content-Type": "application/x-www-form-urlencoded",
                "Accept": "application/json",
            },
            timeout=_TOKEN_TIMEOUT,
        )
    except httpx.TimeoutException as exc:
        raise OAuthError("network", "Token request timed out", retryable=True) from exc
    except httpx.HTTPError as exc:
        raise OAuthError("network", f"Token request failed: {exc}", retryable=True) from exc

    return _parse_token_response(response)


def _parse_token_response(response: httpx.Response) -> OAuthToken:
    check_body_size(response.content, MAX_ERROR_BODY_BYTES, "Token")

    try:
        data = response.json()
    except ValueError as exc:
        raise OAuthError(
            "api_error",
            f"Failed to parse token response: {truncate(response.text)}",
            http_status=response.status_code,
        ) from exc

    if not isinstance(data, dict):
        raise OAuthError(
            "api_error",
            f"Expected JSON object in token response, got {type(data).__name__}",
            http_status=response.status_code,
        )

    if not response.is_success:
        _handle_error(response.status_code, data)

    if not data.get("access_token"):
        raise OAuthError("api_error", "Token response missing access_token")

    return OAuthToken(
        access_token=data["access_token"],
        token_type=data.get("token_type", "Bearer"),
        refresh_token=data.get("refresh_token"),
        expires_in=data.get("expires_in"),
        scope=data.get("scope"),
    )


def _handle_error(status: int, data: dict) -> None:
    message = truncate(data.get("error_description") or data.get("error") or "Token request failed")

    if status == 401 or data.get("error") == "invalid_grant":
        raise OAuthError(
            "auth",
            message,
            http_status=status,
            hint="The authorization code or refresh token may be invalid or expired",
        )

    raise OAuthError("api_error", message, http_status=status)
