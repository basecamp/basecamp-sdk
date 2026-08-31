from __future__ import annotations

import json

import httpx

from basecamp._security import MAX_ERROR_BODY_BYTES, is_localhost, require_https, truncate
from basecamp.oauth._transport import request_bounded
from basecamp.oauth.errors import OAuthError
from basecamp.oauth.token import OAuthToken

_TOKEN_TIMEOUT = 30.0

# The redirects a token endpoint is refused (SPEC §16 "Token-Endpoint
# Transport Policy") — same set as the signed download hop (SPEC §14). 304 is
# not in the set: it is a cache validator, not a redirect-with-Location, and
# falls through to the generic non-2xx handling.
_REDIRECT_STATUSES = frozenset({301, 302, 303, 307, 308})


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
    # APPENDED LAST (SPEC append-only refresh contract): keyword-only keeps
    # calls source-compatible, but reflected/published signature order must
    # not shift existing parameters.
    resource: str | None = None,
) -> OAuthToken:
    """Refresh an access token.

    Pass *resource* to echo the stored token's RFC 8707 resource indicator —
    BC5 multi-account refresh tokens reject a refresh without it (SPEC §16);
    it is sent only when set.

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
    # Truthiness, not `is not None`: an empty string is not a binding — treat
    # it as unset (omit) per the send-only-when-set contract, matching the
    # other SDKs. Sending `resource=` would provoke a 400 on BC5.
    if resource:
        params["resource"] = resource

    return _token_request(token_endpoint, params)


# ------------------------------------------------------------------
# Internal helpers
# ------------------------------------------------------------------


def _token_request(token_endpoint: str, params: dict[str, str]) -> OAuthToken:
    if not is_localhost(token_endpoint):
        require_https(token_endpoint, "token endpoint")

    try:
        # request_bounded, not a bare httpx call: httpx's timeout is per
        # I/O phase (it resets on every received chunk), so a peer dripping
        # bytes just under the window could hold a plain httpx.stream open
        # indefinitely, and response.read() would buffer the whole body
        # before any size check. The shared OAuth transport bounds the WHOLE
        # round trip by wall clock, suppresses redirects (a load-bearing
        # SSRF control here — SPEC §16 "Token-Endpoint Transport Policy" —
        # not library happenstance), and reads the body under a streaming
        # cap that aborts once exceeded, never post hoc. read_body skips
        # the refused redirect statuses so they classify from the headers
        # with the body unread: a 302 whose body stalls forever is the typed
        # refusal below, never a timeout.
        status, body = request_bounded(
            "POST",
            token_endpoint,
            headers={
                "Content-Type": "application/x-www-form-urlencoded",
                "Accept": "application/json",
            },
            params=params,
            timeout=_TOKEN_TIMEOUT,
            max_body_bytes=MAX_ERROR_BODY_BYTES,
            read_body=lambda status: status not in _REDIRECT_STATUSES,
            context="Token",
        )
    except httpx.TimeoutException as exc:
        raise OAuthError("network", "Token request timed out", retryable=True) from exc
    except httpx.HTTPError as exc:
        raise OAuthError("network", f"Token request failed: {exc}", retryable=True) from exc

    # A redirect is never a valid token-endpoint outcome and its Location is
    # never dialled — refuse it with the body unread.
    if status in _REDIRECT_STATUSES:
        raise OAuthError(
            "api_error",
            f"redirect {status} on the token endpoint is not followed",
            http_status=status,
        )

    return _parse_token_response(status, body)


def _parse_token_response(status: int, body: bytes) -> OAuthToken:
    try:
        data = json.loads(body)
    except ValueError:
        # A token response that fails to parse may still contain credential
        # material (a syntactically-broken body carrying an access_token) —
        # never echo ANY of it into an error message, where it would reach
        # logs and exception telemetry. The status is diagnosis enough.
        # from None — json.JSONDecodeError retains the whole document as its
        # .doc attribute, so chaining it would keep the body alive in
        # exception telemetry.
        raise OAuthError(
            "api_error",
            "Failed to parse token response",
            http_status=status,
        ) from None

    if not isinstance(data, dict):
        raise OAuthError(
            "api_error",
            f"Expected JSON object in token response, got {type(data).__name__}",
            http_status=status,
        )

    if not 200 <= status < 300:
        _handle_error(status, data)

    access_token = data.get("access_token")
    if not isinstance(access_token, str) or not access_token:
        # Non-empty STRING, not merely truthy: a numeric access_token is not a
        # usable credential (SPEC §16), and the status makes the malformed
        # response diagnosable — matching the device-flow parser.
        raise OAuthError(
            "api_error",
            "Token response missing or non-string access_token",
            http_status=status,
        )

    # resource: absent and JSON null are unset; when present it must be a
    # non-empty string (SPEC §16) — an empty binding is not a binding.
    resource = data.get("resource")
    if resource is not None and (not isinstance(resource, str) or not resource):
        raise OAuthError(
            "api_error",
            "Token response resource must be a non-empty string when present",
            http_status=status,
        )

    # token_type: absent or JSON null defaults to Bearer (dict.get's default
    # covers only absence — an explicit null passed through as None); present
    # must be a non-empty string — matching the device-flow parser (SPEC §16).
    token_type = data.get("token_type")
    if token_type is None:
        # RFC 6750 authentication scheme name, not a credential.
        token_type = "Bearer"  # noqa: S105
    elif not isinstance(token_type, str) or not token_type:
        raise OAuthError(
            "api_error",
            "Token response token_type must be a non-empty string when present",
            http_status=status,
        )

    return OAuthToken(
        access_token=data["access_token"],
        token_type=token_type,
        refresh_token=data.get("refresh_token"),
        expires_in=data.get("expires_in"),
        scope=data.get("scope"),
        resource=resource,
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
