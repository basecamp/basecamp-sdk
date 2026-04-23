from __future__ import annotations

import json
import threading
from importlib import resources
from typing import Any, Self

from basecamp._async_http import AsyncHttpClient
from basecamp.async_auth import AsyncAuthStrategy, AsyncBearerAuth, AsyncStaticTokenProvider, AsyncTokenProvider
from basecamp.config import Config
from basecamp.download import DownloadResult, download_async
from basecamp.hooks import BasecampHooks, OperationInfo, OperationResult, safe_hook
from basecamp.services.authorization import AsyncAuthorizationService


def _load_metadata() -> dict:
    try:
        ref = resources.files("basecamp.generated") / "metadata.json"
        return json.loads(ref.read_text())
    except Exception:
        return {}


class AsyncClient:
    """Main client for the Basecamp API (async)."""

    def __init__(
        self,
        *,
        config: Config | None = None,
        access_token: str | None = None,
        token_provider: AsyncTokenProvider | None = None,
        auth: AsyncAuthStrategy | None = None,
        hooks: BasecampHooks | None = None,
        user_agent: str | None = None,
    ):
        given = sum(1 for x in (access_token, token_provider, auth) if x is not None)
        if given != 1:
            raise ValueError("Provide exactly one of: access_token, token_provider, auth")

        self.config = config or Config()
        self._hooks = hooks or BasecampHooks()
        self._metadata = _load_metadata()

        if access_token is not None:
            token_provider = AsyncStaticTokenProvider(access_token)
        if token_provider and not auth:
            auth = AsyncBearerAuth(token_provider)

        self._http = AsyncHttpClient(
            self.config,
            auth,
            self._hooks,
            user_agent=user_agent,
            metadata=self._metadata,
        )
        self._lock = threading.Lock()
        self._authorization: AsyncAuthorizationService | None = None

    @property
    def authorization(self) -> AsyncAuthorizationService:
        with self._lock:
            if self._authorization is None:
                self._authorization = AsyncAuthorizationService(self)
            return self._authorization

    @property
    def http(self) -> AsyncHttpClient:
        return self._http

    @property
    def hooks(self) -> BasecampHooks:
        return self._hooks

    def for_account(self, account_id: str | int) -> AsyncAccountClient:
        account_id = str(account_id)
        if not account_id:
            raise ValueError("account_id cannot be empty")
        if not account_id.isdigit():
            raise ValueError(f"account_id must be numeric, got: {account_id}")
        return AsyncAccountClient(parent=self, account_id=account_id)

    async def close(self) -> None:
        await self._http.close()

    async def __aenter__(self) -> Self:
        return self

    async def __aexit__(self, *exc: Any) -> None:
        await self.close()


