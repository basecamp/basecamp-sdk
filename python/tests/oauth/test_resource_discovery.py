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
    DiscoveryResult,
    DiscoverySelectionError,
    OAuthConfig,
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
            # Register the mock at the NORMALIZED origin: the SDK builds the
            # well-known URL from the normalized origin even when the caller's
            # spelling differs (trailing slash, explicit :443), so the raw string
            # would not match the actual request.
            _add_route(
                router,
                f"{require_origin_root(fx['resourceOrigin'])}{WELL_KNOWN_RESOURCE}",
                fx["hop1"],
            )
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


def test_origin_root_preserves_idna_a_label_ascii() -> None:
    # An ASCII A-label origin must round-trip as ASCII (raw_host), not be
    # rewritten to httpx's IDNA-decoded Unicode form: an AS echoing its ASCII
    # issuer/resource URI must still bind by code-point.
    assert require_origin_root("https://xn--e1afmkfd.example") == "https://xn--e1afmkfd.example"


@pytest.mark.parametrize("raw", ["https://api.example.com?", "https://api.example.com#"])
def test_origin_root_rejects_bare_query_or_fragment(raw: str) -> None:
    # httpx collapses a bare "?"/"#" to an empty (falsy) query/fragment, so the
    # parsed fields miss it; the raw scan must still reject the delimiter.
    with pytest.raises(UsageError, match="query or fragment"):
        require_origin_root(raw)


def test_body_cap_normalizes_non_finite_to_default() -> None:
    from basecamp.oauth.discovery import MAX_DISCOVERY_BODY_BYTES, _normalize_body_cap

    # None/float/inf/negative would disable the streaming memory bound; each must
    # fall back to the finite default so the SSRF cap can never be turned off.
    for bad in (None, float("inf"), 1.5, -1, True, "big"):
        assert _normalize_body_cap(bad) == MAX_DISCOVERY_BODY_BYTES
    # A valid non-negative int is preserved.
    assert _normalize_body_cap(4096) == 4096


@pytest.mark.parametrize("raw", ["https:\\\\host", "https://host\n", "https://host ", "https://ho st"])
def test_origin_root_rejects_normalized_spellings(raw: str) -> None:
    # Parsers strip C0 controls / whitespace or percent-encode a space into the
    # host; the up-front raw scan must reject these before they are cleaned.
    with pytest.raises(UsageError, match="invalid characters"):
        require_origin_root(raw)


@pytest.mark.parametrize("raw", ["https://api.example/a/..", "https://api.example/%2e%2e"])
def test_origin_root_rejects_dot_segment_path(raw: str) -> None:
    # httpx resolves "/a/.." to "/", so the normalized-path check misses it; the
    # raw-path scan must reject any path beyond "/".
    with pytest.raises(UsageError, match="no path"):
        require_origin_root(raw)


def test_origin_root_rejects_dangling_port() -> None:
    # A dangling ":" normalizes to port None under httpx (looks like no port); the
    # raw-authority check must still reject the malformed authority.
    with pytest.raises(UsageError, match="invalid port"):
        require_origin_root("https://bc5.example:")


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


@pytest.mark.parametrize(
    "raw",
    [
        "https://[v1.foo]",  # IPvFuture: urllib would strip brackets → https://v1.foo
        "https://xn--invalid-.example",  # invalid IDNA A-label (ends with a hyphen)
        "https://example.com:99999",  # out-of-range port (httpx accepts; we range-check)
    ],
)
def test_origin_root_rejects_transport_unrepresentable(raw: str) -> None:
    # Validate with the SAME parser the transport dials with (httpx.URL): a value
    # httpx cannot represent must be rejected here, not silently converted by a
    # different parser and then rewritten/rejected at request time.
    with pytest.raises(UsageError):
        require_origin_root(raw)


def test_oauthconfig_positional_registration_endpoint_slot() -> None:
    # device_authorization_endpoint is APPENDED, so a legacy 4-positional-arg
    # caller still lands its 4th value in registration_endpoint, not the new field.
    cfg = OAuthConfig("https://iss", "https://iss/auth", "https://iss/token", "https://iss/register")
    assert cfg.registration_endpoint == "https://iss/register"
    assert cfg.device_authorization_endpoint is None


