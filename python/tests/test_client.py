from __future__ import annotations

import importlib
import inspect
import pkgutil

import pytest

import basecamp.generated.services as generated_services
from basecamp.async_client import AsyncAccountClient, AsyncClient
from basecamp.auth import BearerAuth, StaticTokenProvider
from basecamp.client import AccountClient, Client
from basecamp.services.authorization import AuthorizationService

# --- Grouped-client accessor inventory ---------------------------------------
#
# The roster is derived from the generated service package rather than typed out
# here, so a newly generated service enters this test the moment it is generated
# and fails until someone wires an accessor for it. A hand-copied literal list
# would have to be edited by the same person who forgot the accessor, which is
# exactly how `gauges` and `my_notifications` shipped unreachable (#732).
#
# One spelling rule stands between a module filename and its accessor name: the
# generator emits `webhooks_service.py` rather than `webhooks.py` to avoid a
# clash with the `basecamp.webhooks` package (`generate_services.py`'s
# `service_filename`), and the accessor is `webhooks`. It is stated as data,
# not as a branch, and nothing else is normalized.
_MODULE_TO_ACCESSOR = {"webhooks_service": "webhooks"}

# Properties on an account client that are not service accessors.
_INFRASTRUCTURE_PROPERTIES = {"account_id", "config", "http", "hooks"}


def _generated_service_modules() -> list[str]:
    """Every non-underscore module under `basecamp/generated/services/`."""
    return sorted(m.name for m in pkgutil.iter_modules(generated_services.__path__) if not m.name.startswith("_"))


def _service_classes(module_name: str) -> tuple[type, type]:
    """The (sync, async) service classes a generated module defines.

    Read off the module itself — `inspect` finds the classes it declares, so no
    class-name spelling has to be re-derived from the filename here.
    """
    module = importlib.import_module(f"{generated_services.__name__}.{module_name}")
    declared = [
        obj
        for _, obj in inspect.getmembers(module, inspect.isclass)
        if obj.__module__ == module.__name__ and obj.__name__.endswith("Service")
    ]
    sync = [c for c in declared if not c.__name__.startswith("Async")]
    async_ = [c for c in declared if c.__name__.startswith("Async")]
    assert len(sync) == 1, f"{module_name}: expected one sync service class, got {[c.__name__ for c in sync]}"
    assert len(async_) == 1, f"{module_name}: expected one async service class, got {[c.__name__ for c in async_]}"
    return sync[0], async_[0]


GENERATED_SERVICE_MODULES = _generated_service_modules()
EXPECTED_ACCESSORS = sorted(_MODULE_TO_ACCESSOR.get(m, m) for m in GENERATED_SERVICE_MODULES)


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


class TestGroupedClientAccessorInventory:
    """Every generated service must be reachable from an account client.

    `make py-check-drift` compares the generated service layer against the
    OpenAPI spec; nothing asked whether `Client` reaches what was generated, so
    two services shipped with no accessor at all (#732). This is that question.
    """

    def test_roster_is_not_empty(self):
        # Guards the derivation itself: an import or path change that yielded an
        # empty roster would make every assertion below vacuously true.
        assert len(EXPECTED_ACCESSORS) > 40
        assert len(EXPECTED_ACCESSORS) == len(GENERATED_SERVICE_MODULES)

    @pytest.mark.parametrize("accessor", EXPECTED_ACCESSORS)
    def test_sync_account_client_exposes_accessor(self, account, accessor):
        assert hasattr(type(account), accessor), (
            f"AccountClient has no `{accessor}` property; "
            f"basecamp/generated/services/ defines that service but client.py does not wire it"
        )

    @pytest.mark.parametrize("accessor", EXPECTED_ACCESSORS)
    def test_async_account_client_exposes_accessor(self, accessor):
        async_account = AsyncClient(access_token="test-token").for_account("999")
        assert hasattr(type(async_account), accessor), (
            f"AsyncAccountClient has no `{accessor}` property; "
            f"basecamp/generated/services/ defines that service but async_client.py does not wire it"
        )

    @pytest.mark.parametrize("module_name", GENERATED_SERVICE_MODULES)
    def test_accessor_resolves_to_that_module_s_service(self, account, module_name):
        # Presence alone would pass if `gauges` returned the wrong service, so
        # resolve it and check the class it actually yields. Hand-written
        # composites (§18) subclass their generated service, so `isinstance`
        # holds for them too.
        accessor = _MODULE_TO_ACCESSOR.get(module_name, module_name)
        sync_class, async_class = _service_classes(module_name)

        service = getattr(account, accessor)
        assert isinstance(service, sync_class), f"account.{accessor} returned {type(service).__name__}"
        assert not isinstance(service, async_class)

    @pytest.mark.parametrize("module_name", GENERATED_SERVICE_MODULES)
    def test_async_accessor_resolves_to_the_async_service(self, module_name):
        # The pairing matters: an async accessor handed the sync class would
        # return coroutines' worth of un-awaited calls, which reads as success.
        accessor = _MODULE_TO_ACCESSOR.get(module_name, module_name)
        _sync_class, async_class = _service_classes(module_name)

        async_account = AsyncClient(access_token="test-token").for_account("999")
        service = getattr(async_account, accessor)
        assert isinstance(service, async_class), f"async account.{accessor} returned {type(service).__name__}"

    @pytest.mark.parametrize("client_class", [AccountClient, AsyncAccountClient])
    def test_no_accessor_without_a_generated_service(self, client_class):
        # The other direction: an accessor left behind by a removed or renamed
        # service. Everything that is a property and is not one of the four
        # infrastructure properties is expected to be a service accessor.
        accessors = {
            name for name, value in vars(client_class).items() if isinstance(value, property)
        } - _INFRASTRUCTURE_PROPERTIES
        assert accessors == set(EXPECTED_ACCESSORS)
