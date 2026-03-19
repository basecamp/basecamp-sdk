from __future__ import annotations

from urllib.parse import urlencode

from basecamp.oauth.pkce import PKCE


def build_authorization_url(
    endpoint: str,
    client_id: str,
    redirect_uri: str,
    state: str,
    *,
    pkce: PKCE | None = None,
    scope: str | None = None,
) -> str:
    """Build a full authorization URL with query parameters.

    Parameters
    ----------
    endpoint:
        The authorization endpoint URL.
    client_id:
        OAuth client identifier.
    redirect_uri:
        Where the authorization server should redirect back to.
    state:
        Opaque CSRF-prevention value.
    pkce:
        Optional PKCE tuple.  When provided, ``code_challenge`` and
        ``code_challenge_method`` are appended.
    scope:
        Optional space-separated scope string.
    """
    params: dict[str, str] = {
        "response_type": "code",
        "client_id": client_id,
        "redirect_uri": redirect_uri,
        "state": state,
    }

    if pkce is not None:
        params["code_challenge"] = pkce.challenge
        params["code_challenge_method"] = pkce.method

    if scope is not None:
        params["scope"] = scope

    return f"{endpoint}?{urlencode(params)}"