def test_discovery_result_enforces_per_kind_invariant() -> None:
    with pytest.raises(ValueError):
        DiscoveryResult(kind="selected", config=None, issuer=None)  # missing config/issuer
    with pytest.raises(ValueError):
        DiscoveryResult(kind="fallback", reason=None)  # missing reason
    with pytest.raises(ValueError):
        DiscoveryResult(kind="fallback", reason="no_as_advertised", issuer="https://iss")  # stray issuer


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
def test_duplicate_advertised_issuer_is_one_candidate_not_ambiguous() -> None:
    # The same non-Launchpad issuer advertised twice is ONE candidate: the
    # exclusion heuristic dedupes by code-point rather than raising ambiguous.
    resource_origin = ORIGINS["{{RESOURCE_ORIGIN}}"]
    bc5 = ORIGINS["{{BC5_ISSUER}}"]
    respx.get(f"{resource_origin}{WELL_KNOWN_RESOURCE}").mock(
        return_value=httpx.Response(200, json={"resource": resource_origin, "authorization_servers": [bc5, bc5]})
    )
    respx.get(f"{bc5}{WELL_KNOWN_AS}").mock(
        return_value=httpx.Response(200, json={"issuer": bc5, "token_endpoint": f"{bc5}/token"})
    )

    result = discover_from_resource(resource_origin)

    assert result.kind == "selected"
    assert result.issuer == bc5


@respx.mock
def test_resource_binds_against_raw_caller_default_port() -> None:
    # ":443" normalizes away for the fetch URL, but the metadata resource is bound
    # code-point-exact against the ORIGINAL caller identifier (RFC 9728 §3.3).
    res = "https://api.basecamp-test.example"
    respx.get(f"{res}{WELL_KNOWN_RESOURCE}").mock(return_value=httpx.Response(200, json={"resource": f"{res}:443"}))
    meta = discover_protected_resource(f"{res}:443")
    assert meta.resource == f"{res}:443"


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
def test_present_null_authorization_servers_is_malformed_not_empty() -> None:
    # A present JSON null authorization_servers is MALFORMED metadata, not
    # "present but empty": it must fail hop-1 (soft resource_discovery_failed),
    # never be normalized to [] and read as no_as_advertised.
    origin = ORIGINS["{{RESOURCE_ORIGIN}}"]
    respx.get(f"{origin}{WELL_KNOWN_RESOURCE}").mock(
        return_value=httpx.Response(200, json={"resource": origin, "authorization_servers": None})
    )
    result = discover_from_resource(origin)
    assert result.kind == "fallback"
    assert result.reason == "resource_discovery_failed"


@respx.mock
def test_present_null_grant_types_is_rejected_not_absent() -> None:
    # A present JSON null list field is malformed, distinct from an absent key.
    issuer = ORIGINS["{{ISSUER_ORIGIN}}"]
    respx.get(f"{issuer}{WELL_KNOWN_AS}").mock(
        return_value=httpx.Response(
            200,
            json={"issuer": issuer, "token_endpoint": f"{issuer}/token", "grant_types_supported": None},
        )
    )
    with pytest.raises(OAuthError, match="grant_types_supported must be an array of strings"):
        discover(issuer)


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


def test_oauth_config_preserves_positional_field_order() -> None:
    # dataclasses generate a positional __init__ in field order — the pre-BC5
    # public order (issuer, authorization_endpoint, token_endpoint) must hold
    # so existing positional callers don't silently swap endpoints.
    from basecamp.oauth import OAuthConfig

    config = OAuthConfig("https://iss.example", "https://iss.example/auth", "https://iss.example/token")

    assert config.authorization_endpoint == "https://iss.example/auth"
    assert config.token_endpoint == "https://iss.example/token"


def test_selected_config_narrows_on_selected_result() -> None:
    from basecamp.oauth import DiscoveryResult, OAuthConfig

    config = OAuthConfig(
        issuer="https://bc5.example",
        authorization_endpoint=None,
        token_endpoint="https://bc5.example/oauth/token",
    )
    result = DiscoveryResult(kind="selected", config=config, issuer=config.issuer)

    # Narrows OAuthConfig | None → OAuthConfig for typed callers.
    assert result.selected_config() is config


def test_selected_config_raises_on_fallback_result() -> None:
    from basecamp.oauth import DiscoveryResult
    from basecamp.oauth.config import FallbackReason

    result = DiscoveryResult(kind="fallback", reason=FallbackReason.NO_AS_ADVERTISED)

    with pytest.raises(ValueError):
        result.selected_config()


def test_fallback_result_rejects_non_fallbackreason() -> None:
    from basecamp.oauth import DiscoveryResult

    # A fallback must carry a real FallbackReason member; an arbitrary string is
    # an invalid public result and must be refused at construction.
    with pytest.raises(ValueError, match="FallbackReason"):
        DiscoveryResult(kind="fallback", reason="made_up_reason")  # type: ignore[arg-type]
