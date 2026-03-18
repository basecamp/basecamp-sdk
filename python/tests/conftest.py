from __future__ import annotations

import pytest

from basecamp.auth import BearerAuth, StaticTokenProvider
from basecamp.client import Client
from basecamp.config import Config
from basecamp.hooks import BasecampHooks


@pytest.fixture()
def config():
    return Config()


@pytest.fixture()
def mock_config():
    return Config(
        base_url="https://3.basecampapi.com",
        max_retries=3,
        base_delay=0.001,
        max_jitter=0.0,
    )


@pytest.fixture()
def auth():
    return BearerAuth(StaticTokenProvider("test-token"))


@pytest.fixture()
def hooks():
    return BasecampHooks()


@pytest.fixture()
def client():
    c = Client(access_token="test-token")
    yield c
    c.close()


@pytest.fixture()
def account(client):
    return client.for_account("999")
