"""Tests for generated tools service routes."""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client, ValidationError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[OperationInfo] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(info)


def _fixture(name: str) -> dict:
    """One of the shared dock-tool fixtures `make check-fixture-coverage` validates."""
    return json.loads((_FIXTURES / "tools" / f"{name}.json").read_text(encoding="utf-8"))


def _tool(tool_id: int = 800, *, title: str = "Message Board") -> dict:
    """bc3's real dock-tool projection, trimmed to the keys these tests read.

    `app/views/api/docks/tools/show.json.jbuilder` is one line — it renders the
    bare `recordings/recording` partial and adds nothing. So a tool response
    carries NO `name` (unlike Todoset/Questionnaire, whose own recordable
    partials emit `json.name recording.recordable.name`; tools have no such
    partial) and NO `enabled` at any layer. Both were `@required` in the spec
    until #650 despite bc3 never emitting either — this helper used to fabricate
    them, which is exactly the shape no real response has.

    Conditional keys, matching the partial's branches: `subscription_url` only
    when the recordable is subscribable — Message::Board is not (Vault,
    Schedule, Inbox and Questionnaire are not either; Chat::Transcript, Todoset
    and Kanban::Board are) — and `position` only when `recording.positioned?`,
    which a docked tool is. `parent` is emitted only when `!recording.docked?`,
    so a docked tool has none. The full shapes live in `spec/fixtures/tools/`.
    """
    return {
        "id": tool_id,
        "status": "active",
        "visible_to_clients": False,
        "created_at": "2026-05-28T17:23:17.384Z",
        "updated_at": "2026-07-21T00:01:05.529Z",
        "title": title,
        "inherits_status": True,
        "type": "Message::Board",
        "url": f"{BASE}/buckets/456/message_boards/{tool_id}.json",
        "app_url": f"https://3.basecamp.com/12345/buckets/456/message_boards/{tool_id}",
        "bookmark_url": f"{BASE}/my/bookmarks/BAh7Bkki--7e5f099c.json",
        "position": 3,
        "bucket": {"id": 456, "name": "The Leto Laptop", "type": "Project"},
        "creator": {
            "id": 1049715913,
            "name": "Victor Cooper",
            "personable_type": "User",
            "title": "Chief Strategist",
            "email_address": "victor@honchodesign.com",
            "admin": True,
            "owner": True,
            "client": False,
            "employee": True,
            "time_zone": "America/Chicago",
            "avatar_url": f"{BASE}/people/BAhpBMlkkT4=--5fe7b70f/avatar",
            "company": {"id": 1033447817, "name": "Honcho Design"},
        },
    }


