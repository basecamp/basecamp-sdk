from __future__ import annotations

import json
import re
import threading
from collections import OrderedDict
from collections.abc import Callable
from typing import Any

from basecamp.webhooks.errors import WebhookVerificationError
from basecamp.webhooks.verify import verify_signature


class WebhookReceiver:
    """Receives and routes webhook events from Basecamp.

    Framework-agnostic: works with raw body bytes and a header dict.
    """

    DEFAULT_SIGNATURE_HEADER = "X-Basecamp-Signature"
    DEFAULT_DEDUP_WINDOW_SIZE = 1000

    def __init__(
        self,
        *,
        secret: str | None = None,
        signature_header: str = DEFAULT_SIGNATURE_HEADER,
        dedup_window_size: int = DEFAULT_DEDUP_WINDOW_SIZE,
    ) -> None:
        self._secret = secret
        self._signature_header = signature_header
        self._dedup_window_size = dedup_window_size
        self._handlers: dict[str, list[Callable]] = {}
        self._any_handlers: list[Callable] = []
        self._middleware: list[Callable] = []
        self._dedup_seen: OrderedDict[int, bool] = OrderedDict()
        self._dedup_pending: dict[int, bool] = {}
        self._lock = threading.Lock()

    def on(self, pattern: str, handler: Callable) -> WebhookReceiver:
        """Register a handler for a specific event kind pattern.

        Supports glob patterns: "todo_*" matches "todo_created", etc.
        """
        self._handlers.setdefault(pattern, []).append(handler)
        return self

    def on_any(self, handler: Callable) -> WebhookReceiver:
        """Register a handler for all events."""
        self._any_handlers.append(handler)
        return self

    def use(self, middleware: Callable) -> WebhookReceiver:
        """Add middleware to the processing chain.

        Middleware receives (event, next_fn) and must call next_fn() to continue.
        """
        self._middleware.append(middleware)
        return self

    def handle_request(self, raw_body: bytes, headers: dict[str, str]) -> dict[str, Any]:
        """Process a raw webhook request.

        Verifies signature, parses JSON, deduplicates, and dispatches handlers.
        Raises WebhookVerificationError if signature is invalid.
        """
        if self._secret:
            signature = self._extract_header(headers, self._signature_header)
            if not verify_signature(raw_body, self._secret, signature or ""):
                raise WebhookVerificationError

        event: dict[str, Any] = json.loads(raw_body)
        event_id = event.get("id")

        if not self._claim(event_id):
            return event

        try:

            def run_handlers() -> None:
                return self._dispatch_handlers(event)

            chain = run_handlers
            for mw in reversed(self._middleware):
                outer_next = chain

                def chain(_mw=mw, _next=outer_next) -> None:
                    return _mw(event, _next)

            chain()
            self._commit_seen(event_id)
        except Exception:
            self._release_claim(event_id)
            raise

        return event

    def _extract_header(self, headers: dict[str, str], name: str) -> str | None:
        """Look up a header by exact match, then case-insensitive."""
        if name in headers:
            return headers[name]
        lower = name.lower()
        for key, value in headers.items():
            if key.lower() == lower:
                return value
        return None

    def _claim(self, event_id: Any) -> bool:
        """Atomically claim an event for processing. Returns False if already seen or in-flight."""
        if self._dedup_window_size <= 0 or event_id is None:
            return True

        with self._lock:
            if event_id in self._dedup_seen or event_id in self._dedup_pending:
                return False
            self._dedup_pending[event_id] = True
            return True

    def _commit_seen(self, event_id: Any) -> None:
        """Promote from pending to seen after successful handling."""
        if self._dedup_window_size <= 0 or event_id is None:
            return

        with self._lock:
            self._dedup_pending.pop(event_id, None)
            if len(self._dedup_seen) >= self._dedup_window_size:
                self._dedup_seen.popitem(last=False)
            self._dedup_seen[event_id] = True

    def _release_claim(self, event_id: Any) -> None:
        """Release claim so retries can re-attempt."""
        if event_id is None:
            return

        with self._lock:
            self._dedup_pending.pop(event_id, None)

    def _dispatch_handlers(self, event: dict[str, Any]) -> None:
        """Match and invoke handlers for the event."""
        kind = event.get("kind")
        matched: list[Callable] = []

        for pattern, handlers in self._handlers.items():
            if self._match_pattern(pattern, kind):
                matched.extend(handlers)

        matched.extend(self._any_handlers)

        for handler in matched:
            handler(event)

    @staticmethod
    def _match_pattern(pattern: str, value: str | None) -> bool:
        """Match a glob pattern against an event kind string."""
        if value is None:
            return False
        if pattern == value:
            return True

        regex = "\\A" + ".*".join(re.escape(part) for part in pattern.split("*")) + "\\Z"
        return re.match(regex, value) is not None