class AsyncAccountClient:
    """Client bound to a specific Basecamp account (async)."""

    def __init__(self, *, parent: AsyncClient, account_id: str) -> None:
        self._parent = parent
        self._account_id = account_id
        self._services: dict[str, Any] = {}
        self._lock = threading.Lock()

    @property
    def account_id(self) -> str:
        return self._account_id

    @property
    def config(self) -> Config:
        return self._parent.config

    @property
    def http(self) -> AsyncHttpClient:
        return self._parent._http

    @property
    def hooks(self) -> BasecampHooks:
        return self._parent._hooks

    def account_path(self, path: str) -> str:
        if path.startswith("http://") or path.startswith("https://"):
            return path
        if not path.startswith("/"):
            path = f"/{path}"
        prefix = f"/{self._account_id}"
        if path.startswith(prefix):
            rest = path[len(prefix) :]
            if not rest or rest.startswith("/") or rest.startswith("?"):
                return path
        return f"/{self._account_id}{path}"

    async def download_url(self, raw_url: str) -> DownloadResult:
        op = OperationInfo(service="Account", operation="DownloadURL", resource_type="download", is_mutation=False)
        import time

        start = time.monotonic()
        safe_hook(self.hooks.on_operation_start, op)
        try:
            result = await download_async(raw_url, http_client=self.http, config=self.config)
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self.hooks.on_operation_end, op, OperationResult(duration_ms=duration_ms))
            return result
        except Exception as e:
            duration_ms = int((time.monotonic() - start) * 1000)
            safe_hook(self.hooks.on_operation_end, op, OperationResult(duration_ms=duration_ms, error=e))
            raise

    def _service(self, name: str, factory):
        with self._lock:
            if name not in self._services:
                self._services[name] = factory()
            return self._services[name]

    # --- Async service properties (lazy-initialized) ---

    @property
    def account(self):
        from basecamp.generated.services.account import AsyncAccountService

        return self._service("account", lambda: AsyncAccountService(self))

    @property
    def projects(self):
        from basecamp.generated.services.projects import AsyncProjectsService

        return self._service("projects", lambda: AsyncProjectsService(self))

    @property
    def todos(self):
        from basecamp.generated.services.todos import AsyncTodosService

        return self._service("todos", lambda: AsyncTodosService(self))

    @property
    def todosets(self):
        from basecamp.generated.services.todosets import AsyncTodosetsService

        return self._service("todosets", lambda: AsyncTodosetsService(self))

    @property
    def todolists(self):
        from basecamp.generated.services.todolists import AsyncTodolistsService

        return self._service("todolists", lambda: AsyncTodolistsService(self))

    @property
    def todolist_groups(self):
        from basecamp.generated.services.todolist_groups import AsyncTodolistGroupsService

        return self._service("todolist_groups", lambda: AsyncTodolistGroupsService(self))

    @property
    def hill_charts(self):
        from basecamp.generated.services.hill_charts import AsyncHillChartsService

        return self._service("hill_charts", lambda: AsyncHillChartsService(self))

    @property
    def people(self):
        from basecamp.generated.services.people import AsyncPeopleService

        return self._service("people", lambda: AsyncPeopleService(self))

    @property
    def comments(self):
        from basecamp.generated.services.comments import AsyncCommentsService

        return self._service("comments", lambda: AsyncCommentsService(self))

    @property
    def messages(self):
        from basecamp.generated.services.messages import AsyncMessagesService

        return self._service("messages", lambda: AsyncMessagesService(self))

    @property
    def message_boards(self):
        from basecamp.generated.services.message_boards import AsyncMessageBoardsService

        return self._service("message_boards", lambda: AsyncMessageBoardsService(self))

    @property
    def message_types(self):
        from basecamp.generated.services.message_types import AsyncMessageTypesService

        return self._service("message_types", lambda: AsyncMessageTypesService(self))

    @property
    def webhooks(self):
        from basecamp.generated.services.webhooks_service import AsyncWebhooksService

        return self._service("webhooks", lambda: AsyncWebhooksService(self))

    @property
    def campfires(self):
        from basecamp.generated.services.campfires import AsyncCampfiresService

        return self._service("campfires", lambda: AsyncCampfiresService(self))

    @property
    def schedules(self):
        from basecamp.generated.services.schedules import AsyncSchedulesService

        return self._service("schedules", lambda: AsyncSchedulesService(self))

    @property
    def timesheets(self):
        from basecamp.generated.services.timesheets import AsyncTimesheetsService

        return self._service("timesheets", lambda: AsyncTimesheetsService(self))

    @property
    def vaults(self):
        from basecamp.generated.services.vaults import AsyncVaultsService

        return self._service("vaults", lambda: AsyncVaultsService(self))

    @property
    def documents(self):
        from basecamp.generated.services.documents import AsyncDocumentsService

        return self._service("documents", lambda: AsyncDocumentsService(self))

    @property
    def uploads(self):
        from basecamp.services.uploads import AsyncUploadsService

        return self._service("uploads", lambda: AsyncUploadsService(self))

    @property
    def attachments(self):
        from basecamp.generated.services.attachments import AsyncAttachmentsService

        return self._service("attachments", lambda: AsyncAttachmentsService(self))

    @property
    def recordings(self):
        from basecamp.generated.services.recordings import AsyncRecordingsService

        return self._service("recordings", lambda: AsyncRecordingsService(self))

    @property
    def events(self):
        from basecamp.generated.services.events import AsyncEventsService

        return self._service("events", lambda: AsyncEventsService(self))

    @property
    def card_tables(self):
        from basecamp.generated.services.card_tables import AsyncCardTablesService

        return self._service("card_tables", lambda: AsyncCardTablesService(self))

    @property
    def cards(self):
        from basecamp.generated.services.cards import AsyncCardsService

        return self._service("cards", lambda: AsyncCardsService(self))

    @property
    def card_columns(self):
        from basecamp.generated.services.card_columns import AsyncCardColumnsService

        return self._service("card_columns", lambda: AsyncCardColumnsService(self))

    @property
    def card_steps(self):
        from basecamp.generated.services.card_steps import AsyncCardStepsService

        return self._service("card_steps", lambda: AsyncCardStepsService(self))

    @property
    def checkins(self):
        from basecamp.generated.services.checkins import AsyncCheckinsService

        return self._service("checkins", lambda: AsyncCheckinsService(self))

    @property
    def forwards(self):
        from basecamp.generated.services.forwards import AsyncForwardsService

        return self._service("forwards", lambda: AsyncForwardsService(self))

    @property
    def templates(self):
        from basecamp.generated.services.templates import AsyncTemplatesService

        return self._service("templates", lambda: AsyncTemplatesService(self))

    @property
    def search(self):
        from basecamp.generated.services.search import AsyncSearchService

        return self._service("search", lambda: AsyncSearchService(self))

    @property
    def reports(self):
        from basecamp.generated.services.reports import AsyncReportsService

        return self._service("reports", lambda: AsyncReportsService(self))

    @property
    def timeline(self):
        from basecamp.generated.services.timeline import AsyncTimelineService

        return self._service("timeline", lambda: AsyncTimelineService(self))

    @property
    def tools(self):
        from basecamp.generated.services.tools import AsyncToolsService

        return self._service("tools", lambda: AsyncToolsService(self))

    @property
    def lineup(self):
        from basecamp.generated.services.lineup import AsyncLineupService

        return self._service("lineup", lambda: AsyncLineupService(self))

    @property
    def automation(self):
        from basecamp.generated.services.automation import AsyncAutomationService

        return self._service("automation", lambda: AsyncAutomationService(self))

    @property
    def subscriptions(self):
        from basecamp.generated.services.subscriptions import AsyncSubscriptionsService

        return self._service("subscriptions", lambda: AsyncSubscriptionsService(self))

    @property
    def boosts(self):
        from basecamp.generated.services.boosts import AsyncBoostsService

        return self._service("boosts", lambda: AsyncBoostsService(self))

    @property
    def client_approvals(self):
        from basecamp.generated.services.client_approvals import AsyncClientApprovalsService

        return self._service("client_approvals", lambda: AsyncClientApprovalsService(self))

    @property
    def client_correspondences(self):
        from basecamp.generated.services.client_correspondences import AsyncClientCorrespondencesService

        return self._service("client_correspondences", lambda: AsyncClientCorrespondencesService(self))

    @property
    def client_replies(self):
        from basecamp.generated.services.client_replies import AsyncClientRepliesService

        return self._service("client_replies", lambda: AsyncClientRepliesService(self))

    @property
    def client_visibility(self):
        from basecamp.generated.services.client_visibility import AsyncClientVisibilityService

        return self._service("client_visibility", lambda: AsyncClientVisibilityService(self))