class TestSyncTools:
    @respx.mock
    def test_create_posts_to_bucket_scoped_dock_tools_path(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool(title="Message Board (Copy)"))
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.tools.create(
            bucket_id=456,
            tool_type="Message::Board",
            title="Message Board (Copy)",
        )

        assert route.called
        request = route.calls[0].request
        assert request.method == "POST"
        assert json.loads(request.content) == {
            "tool_type": "Message::Board",
            "title": "Message Board (Copy)",
        }
        assert result["id"] == 800

    @respx.mock
    def test_create_omits_title_when_not_provided(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool())
        )

        account = Client(access_token="test-token").for_account("12345")
        account.tools.create(bucket_id=456, tool_type="Message::Board")

        assert route.called
        assert json.loads(route.calls[0].request.content) == {"tool_type": "Message::Board"}

    @respx.mock
    def test_create_operation_metadata_scopes_project_with_null_resource(self):
        respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool())
        )

        hooks = _RecordingHooks()
        account = Client(access_token="test-token", hooks=hooks).for_account("12345")
        account.tools.create(bucket_id=456, tool_type="Message::Board")

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "tools"
        assert info.operation == "create"
        assert info.project_id == 456
        assert info.resource_id is None

    @respx.mock
    def test_create_visible_to_clients_tristate(self):
        # visible_to_clients is tri-state: unset omits the key (_compact drops None),
        # true/false are sent verbatim. An explicit false must reach the wire. Only
        # Chat::Transcript and Kanban::Board honor it; all other tool types ignore it.
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool())
        )
        account = Client(access_token="test-token").for_account("12345")

        account.tools.create(bucket_id=456, tool_type="Chat::Transcript")
        assert "visible_to_clients" not in json.loads(route.calls[-1].request.content)

        account.tools.create(bucket_id=456, tool_type="Chat::Transcript", visible_to_clients=True)
        assert json.loads(route.calls[-1].request.content)["visible_to_clients"] is True

        account.tools.create(bucket_id=456, tool_type="Chat::Transcript", visible_to_clients=False)
        body = json.loads(route.calls[-1].request.content)
        assert "visible_to_clients" in body
        assert body["visible_to_clients"] is False

    @respx.mock
    def test_create_raises_validation_error_on_422(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(422, json={"error": "Tool type is not included in the list"})
        )

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(ValidationError):
            account.tools.create(bucket_id=456, tool_type="Bogus::Tool")

        assert route.call_count == 1

    @respx.mock
    def test_get_returns_a_response_carrying_neither_name_nor_enabled(self):
        # The #650 regression, from the response side: EVERY real dock-tool
        # response omits both keys, because docks/tools/show.json.jbuilder is a
        # single `json.partial! "recordings/recording"` and no layer of that
        # partial emits `name` or `enabled`. They were @required in the spec
        # until #650, so the modeled contract described a body bc3 cannot
        # produce. This asserts the two keys stay absent while the keys bc3 does
        # emit flow through intact.
        respx.get(f"{BASE}/dock/tools/1069479832").mock(return_value=httpx.Response(200, json=_fixture("get")))

        account = Client(access_token="test-token").for_account("12345")
        result = account.tools.get(tool_id=1069479832)

        assert "name" not in result
        assert "enabled" not in result

        assert result["id"] == 1069479832
        assert result["title"] == "Chat"
        assert result["type"] == "Chat::Transcript"
        assert result["status"] == "active"
        assert result["visible_to_clients"] is False
        assert result["inherits_status"] is True
        assert result["creator"]["name"] == "Victor Cooper"
        # Chat::Transcript overrides Recordable#subscribable?, and a docked tool
        # is positioned — so both conditional keys are here. `parent` is emitted
        # only when !recording.docked?, so this docked tool has none.
        assert result["subscription_url"].endswith("/recordings/1069479832/subscription.json")
        assert result["position"] == 5
        assert "parent" not in result

    @respx.mock
    def test_get_disabled_tool_omits_position_and_subscription_url(self):
        # Disabling a tool removes it from the dock without deleting it, so
        # `recording.positioned?` is false and the partial emits no `position`
        # at all — absence of `position`, not `enabled: false`, is the disabled
        # signal (bc3 never emits `enabled`). This one is a Vault, which does not
        # override Recordable#subscribable? (default false), so
        # `subscription_url` is absent too.
        respx.get(f"{BASE}/dock/tools/1069479343").mock(return_value=httpx.Response(200, json=_fixture("disabled")))

        account = Client(access_token="test-token").for_account("12345")
        result = account.tools.get(tool_id=1069479343)

        assert "position" not in result
        assert "subscription_url" not in result
        assert "enabled" not in result
        assert "name" not in result
        assert result["type"] == "Vault"
        assert result["title"] == "Docs & Files"

    @respx.mock
    def test_get_nested_vault_carries_a_parent(self):
        # `parent` is emitted only when !recording.docked?. The dock-tool lookup
        # scopes by recordable TYPE (Recordable::CORE_GROUPS["dock_tools"]
        # includes Vault) rather than by dock membership, so a vault nested
        # inside another vault does resolve through GET /dock/tools/:id and does
        # carry a parent.
        respx.get(f"{BASE}/dock/tools/1069479562").mock(return_value=httpx.Response(200, json=_fixture("nested_vault")))

        account = Client(access_token="test-token").for_account("12345")
        result = account.tools.get(tool_id=1069479562)

        assert result["parent"]["id"] == 1069479343
        assert result["parent"]["title"] == "Docs & Files"
        assert result["parent"]["type"] == "Vault"
        assert "name" not in result
        assert "enabled" not in result


