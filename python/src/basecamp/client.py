from __future__ import annotations

import json
import threading
from importlib import resources
from typing import Any, Self

from basecamp._http import HttpClient
from basecamp.auth import AuthStrategy, BearerAuth, StaticTokenProvider, TokenProvider
from basecamp.config import Config
from basecamp.download import DownloadResult, download_sync
from basecamp.hooks import BasecampHooks, OperationInfo, OperationResult, safe_hook
from basecamp.services.authorization import AuthorizationService


def _load_metadata() -> dict:
    try:
        ref = resources.files("basecamp.generated") / "metadata.json"
        return json.loads(ref.read_text())
    except Exception:
        return {}


class Client:
    """Main client for the Basecamp API (sync)."""

    def __init__(
        self,
        *,
        config: Config | None = None,
        access_token: str | None = None,
        token_provider: TokenProvider | None = None,
        auth: AuthStrategy | None = None,
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
            token_provider = StaticTokenProvider(access_token)
        if token_provider and not auth:
            auth = BearerAuth(token_provider)

        self._http = HttpClient(
            self.config,
            auth,
            self._hooks,
            user_agent=user_agent,
            metadata=self._metadata,
        )
        self._lock = threading.Lock()
        self._authorization: AuthorizationService | None = None

    @property
    def authorization(self) -> AuthorizationService:
        with self._lock:
            if self._authorization is None:
                self._authorization = AuthorizationService(self)
            return self._authorization

    @property
    def http(self) -> HttpClient:
        return self._http

    @property
    def hooks(self) -> BasecampHooks:
        return self._hooks

    def for_account(self, account_id: str | int) -> AccountClient:
        account_id = str(account_id)
        if not account_id:
            raise ValueError("account_id cannot be empty")
        if not account_id.isdigit():
            raise ValueError(f"account_id must be numeric, got: {account_id}")
        return AccountClient(parent=self, account_id=account_id)

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> Self:
        return self

    def __exit__(self, *exc: Any) -> None:
        self.close()


class AccountClient:
    """Client bound to a specific Basecamp account (sync)."""

    def __init__(self, *, parent: Client, account_id: str) -> None:
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
    def http(self) -> HttpClient:
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

    def download_url(self, raw_url: str) -> DownloadResult:
        op = OperationInfo(service="Account", operation="DownloadURL", resource_type="download", is_mutation=False)
        import time

        start = time.monotonic()
        safe_hook(self.hooks.on_operation_start, op)
        try:
            result = download_sync(raw_url, http_client=self.http, config=self.config)
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

    # --- Service properties (lazy-initialized) ---
    # These will be populated after generated services are available

    @property
    def account(self):
        from basecamp.generated.services.account import AccountService

        return self._service("account", lambda: AccountService(self))

    @property
    def projects(self):
        from basecamp.generated.services.projects import ProjectsService

        return self._service("projects", lambda: ProjectsService(self))

    @property
    def todos(self):
        from basecamp.generated.services.todos import TodosService

        return self._service("todos", lambda: TodosService(self))

    @property
    def todosets(self):
        from basecamp.generated.services.todosets import TodosetsService

        return self._service("todosets", lambda: TodosetsService(self))

    @property
    def todolists(self):
        from basecamp.generated.services.todolists import TodolistsService

        return self._service("todolists", lambda: TodolistsService(self))

    @property
    def todolist_groups(self):
        from basecamp.generated.services.todolist_groups import TodolistGroupsService

        return self._service("todolist_groups", lambda: TodolistGroupsService(self))

    @property
    def hill_charts(self):
        from basecamp.generated.services.hill_charts import HillChartsService

        return self._service("hill_charts", lambda: HillChartsService(self))

    @property
    def people(self):
        from basecamp.generated.services.people import PeopleService

        return self._service("people", lambda: PeopleService(self))

    @property
    def comments(self):
        from basecamp.generated.services.comments import CommentsService

        return self._service("comments", lambda: CommentsService(self))

    @property
    def messages(self):
        from basecamp.generated.services.messages import MessagesService

        return self._service("messages", lambda: MessagesService(self))

    @property
    def message_boards(self):
        from basecamp.generated.services.message_boards import MessageBoardsService

        return self._service("message_boards", lambda: MessageBoardsService(self))

    @property
    def message_types(self):
        from basecamp.generated.services.message_types import MessageTypesService

        return self._service("message_types", lambda: MessageTypesService(self))

    @property
    def webhooks(self):
        from basecamp.generated.services.webhooks_service import WebhooksService

        return self._service("webhooks", lambda: WebhooksService(self))

    @property
    def campfires(self):
        from basecamp.generated.services.campfires import CampfiresService

        return self._service("campfires", lambda: CampfiresService(self))

    @property
    def schedules(self):
        from basecamp.generated.services.schedules import SchedulesService

        return self._service("schedules", lambda: SchedulesService(self))

    @property
    def timesheets(self):
        from basecamp.generated.services.timesheets import TimesheetsService

        return self._service("timesheets", lambda: TimesheetsService(self))

    @property
    def vaults(self):
        from basecamp.generated.services.vaults import VaultsService

        return self._service("vaults", lambda: VaultsService(self))

    @property
    def documents(self):
        from basecamp.generated.services.documents import DocumentsService

        return self._service("documents", lambda: DocumentsService(self))

    @property
    def uploads(self):
        from basecamp.services.uploads import UploadsService

        return self._service("uploads", lambda: UploadsService(self))

    @property
    def attachments(self):
        from basecamp.generated.services.attachments import AttachmentsService

        return self._service("attachments", lambda: AttachmentsService(self))

    @property
    def recordings(self):
        from basecamp.generated.services.recordings import RecordingsService

        return self._service("recordings", lambda: RecordingsService(self))

    @property
    def events(self):
        from basecamp.generated.services.events import EventsService

        return self._service("events", lambda: EventsService(self))

    @property
    def card_tables(self):
        from basecamp.generated.services.card_tables import CardTablesService

        return self._service("card_tables", lambda: CardTablesService(self))

    @property
    def cards(self):
        from basecamp.generated.services.cards import CardsService

        return self._service("cards", lambda: CardsService(self))

    @property
    def card_columns(self):
        from basecamp.generated.services.card_columns import CardColumnsService

        return self._service("card_columns", lambda: CardColumnsService(self))

    @property
    def card_steps(self):
        from basecamp.generated.services.card_steps import CardStepsService

        return self._service("card_steps", lambda: CardStepsService(self))

    @property
    def checkins(self):
        from basecamp.generated.services.checkins import CheckinsService

        return self._service("checkins", lambda: CheckinsService(self))

    @property
    def forwards(self):
        from basecamp.generated.services.forwards import ForwardsService

        return self._service("forwards", lambda: ForwardsService(self))

    @property
    def templates(self):
        from basecamp.generated.services.templates import TemplatesService

        return self._service("templates", lambda: TemplatesService(self))

    @property
    def search(self):
        from basecamp.generated.services.search import SearchService

        return self._service("search", lambda: SearchService(self))

    @property
    def reports(self):
        from basecamp.generated.services.reports import ReportsService

        return self._service("reports", lambda: ReportsService(self))

    @property
    def timeline(self):
        from basecamp.generated.services.timeline import TimelineService

        return self._service("timeline", lambda: TimelineService(self))

    @property
    def tools(self):
        from basecamp.generated.services.tools import ToolsService

        return self._service("tools", lambda: ToolsService(self))

    @property
    def lineup(self):
        from basecamp.generated.services.lineup import LineupService

        return self._service("lineup", lambda: LineupService(self))

    @property
    def automation(self):
        from basecamp.generated.services.automation import AutomationService

        return self._service("automation", lambda: AutomationService(self))

    @property
    def subscriptions(self):
        from basecamp.generated.services.subscriptions import SubscriptionsService

        return self._service("subscriptions", lambda: SubscriptionsService(self))

    @property
    def boosts(self):
        from basecamp.generated.services.boosts import BoostsService

        return self._service("boosts", lambda: BoostsService(self))

    @property
    def client_approvals(self):
        from basecamp.generated.services.client_approvals import ClientApprovalsService

        return self._service("client_approvals", lambda: ClientApprovalsService(self))

    @property
    def client_correspondences(self):
        from basecamp.generated.services.client_correspondences import ClientCorrespondencesService

        return self._service("client_correspondences", lambda: ClientCorrespondencesService(self))

    @property
    def client_replies(self):
        from basecamp.generated.services.client_replies import ClientRepliesService

        return self._service("client_replies", lambda: ClientRepliesService(self))

    @property
    def client_visibility(self):
        from basecamp.generated.services.client_visibility import ClientVisibilityService

        return self._service("client_visibility", lambda: ClientVisibilityService(self))
