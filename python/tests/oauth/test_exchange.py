from __future__ import annotations

import httpx
import pytest
import respx

from basecamp.oauth.errors import OAuthError
from basecamp.oauth.exchange import exchange_code, refresh_token
from basecamp.oauth.token import OAuthToken

TOKEN_ENDPOINT = "https://launchpad.37signals.com/authorization/token"

TOKEN_RESPONSE = {
    "access_token": "BAhbB0kiAbB7ImNsa",  # gitleaks:allow (test fixture)
    "token_type": "Bearer",
    "refresh_token": "BAhbB0kiAbR7ImNsa",  # gitleaks:allow (test fixture)
    "expires_in": 1209600,
    "scope": "read write",
}


class TestExchangeCode:
    @respx.mock
    def test_parse_failure_never_echoes_the_body(self):
        # A syntactically-broken token body can still carry credential
        # material — the parse error must not echo any of it into the
        # message, where it would reach logs and exception telemetry.
        secret = "sk-live-SUPERSECRET"
        respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, text='{"access_token": "' + secret + "' oops"))

        with pytest.raises(OAuthError) as exc_info:
            exchange_code(
                TOKEN_ENDPOINT,
                code="auth-code-123",
                redirect_uri="https://myapp.com/callback",
                client_id="client-id",
            )
        assert exc_info.value.oauth_type == "api_error"
        assert secret not in str(exc_info.value)
        # from None: JSONDecodeError retains the whole body as .doc — the
        # chain must be suppressed, not merely sanitized.
        assert exc_info.value.__cause__ is None
        assert exc_info.value.__suppress_context__

    @respx.mock
    def test_exchange_code(self):
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        token = exchange_code(
            TOKEN_ENDPOINT,
            code="auth-code-123",
            redirect_uri="https://myapp.com/callback",
            client_id="client-id",
            client_secret="client-secret",
        )

        assert isinstance(token, OAuthToken)
        assert token.access_token == "BAhbB0kiAbB7ImNsa"  # gitleaks:allow
        assert token.token_type == "Bearer"
        assert token.refresh_token == "BAhbB0kiAbR7ImNsa"  # gitleaks:allow
        assert token.expires_in == 1209600
        assert token.scope == "read write"

        # Verify the request used standard grant_type
        request = route.calls[0].request
        body = request.content.decode()
        assert "grant_type=authorization_code" in body
        assert "code=auth-code-123" in body

    @respx.mock
    def test_exchange_code_legacy_format(self):
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        exchange_code(
            TOKEN_ENDPOINT,
            code="auth-code-123",
            redirect_uri="https://myapp.com/callback",
            client_id="client-id",
            use_legacy_format=True,
        )

        request = route.calls[0].request
        body = request.content.decode()
        assert "type=web_server" in body
        assert "grant_type" not in body

    @respx.mock
    def test_token_type_contract(self):
        # SPEC §16: token_type defaults to Bearer only when absent/JSON-null;
        # a present-but-empty or non-string value is a malformed response —
        # matching the device-flow parser.
        for body, expected in (
            ({"access_token": "a"}, "Bearer"),
            ({"access_token": "a", "token_type": None}, "Bearer"),
            ({"access_token": "a", "token_type": "Bearer"}, "Bearer"),
        ):
            respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=body))
            token = exchange_code(
                TOKEN_ENDPOINT,
                code="c",
                redirect_uri="https://myapp.com/callback",
                client_id="client-id",
            )
            assert token.token_type == expected, body

        for bad in ("", 7):
            respx.post(TOKEN_ENDPOINT).mock(
                return_value=httpx.Response(200, json={"access_token": "a", "token_type": bad})
            )
            with pytest.raises(OAuthError) as exc_info:
                exchange_code(
                    TOKEN_ENDPOINT,
                    code="c",
                    redirect_uri="https://myapp.com/callback",
                    client_id="client-id",
                )
            assert exc_info.value.oauth_type == "api_error", bad

    @respx.mock
    def test_exchange_error(self):
        respx.post(TOKEN_ENDPOINT).mock(
            return_value=httpx.Response(
                401,
                json={"error": "invalid_grant", "error_description": "Code expired"},
            )
        )

        with pytest.raises(OAuthError) as exc_info:
            exchange_code(
                TOKEN_ENDPOINT,
                code="bad-code",
                redirect_uri="https://myapp.com/callback",
                client_id="client-id",
            )

        assert exc_info.value.http_status == 401


class TestRefreshToken:
    @respx.mock
    def test_refresh_token(self):
        new_token_response = {
            "access_token": "new-access-token",
            "token_type": "Bearer",
            "expires_in": 1209600,
        }
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=new_token_response))

        token = refresh_token(
            TOKEN_ENDPOINT,
            refresh_tok="refresh-tok-123",
            client_id="client-id",
            client_secret="client-secret",
        )

        assert token.access_token == "new-access-token"
        assert token.expires_in == 1209600

        request = route.calls[0].request
        body = request.content.decode()
        assert "grant_type=refresh_token" in body
        assert "refresh_token=refresh-tok-123" in body

    @respx.mock
    def test_refresh_token_legacy_format(self):
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        refresh_token(
            TOKEN_ENDPOINT,
            refresh_tok="refresh-tok-123",
            use_legacy_format=True,
        )

        request = route.calls[0].request
        body = request.content.decode()
        assert "type=refresh" in body
        assert "grant_type" not in body


