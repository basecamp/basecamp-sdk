"""Drives the shared, data-only fixtures in ``conformance/oauth-token/fixtures``:
one refresh round-trip per fixture, asserting the sent resource form parameter
and the response decode (round-trip, absent/null as unset,
present-empty/non-string rejected). Lifecycle preservation across a stored
credential is per-manager behavior — not modeled here.
"""

from __future__ import annotations

import json
from pathlib import Path
from urllib.parse import parse_qs

import httpx
import pytest
import respx

from basecamp.oauth.errors import OAuthError
from basecamp.oauth.exchange import refresh_token

FIXTURE_DIR = Path(__file__).resolve().parents[2].parent / "conformance" / "oauth-token" / "fixtures"
TOKEN_ENDPOINT = "https://issuer.token-fixtures.example/oauth/token"

FIXTURES = sorted(FIXTURE_DIR.glob("*.json"))
assert FIXTURES, f"no fixtures found in {FIXTURE_DIR}"


@pytest.mark.parametrize("path", FIXTURES, ids=lambda p: p.name)
@respx.mock
def test_oauth_token_fixture(path: Path) -> None:
    fixture = json.loads(path.read_text())
    assert fixture["operation"] == "refreshToken"

    route = respx.post(TOKEN_ENDPOINT).mock(
        return_value=httpx.Response(fixture["response"].get("status", 200), json=fixture["response"]["body"])
    )

    kwargs = {}
    resource = fixture.get("request", {}).get("resource")
    if resource is not None:
        kwargs["resource"] = resource

    expect = fixture["expect"]
    if expect["outcome"] == "token":
        token = refresh_token(TOKEN_ENDPOINT, refresh_tok="refresh-token", client_id="basecamp-cli", **kwargs)
        if "resource" in expect:
            assert token.resource == expect["resource"]
        if expect.get("resourceAbsent"):
            assert token.resource is None
    else:
        with pytest.raises(OAuthError) as exc_info:
            refresh_token(TOKEN_ENDPOINT, refresh_tok="refresh-token", client_id="basecamp-cli", **kwargs)
        assert exc_info.value.code == "api_error"

    # keep_blank_values: a regression that sends `resource=` (blank value)
    # instead of omitting the key must FAIL formResourceAbsent — parse_qs drops
    # blank-valued parameters by default, which would mask exactly that bug.
    form = parse_qs(route.calls[0].request.content.decode(), keep_blank_values=True)
    if "formResource" in expect:
        assert form.get("resource") == [expect["formResource"]]
    if expect.get("formResourceAbsent"):
        assert "resource" not in form