class TestAsyncTools:
    @pytest.mark.asyncio
    @respx.mock
    async def test_create_posts_to_bucket_scoped_dock_tools_path(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool(title="Message Board (Copy)"))
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.tools.create(
            bucket_id=456,
            tool_type="Message::Board",
            title="Message Board (Copy)",
        )

        assert route.called
        request = route.calls[0].request
        assert request.method == "POST"
        assert json.loads(request.content) == {
            "tool_type": "Message::Board",
            "title": "Message Board (Copy)",
        }
        assert result["id"] == 800

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_omits_title_when_not_provided(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool())
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        await account.tools.create(bucket_id=456, tool_type="Message::Board")

        assert route.called
        assert json.loads(route.calls[0].request.content) == {"tool_type": "Message::Board"}

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_operation_metadata_scopes_project_with_null_resource(self):
        respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool())
        )

        hooks = _RecordingHooks()
        account = AsyncClient(access_token="test-token", hooks=hooks).for_account("12345")
        await account.tools.create(bucket_id=456, tool_type="Message::Board")

        assert len(hooks.operations) == 1
        info = hooks.operations[0]
        assert info.service == "tools"
        assert info.operation == "create"
        assert info.project_id == 456
        assert info.resource_id is None

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_visible_to_clients_tristate(self):
        # Async counterpart of the sync tri-state test: unset omits the key,
        # true/false are sent verbatim, and an explicit false reaches the wire.
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(201, json=_tool())
        )
        account = AsyncClient(access_token="test-token").for_account("12345")

        await account.tools.create(bucket_id=456, tool_type="Chat::Transcript")
        assert "visible_to_clients" not in json.loads(route.calls[-1].request.content)

        await account.tools.create(bucket_id=456, tool_type="Chat::Transcript", visible_to_clients=True)
        assert json.loads(route.calls[-1].request.content)["visible_to_clients"] is True

        await account.tools.create(bucket_id=456, tool_type="Chat::Transcript", visible_to_clients=False)
        body = json.loads(route.calls[-1].request.content)
        assert "visible_to_clients" in body
        assert body["visible_to_clients"] is False

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_raises_validation_error_on_422(self):
        route = respx.post("https://3.basecampapi.com/12345/buckets/456/dock/tools.json").mock(
            return_value=httpx.Response(422, json={"error": "Tool type is not included in the list"})
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        with pytest.raises(ValidationError):
            await account.tools.create(bucket_id=456, tool_type="Bogus::Tool")

        assert route.call_count == 1

    @pytest.mark.asyncio
    @respx.mock
    async def test_get_returns_a_response_carrying_neither_name_nor_enabled(self):
        # Async counterpart of the sync #650 regression: docks/tools/show
        # renders the bare recordings/recording partial, which emits neither
        # `name` nor `enabled` on any response.
        respx.get(f"{BASE}/dock/tools/1069479832").mock(return_value=httpx.Response(200, json=_fixture("get")))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.tools.get(tool_id=1069479832)

        assert "name" not in result
        assert "enabled" not in result

        assert result["id"] == 1069479832
        assert result["title"] == "Chat"
        assert result["type"] == "Chat::Transcript"
        assert result["status"] == "active"
        assert result["visible_to_clients"] is False
        assert result["inherits_status"] is True
        assert result["creator"]["name"] == "Victor Cooper"
        assert result["subscription_url"].endswith("/recordings/1069479832/subscription.json")
        assert result["position"] == 5
        assert "parent" not in result

    @pytest.mark.asyncio
    @respx.mock
    async def test_get_disabled_tool_omits_position_and_subscription_url(self):
        # Async counterpart: a disabled tool is un-positioned rather than
        # deleted, so `position` is absent; a Vault is not subscribable, so
        # `subscription_url` is absent.
        respx.get(f"{BASE}/dock/tools/1069479343").mock(return_value=httpx.Response(200, json=_fixture("disabled")))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.tools.get(tool_id=1069479343)

        assert "position" not in result
        assert "subscription_url" not in result
        assert "enabled" not in result
        assert "name" not in result
        assert result["type"] == "Vault"
        assert result["title"] == "Docs & Files"

    @pytest.mark.asyncio
    @respx.mock
    async def test_get_nested_vault_carries_a_parent(self):
        # Async counterpart: a vault nested inside another vault is not docked,
        # so the partial emits `parent`.
        respx.get(f"{BASE}/dock/tools/1069479562").mock(return_value=httpx.Response(200, json=_fixture("nested_vault")))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.tools.get(tool_id=1069479562)

        assert result["parent"]["id"] == 1069479343
        assert result["parent"]["title"] == "Docs & Files"
        assert result["parent"]["type"] == "Vault"
        assert "name" not in result
        assert "enabled" not in result