class TestResourceIndicator:
    @respx.mock
    def test_refresh_sends_resource_when_set(self):
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        refresh_token(
            TOKEN_ENDPOINT,
            refresh_tok="refresh-tok-123",
            client_id="basecamp-cli",
            resource="urn:bc:account:42",
        )

        body = route.calls[0].request.content.decode()
        assert "resource=urn%3Abc%3Aaccount%3A42" in body

    @respx.mock
    @pytest.mark.parametrize("resource", [None, ""])
    def test_refresh_omits_resource_when_unset_or_empty(self, resource):
        # None is unset; an empty string is not a binding — both must omit the
        # form key entirely (send-only-when-set; `resource=` provokes a 400).
        route = respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        refresh_token(TOKEN_ENDPOINT, refresh_tok="refresh-tok-123", resource=resource)

        body = route.calls[0].request.content.decode()
        assert "resource=" not in body

    @respx.mock
    def test_token_response_resource_round_trips(self):
        respx.post(TOKEN_ENDPOINT).mock(
            return_value=httpx.Response(200, json={**TOKEN_RESPONSE, "resource": "urn:bc:account:42"})
        )

        token = refresh_token(TOKEN_ENDPOINT, refresh_tok="refresh-tok-123")

        assert token.resource == "urn:bc:account:42"

    @respx.mock
    def test_token_response_null_resource_is_absent(self):
        respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json={**TOKEN_RESPONSE, "resource": None}))

        token = refresh_token(TOKEN_ENDPOINT, refresh_tok="refresh-tok-123")

        assert token.resource is None

    @respx.mock
    @pytest.mark.parametrize("resource", ["", 7])
    def test_token_response_malformed_resource_rejected(self, resource):
        respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(200, json={**TOKEN_RESPONSE, "resource": resource}))

        with pytest.raises(OAuthError) as exc_info:
            refresh_token(TOKEN_ENDPOINT, refresh_tok="refresh-tok-123")

        assert exc_info.value.code == "api_error"
        assert "resource" in str(exc_info.value)


REDIRECT_STATUSES = (301, 302, 303, 307, 308)

ATTACKER_LOCATION = "https://attacker.example/steal"


def _exchange():
    return exchange_code(
        TOKEN_ENDPOINT,
        code="auth-code-123",
        redirect_uri="https://myapp.com/callback",
        client_id="client-id",
    )


def _refresh():
    return refresh_token(TOKEN_ENDPOINT, refresh_tok="refresh-tok-123")


class TestRedirectRefusal:
    """SPEC §16 "Token-Endpoint Transport Policy": a redirect is refused by
    status with its body unread, and its Location is never dialled."""

    @respx.mock
    @pytest.mark.parametrize("call", [_exchange, _refresh], ids=["exchange", "refresh"])
    @pytest.mark.parametrize("status", REDIRECT_STATUSES)
    def test_redirects_are_refused_and_never_followed(self, status, call):
        route = respx.post(TOKEN_ENDPOINT).mock(
            return_value=httpx.Response(status, headers={"Location": ATTACKER_LOCATION})
        )
        # A usable token waits at the Location — following it would "succeed",
        # which is exactly the mutation this test exists to catch.
        attacker = respx.route(host="attacker.example").mock(return_value=httpx.Response(200, json=TOKEN_RESPONSE))

        with pytest.raises(OAuthError) as exc_info:
            call()

        assert exc_info.value.oauth_type == "api_error"
        assert exc_info.value.http_status == status
        assert "not followed" in str(exc_info.value)
        assert route.call_count == 1
        assert attacker.call_count == 0

    @respx.mock
    def test_304_is_generic_not_a_refused_redirect(self):
        # 304 is a cache validator, not a redirect-with-Location — it keeps
        # the generic malformed-response classification.
        respx.post(TOKEN_ENDPOINT).mock(return_value=httpx.Response(304))

        with pytest.raises(OAuthError) as exc_info:
            _exchange()

        assert exc_info.value.oauth_type == "api_error"
        assert exc_info.value.http_status == 304
        assert "not followed" not in str(exc_info.value)

    @respx.mock
    def test_redirect_classified_from_headers_before_any_body_read(self):
        # A refused redirect whose body never completes must classify from
        # the headers, not time out mid-read: the stream raises if iterated.
        class ExplodingStream(httpx.SyncByteStream):
            def __iter__(self):
                raise AssertionError("a refused redirect's body must never be read")

        respx.post(TOKEN_ENDPOINT).mock(
            return_value=httpx.Response(
                302,
                headers={"Location": ATTACKER_LOCATION},
                stream=ExplodingStream(),
            )
        )

        with pytest.raises(OAuthError) as exc_info:
            _exchange()

        assert exc_info.value.http_status == 302
        assert "not followed" in str(exc_info.value)
