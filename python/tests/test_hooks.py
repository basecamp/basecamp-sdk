from __future__ import annotations

from basecamp.hooks import (
    BasecampHooks,
    OperationInfo,
    OperationResult,
    RequestInfo,
    RequestResult,
    chain_hooks,
    console_hooks,
    safe_hook,
)


class TestBasecampHooks:
    def test_noop_methods_dont_raise(self):
        h = BasecampHooks()
        info = OperationInfo(service="Test", operation="get")
        result = OperationResult(duration_ms=10)
        req = RequestInfo(method="GET", url="https://example.com")
        req_result = RequestResult(status_code=200, duration=0.1)

        h.on_operation_start(info)
        h.on_operation_end(info, result)
        h.on_request_start(req)
        h.on_request_end(req, req_result)
        h.on_retry(req, 2, RuntimeError("err"), 1.0)
        h.on_paginate("https://example.com/page2", 2)


class TestChainHooks:
    def test_calls_all_hooks_in_order(self):
        calls = []

        class RecordingHook(BasecampHooks):
            def __init__(self, name):
                self.name = name

            def on_request_start(self, info):
                calls.append(self.name)

        h = chain_hooks(RecordingHook("a"), RecordingHook("b"), RecordingHook("c"))
        h.on_request_start(RequestInfo(method="GET", url="/"))
        assert calls == ["a", "b", "c"]

    def test_single_hook_returned_directly(self):
        h = BasecampHooks()
        assert chain_hooks(h) is h

    def test_exception_in_one_hook_does_not_block_others(self):
        calls = []

        class GoodHook(BasecampHooks):
            def on_request_start(self, info):
                calls.append("good")

        class BadHook(BasecampHooks):
            def on_request_start(self, info):
                raise RuntimeError("boom")

        h = chain_hooks(BadHook(), GoodHook())
        h.on_request_start(RequestInfo(method="GET", url="/"))
        assert calls == ["good"]


class TestConsoleHooks:
    def test_writes_to_stderr(self, capsys):
        h = console_hooks()
        info = OperationInfo(service="Projects", operation="list")
        h.on_operation_start(info)
        captured = capsys.readouterr()
        assert "Projects.list started" in captured.err

    def test_request_end_prints_status(self, capsys):
        h = console_hooks()
        req = RequestInfo(method="GET", url="https://example.com/test")
        h.on_request_end(req, RequestResult(status_code=200, duration=0.123))
        captured = capsys.readouterr()
        assert "200" in captured.err


class TestSafeHook:
    def test_swallows_exception(self):
        def bad_hook():
            raise RuntimeError("boom")

        safe_hook(bad_hook)  # should not raise

    def test_calls_function_normally(self):
        called = []
        safe_hook(lambda: called.append(True))
        assert called == [True]
