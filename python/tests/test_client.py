from __future__ import annotations

import pytest

from basecamp.auth import BearerAuth, StaticTokenProvider
from basecamp.client import Client
from basecamp.services.authorization import AuthorizationService


class TestClientConstruction:
    def test_with_access_token(self):
        c = Client(access_token="tok")
        assert c.config is not None
        c.close()

    def test_with_token_provider(self):
        tp = StaticTokenProvider("tok")
        c = Client(token_provider=tp)
        c.close()

    def test_with_auth(self):
        auth = BearerAuth(StaticTokenProvider("tok"))
        c = Client(auth=auth)
        c.close()

    def test_no_auth_raises(self):
        with pytest.raises(ValueError, match="exactly one"):
            Client()

    def test_multiple_auth_raises(self):
        with pytest.raises(ValueError, match="exactly one"):
            Client(access_token="tok", auth=BearerAuth(StaticTokenProvider("tok")))


class TestForAccount:
    def test_valid_account_id(self, client):
        acct = client.for_account("12345")
        assert acct.account_id == "12345"

    def test_integer_account_id(self, client):
        acct = client.for_account(42)
        assert acct.account_id == "42"

    def test_empty_account_id_raises(self, client):
        with pytest.raises(ValueError, match="cannot be empty"):
            client.for_account("")

    def test_non_numeric_raises(self, client):
        with pytest.raises(ValueError, match="must be numeric"):
            client.for_account("abc")


class TestContextManager:
    def test_context_manager_returns_client(self):
        with Client(access_token="tok") as c:
            assert isinstance(c, Client)

    def test_context_manager_closes(self):
        c = Client(access_token="tok")
        with c:
            pass
        # After exit, the internal httpx client is closed.
        # Attempting a request would fail, but we just verify no exception on __exit__.


class TestAuthorizationProperty:
    def test_returns_authorization_service(self, client):
        svc = client.authorization
        assert isinstance(svc, AuthorizationService)

    def test_returns_same_instance(self, client):
        a = client.authorization
        b = client.authorization
        assert a is b
