"""Resource-first OAuth discovery tests.

Drives the shared, data-only fixtures in ``conformance/oauth/fixtures`` with this
harness's mock origins substituted for the ``{{...}}`` placeholders, so issuer /
resource binding stays code-point-exact against the mocked hosts.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import httpx
import pytest
import respx

from basecamp.errors import BasecampError, UsageError
from basecamp.oauth import (
    DiscoverySelectionError,
    OAuthError,
    discover,
    discover_from_resource,
    discover_protected_resource,
    require_origin_root,
)

FIXTURE_DIR = Path(__file__).resolve().parents[3] / "conformance" / "oauth" / "fixtures"

# Mock origins substituted for fixture placeholders. LAUNCHPAD must be the real
# origin because the fallback path targets it.
ORIGINS = {
    "{{RESOURCE_ORIGIN}}": "https://api.basecamp-test.example",
    "{{ISSUER_ORIGIN}}": "https://issuer.basecamp-test.example",
    "{{LAUNCHPAD_ORIGIN}}": "https://launchpad.37signals.com",
    "{{BC5_ISSUER}}": "https://bc5.basecamp-test.example",
}

WELL_KNOWN_RESOURCE = "/.well-known/oauth-protected-resource"
WELL_KNOWN_AS = "/.well-known/oauth-authorization-server"

# Small cap used only for the oversized-body SSRF fixture.
SMALL_CAP = 8 * 1024


def substitute(value: Any) -> Any:
    text = json.dumps(value)
    for placeholder, origin in ORIGINS.items():
        text = text.replace(placeholder, origin)
    return json.loads(text)


def load_fixtures() -> list[dict[str, Any]]:
    return [json.loads(p.read_text()) for p in sorted(FIXTURE_DIR.glob("*.json"))]


def _add_route(router: respx.Router, url: str, exchange: dict[str, Any]) -> None:
    route = router.get(url)
    if exchange.get("transportError"):
        route.mock(side_effect=httpx.ConnectError("connection refused"))
    elif exchange.get("redirectTo"):
        route.mock(
            return_value=httpx.Response(
                exchange.get("status", 302),
                headers={"Location": exchange["redirectTo"]},
            )
        )
    elif exchange.get("oversized"):
        body = dict(exchange.get("body") or {})
        body["pad"] = "x" * (256 * 1024)  # far past the SSRF cap
        route.mock(return_value=httpx.Response(exchange.get("status", 200), json=body))
    else:
        status = exchange.get("status", 200)
        body = exchange.get("body")
        if body is None:
            route.mock(return_value=httpx.Response(status))
        else:
            route.mock(return_value=httpx.Response(status, json=body))


@pytest.mark.parametrize("raw", load_fixtures(), ids=lambda fx: fx["name"])
def test_resource_discovery_fixture(raw: dict[str, Any]) -> None:
    fx = substitute(raw)
    op = fx["operation"]
    expect = fx["expect"]
    outcome = expect["outcome"]

    oversized = bool((fx.get("hop1") or {}).get("oversized") or (fx.get("hop2") or {}).get("oversized"))
    cap = SMALL_CAP if oversized else 1 * 1024 * 1024

    with respx.mock(assert_all_called=False, assert_all_mocked=True) as router:
        contacted = {"launchpad": False}

        def _track(request: httpx.Request) -> httpx.Response:
            contacted["launchpad"] = True
            origin = ORIGINS["{{LAUNCHPAD_ORIGIN}}"]
            return httpx.Response(
                200,
                json={
                    "issuer": origin,
                    "authorization_endpoint": f"{origin}/authorization/new",
                    "token_endpoint": f"{origin}/authorization/token",
                },
            )

        router.get(f"{ORIGINS['{{LAUNCHPAD_ORIGIN}}']}{WELL_KNOWN_AS}").mock(side_effect=_track)

        if fx.get("hop1"):
            _add_route(router, f"{fx['resourceOrigin']}{WELL_KNOWN_RESOURCE}", fx["hop1"])
        if fx.get("hop2"):
            issuer_origin = fx["hop2"].get("origin") or fx.get("issuerOrigin")
            _add_route(router, f"{issuer_origin}{WELL_KNOWN_AS}", fx["hop2"])

        def run() -> Any:
            if op == "discoverFromResource":
                return discover_from_resource(
                    fx["resourceOrigin"], expected_issuer=fx.get("expectedIssuer"), max_body_bytes=cap
                )
            if op == "discoverProtectedResource":
                return discover_protected_resource(fx["resourceOrigin"], max_body_bytes=cap)
            return discover(fx["issuerOrigin"], max_body_bytes=cap)

        if outcome == "raise":
            with pytest.raises(BasecampError) as exc_info:
                run()
            err = exc_info.value
            error = expect.get("error")
            if error == "usage":
                assert err.code == "usage"
            elif op == "discoverFromResource":
                assert isinstance(err, DiscoverySelectionError)
                assert err.reason == error
            else:
                # discover / discoverProtectedResource hard failures are api_error.
                assert err.code == "api_error"
            # Cross-SDK: the coarse BasecampError category (.code) must equal the
            # fixture's errorCategory ("usage" | "validation" | "api_error").
            if expect.get("errorCategory"):
                assert err.code == expect["errorCategory"]
        elif outcome == "fallback":
            result = run()
            assert result.kind == "fallback"
            assert result.reason == expect["fallbackReason"]
        else:  # selected
            result = run()
            if op == "discoverFromResource":
                assert result.kind == "selected"
                if expect.get("selectedIssuer"):
                    assert result.issuer == expect["selectedIssuer"]
            elif op == "discoverProtectedResource":
                if expect.get("selectedIssuer"):
                    assert result.resource == expect["selectedIssuer"]
            else:  # discover — absence of a throw is success
                assert result.issuer

        if expect.get("launchpadContacted") is False:
            assert contacted["launchpad"] is False


def test_origin_root_accepts_bracketed_ipv6_localhost() -> None:
    # The transport parser accepts bracketed IPv6 where a naive regex breaks.
    assert require_origin_root("http://[::1]:3000") == "http://[::1]:3000"


def test_origin_root_drops_default_port() -> None:
    assert require_origin_root("https://api.example.com:443") == "https://api.example.com"


@pytest.mark.parametrize(
    "raw",
    [
        "https://user@host",  # populated userinfo
        "https://@example.com",  # empty username, absent password
        "https://:@host",  # empty username and empty password
    ],
)
def test_origin_root_rejects_userinfo(raw: str) -> None:
    # Rejection keys off the *presence* of userinfo, not its truthiness: an
    # empty-but-present username (urlsplit reports "" for "@host") must still
    # be rejected rather than slipping through a falsy check.
    with pytest.raises(UsageError, match="userinfo"):
        require_origin_root(raw)


@pytest.mark.parametrize(
    "raw,expected",
    [
        ("https://host", "https://host"),
        ("https://[::1]:3000", "https://[::1]:3000"),
        ("https://host:443", "https://host"),
    ],
)
def test_origin_root_accepts_legitimate_origins(raw: str, expected: str) -> None:
    assert require_origin_root(raw) == expected


@respx.mock
def test_device_only_as_omits_authorization_endpoint() -> None:
    issuer = ORIGINS["{{ISSUER_ORIGIN}}"]
    respx.get(f"{issuer}{WELL_KNOWN_AS}").mock(
        return_value=httpx.Response(
            200,
            json={
                "issuer": issuer,
                "token_endpoint": f"{issuer}/oauth/token",
                "device_authorization_endpoint": f"{issuer}/oauth/device",
                "grant_types_supported": ["urn:ietf:params:oauth:grant-type:device_code", "refresh_token"],
            },
        )
    )

    config = discover(issuer)

    assert config.authorization_endpoint is None
    assert config.device_authorization_endpoint == f"{issuer}/oauth/device"
    assert "urn:ietf:params:oauth:grant-type:device_code" in (config.grant_types_supported or [])


@respx.mock
def test_protected_resource_preserves_absent_vs_empty() -> None:
    origin = "https://api.basecamp-test.example"
    respx.get(f"{origin}{WELL_KNOWN_RESOURCE}").mock(return_value=httpx.Response(200, json={"resource": origin}))
    absent = discover_protected_resource(origin)
    assert absent.authorization_servers is None


@respx.mock
def test_protected_resource_present_empty_array() -> None:
    origin = "https://api.basecamp-test.example"
    respx.get(f"{origin}{WELL_KNOWN_RESOURCE}").mock(
        return_value=httpx.Response(200, json={"resource": origin, "authorization_servers": []})
    )
    empty = discover_protected_resource(origin)
    assert empty.authorization_servers == []


@respx.mock
def test_issuer_mismatch_classified_via_marker() -> None:
    # Committed to a BC5 issuer, the AS document returns a non-matching issuer.
    # The classification must come from the structured _IssuerBindingError marker
    # (isinstance), not a substring match on the message.
    resource_origin = ORIGINS["{{RESOURCE_ORIGIN}}"]
    bc5 = ORIGINS["{{BC5_ISSUER}}"]
    respx.get(f"{resource_origin}{WELL_KNOWN_RESOURCE}").mock(
        return_value=httpx.Response(200, json={"resource": resource_origin, "authorization_servers": [bc5]})
    )
    respx.get(f"{bc5}{WELL_KNOWN_AS}").mock(
        return_value=httpx.Response(
            200,
            json={
                "issuer": "https://impostor.basecamp-test.example",  # ≠ requested bc5
                "token_endpoint": f"{bc5}/oauth/token",
            },
        )
    )

    with pytest.raises(DiscoverySelectionError) as exc_info:
        discover_from_resource(resource_origin)
    assert exc_info.value.reason == "issuer_mismatch"
    assert exc_info.value.code == "api_error"


@respx.mock
def test_issuer_mismatch_marker_is_not_message_based() -> None:
    # A generic AS-fetch failure whose message happens to omit "issuer mismatch"
    # classifies as as_fetch_failed — proving the marker, not wording, decides.
    resource_origin = ORIGINS["{{RESOURCE_ORIGIN}}"]
    bc5 = ORIGINS["{{BC5_ISSUER}}"]
    respx.get(f"{resource_origin}{WELL_KNOWN_RESOURCE}").mock(
        return_value=httpx.Response(200, json={"resource": resource_origin, "authorization_servers": [bc5]})
    )
    respx.get(f"{bc5}{WELL_KNOWN_AS}").mock(return_value=httpx.Response(500))

    with pytest.raises(DiscoverySelectionError) as exc_info:
        discover_from_resource(resource_origin)
    assert exc_info.value.reason == "as_fetch_failed"


@respx.mock
def test_discovery_rejects_non_string_endpoint_type() -> None:
    issuer = ORIGINS["{{ISSUER_ORIGIN}}"]
    respx.get(f"{issuer}{WELL_KNOWN_AS}").mock(
        return_value=httpx.Response(
            200,
            json={"issuer": issuer, "token_endpoint": f"{issuer}/oauth/token", "authorization_endpoint": 42},
        )
    )
    with pytest.raises(OAuthError) as exc_info:
        discover(issuer)
    assert exc_info.value.code == "api_error"


@respx.mock
def test_discovery_rejects_scopes_supported_wrong_type() -> None:
    issuer = ORIGINS["{{ISSUER_ORIGIN}}"]
    respx.get(f"{issuer}{WELL_KNOWN_AS}").mock(
        return_value=httpx.Response(
            200,
            json={"issuer": issuer, "token_endpoint": f"{issuer}/oauth/token", "scopes_supported": "read write"},
        )
    )
    with pytest.raises(OAuthError) as exc_info:
        discover(issuer)
    assert exc_info.value.code == "api_error"


@respx.mock
def test_discovery_rejects_list_field_with_non_string_members() -> None:
    issuer = ORIGINS["{{ISSUER_ORIGIN}}"]
    respx.get(f"{issuer}{WELL_KNOWN_AS}").mock(
        return_value=httpx.Response(
            200,
            json={
                "issuer": issuer,
                "token_endpoint": f"{issuer}/oauth/token",
                "code_challenge_methods_supported": ["S256", 256],
            },
        )
    )
    with pytest.raises(OAuthError) as exc_info:
        discover(issuer)
    assert exc_info.value.code == "api_error"


@respx.mock
def test_protected_resource_rejects_non_string_resource() -> None:
    origin = "https://api.basecamp-test.example"
    respx.get(f"{origin}{WELL_KNOWN_RESOURCE}").mock(
        return_value=httpx.Response(200, json={"resource": ["not", "a", "string"]})
    )
    with pytest.raises(OAuthError) as exc_info:
        discover_protected_resource(origin)
    assert exc_info.value.code == "api_error"


def test_selected_config_narrows_on_selected_result() -> None:
    from basecamp.oauth import DiscoveryResult, OAuthConfig

    config = OAuthConfig(issuer="https://bc5.example", token_endpoint="https://bc5.example/oauth/token")
    result = DiscoveryResult(kind="selected", config=config, issuer=config.issuer)

    # Narrows OAuthConfig | None → OAuthConfig for typed callers.
    assert result.selected_config() is config


def test_selected_config_raises_on_fallback_result() -> None:
    from basecamp.oauth import DiscoveryResult
    from basecamp.oauth.config import FallbackReason

    result = DiscoveryResult(kind="fallback", reason=FallbackReason.NO_AS_ADVERTISED)

    with pytest.raises(ValueError):
        result.selected_config()
