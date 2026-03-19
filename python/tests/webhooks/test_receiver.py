from __future__ import annotations

import json

import pytest

from basecamp.webhooks.errors import WebhookVerificationError
from basecamp.webhooks.receiver import WebhookReceiver
from basecamp.webhooks.verify import compute_signature


def _make_event(kind: str, event_id: int | None = 1) -> bytes:
    event: dict = {"kind": kind}
    if event_id is not None:
        event["id"] = event_id
    return json.dumps(event).encode()


class TestHandleRequest:
    def test_handle_request_no_secret(self):
        receiver = WebhookReceiver()
        collected = []
        receiver.on("todo_created", lambda e: collected.append(e))

        body = _make_event("todo_created")
        result = receiver.handle_request(body, {})

        assert result["kind"] == "todo_created"
        assert len(collected) == 1

    def test_handle_request_with_signature(self):
        secret = "webhook-secret"
        receiver = WebhookReceiver(secret=secret)
        collected = []
        receiver.on("todo_created", lambda e: collected.append(e))

        body = _make_event("todo_created")
        sig = compute_signature(body, secret)
        headers = {"X-Basecamp-Signature": sig}

        result = receiver.handle_request(body, headers)

        assert result["kind"] == "todo_created"
        assert len(collected) == 1

    def test_handle_request_invalid_signature(self):
        receiver = WebhookReceiver(secret="real-secret")

        body = _make_event("todo_created")
        headers = {"X-Basecamp-Signature": "forged"}

        with pytest.raises(WebhookVerificationError):
            receiver.handle_request(body, headers)


class TestPatternMatching:
    def test_pattern_matching(self):
        receiver = WebhookReceiver()
        collected = []
        receiver.on("todo_*", lambda e: collected.append(e["kind"]))

        for kind in ("todo_created", "todo_completed", "todo_changed", "message_created"):
            body = _make_event(kind, event_id=None)
            receiver.handle_request(body, {})

        assert collected == ["todo_created", "todo_completed", "todo_changed"]


class TestOnAnyHandler:
    def test_on_any_handler(self):
        receiver = WebhookReceiver()
        collected = []
        receiver.on_any(lambda e: collected.append(e["kind"]))

        for i, kind in enumerate(("todo_created", "message_created", "comment_created")):
            body = _make_event(kind, event_id=i + 1)
            receiver.handle_request(body, {})

        assert collected == ["todo_created", "message_created", "comment_created"]


class TestDeduplication:
    def test_deduplication(self):
        receiver = WebhookReceiver()
        call_count = 0

        def handler(event):
            nonlocal call_count
            call_count += 1

        receiver.on("todo_created", handler)

        body = _make_event("todo_created", event_id=42)

        receiver.handle_request(body, {})
        receiver.handle_request(body, {})
        receiver.handle_request(body, {})

        assert call_count == 1


class TestMiddleware:
    def test_middleware(self):
        receiver = WebhookReceiver()
        order = []

        def mw_first(event, next_fn):
            order.append("first:before")
            next_fn()
            order.append("first:after")

        def mw_second(event, next_fn):
            order.append("second:before")
            next_fn()
            order.append("second:after")

        receiver.use(mw_first)
        receiver.use(mw_second)
        receiver.on("todo_created", lambda e: order.append("handler"))

        body = _make_event("todo_created")
        receiver.handle_request(body, {})

        assert order == [
            "first:before",
            "second:before",
            "handler",
            "second:after",
            "first:after",
        ]
