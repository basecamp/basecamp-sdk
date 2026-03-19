from __future__ import annotations

import sys
from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class OperationInfo:
    service: str
    operation: str
    resource_type: str | None = None
    is_mutation: bool = False
    project_id: str | int | None = None
    resource_id: str | int | None = None


@dataclass(frozen=True)
class OperationResult:
    duration_ms: int = 0
    error: BaseException | None = None


@dataclass(frozen=True)
class RequestInfo:
    method: str
    url: str
    attempt: int = 1


@dataclass(frozen=True)
class RequestResult:
    status_code: int | None = None
    duration: float = 0.0
    error: BaseException | None = None
    retry_after: int | None = None


class BasecampHooks:
    """Base class for observability hooks. Override methods as needed."""

    def on_operation_start(self, info: OperationInfo) -> None:
        pass

    def on_operation_end(self, info: OperationInfo, result: OperationResult) -> None:
        pass

    def on_request_start(self, info: RequestInfo) -> None:
        pass

    def on_request_end(self, info: RequestInfo, result: RequestResult) -> None:
        pass

    def on_retry(self, info: RequestInfo, attempt: int, error: BaseException, delay: float) -> None:
        pass

    def on_paginate(self, url: str, page: int) -> None:
        pass


class _ChainedHooks(BasecampHooks):
    def __init__(self, hooks: tuple[BasecampHooks, ...]):
        self._hooks = hooks

    def on_operation_start(self, info: OperationInfo) -> None:
        for h in self._hooks:
            _safe_call(h.on_operation_start, info)

    def on_operation_end(self, info: OperationInfo, result: OperationResult) -> None:
        for h in self._hooks:
            _safe_call(h.on_operation_end, info, result)

    def on_request_start(self, info: RequestInfo) -> None:
        for h in self._hooks:
            _safe_call(h.on_request_start, info)

    def on_request_end(self, info: RequestInfo, result: RequestResult) -> None:
        for h in self._hooks:
            _safe_call(h.on_request_end, info, result)

    def on_retry(self, info: RequestInfo, attempt: int, error: BaseException, delay: float) -> None:
        for h in self._hooks:
            _safe_call(h.on_retry, info, attempt, error, delay)

    def on_paginate(self, url: str, page: int) -> None:
        for h in self._hooks:
            _safe_call(h.on_paginate, url, page)


def chain_hooks(*hooks: BasecampHooks) -> BasecampHooks:
    """Compose multiple hooks into a single hooks instance."""
    if len(hooks) == 1:
        return hooks[0]
    return _ChainedHooks(hooks)


class _ConsoleHooks(BasecampHooks):
    def on_operation_start(self, info: OperationInfo) -> None:
        print(f"[basecamp] {info.service}.{info.operation} started", file=sys.stderr)

    def on_operation_end(self, info: OperationInfo, result: OperationResult) -> None:
        status = "error" if result.error else "ok"
        print(f"[basecamp] {info.service}.{info.operation} {status} ({result.duration_ms}ms)", file=sys.stderr)

    def on_request_start(self, info: RequestInfo) -> None:
        print(f"[basecamp] {info.method} {info.url} (attempt {info.attempt})", file=sys.stderr)

    def on_request_end(self, info: RequestInfo, result: RequestResult) -> None:
        print(f"[basecamp] {info.method} {info.url} → {result.status_code} ({result.duration:.3f}s)", file=sys.stderr)

    def on_retry(self, info: RequestInfo, attempt: int, error: BaseException, delay: float) -> None:
        print(
            f"[basecamp] retrying {info.method} {info.url} (attempt {attempt}, delay {delay:.1f}s): {error}",
            file=sys.stderr,
        )

    def on_paginate(self, url: str, page: int) -> None:
        print(f"[basecamp] paginate page {page}: {url}", file=sys.stderr)


def console_hooks() -> BasecampHooks:
    """Create hooks that log to stderr."""
    return _ConsoleHooks()


def _safe_call(fn, *args: Any) -> None:
    try:
        fn(*args)
    except Exception as e:
        print(f"Basecamp hook error: {type(e).__name__}: {e}", file=sys.stderr)


def safe_hook(fn, *args: Any) -> None:
    """Call a hook method, swallowing any exceptions."""
    _safe_call(fn, *args)
