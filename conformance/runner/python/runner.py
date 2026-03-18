#!/usr/bin/env python3
"""Conformance test runner for the Python SDK.

Reads JSON test definitions from conformance/tests/ and executes
them against the SDK using respx for HTTP stubbing.
"""
from __future__ import annotations

import json
import re
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

import httpx
import respx

import basecamp
from basecamp import Client, Config, StaticTokenProvider
from basecamp.auth import BearerAuth
from basecamp.errors import BasecampError


@dataclass
class TestTracker:
    requests: list[dict] = field(default_factory=list)

    def record_request(self, *, time: float, method: str, url: str, headers: dict) -> None:
        self.requests.append({"time": time, "method": method, "url": url, "headers": headers})

    def reset(self) -> None:
        self.requests.clear()

    @property
    def request_count(self) -> int:
        return len(self.requests)

    @property
    def delays_between_requests(self) -> list[int]:
        if len(self.requests) < 2:
            return []
        return [
            int((b["time"] - a["time"]) * 1000)
            for a, b in zip(self.requests, self.requests[1:])
        ]


class ErrorMapper:
    """Used when client construction itself fails."""

    def __init__(self, error: Exception):
        self._error = error

    def __call__(self, *args: Any, **kwargs: Any) -> Any:
        raise self._error


class OperationMapper:
    """Maps conformance operation names to SDK calls."""

    def __init__(self, account_client):
        self._account = account_client

    def __call__(self, operation: str, *, path_params: dict, query_params: dict, body: dict | None) -> Any:
        match operation:
            case "ListProjects":
                return self._account.projects.list()
            case "GetProject":
                return self._account.projects.get(project_id=path_params["projectId"])
            case "CreateProject":
                return self._account.projects.create(name=body["name"])
            case "UpdateProject":
                return self._account.projects.update(project_id=path_params["projectId"], name=body["name"])
            case "TrashProject":
                return self._account.projects.trash(project_id=path_params["projectId"])
            case "ListTodos":
                return self._account.todos.list(todolist_id=path_params["todolistId"])
            case "GetTodo":
                return self._account.todos.get(todo_id=path_params["todoId"])
            case "CreateTodo":
                return self._account.todos.create(todolist_id=path_params["todolistId"], content=body["content"])
            case "GetTimesheetEntry":
                return self._account.timesheets.get(entry_id=path_params["entryId"])
            case "GetProjectTimeline":
                return self._account.timeline.get_project_timeline(project_id=path_params["projectId"])
            case "GetProjectTimesheet":
                return self._account.timesheets.for_project(project_id=path_params["projectId"])
            case "UpdateTimesheetEntry":
                return self._account.timesheets.update(
                    entry_id=path_params["entryId"],
                    date=body.get("date") if body else None,
                    hours=body.get("hours") if body else None,
                    description=body.get("description") if body else None,
                )
            case "ListWebhooks":
                return self._account.webhooks.list(bucket_id=path_params["bucketId"])
            case "CreateWebhook":
                return self._account.webhooks.create(
                    bucket_id=path_params["bucketId"],
                    payload_url=body["payload_url"],
                    types=body["types"],
                )
            case "GetProgressReport":
                return self._account.reports.progress()
            case "GetPersonProgress":
                return self._account.reports.person_progress(person_id=path_params["personId"])
            case "GetTool":
                return self._account.tools.get(tool_id=path_params["toolId"])
            case "CloneTool":
                return self._account.tools.clone(source_recording_id=body["source_recording_id"], title=body["title"])
            case "EnableTool":
                return self._account.tools.enable(tool_id=path_params["toolId"])
            case _:
                raise ValueError(f"Unknown operation: {operation}")


@dataclass
class TestResult:
    name: str
    passed: bool
    message: str | None = None


class TestRunner:
    def __init__(self, test_case: dict, tracker: TestTracker, mapper: Any):
        self._test = test_case
        self._tracker = tracker
        self._mapper = mapper

    def run(self) -> TestResult:
        self._tracker.reset()

        with respx.mock:
            self._setup_mock_responses()

            try:
                result = self._mapper(
                    self._test["operation"],
                    path_params=self._test.get("pathParams", {}),
                    query_params=self._test.get("queryParams", {}),
                    body=self._test.get("requestBody"),
                )
                return self._verify_assertions(result=result, error=None)
            except Exception as e:
                return self._verify_assertions(result=None, error=e)

    def _setup_mock_responses(self) -> None:
        responses = self._test.get("mockResponses", [])
        if not responses:
            return

        path = self._test["path"]
        for key, value in self._test.get("pathParams", {}).items():
            path = path.replace(f"{{{key}}}", str(value))

        method = (self._test.get("method") or "GET").upper()
        paginates = self._auto_paginates()
        response_queue = list(responses)
        call_count = [0]

        def side_effect(request: httpx.Request) -> httpx.Response:
            self._tracker.record_request(
                time=time.time(),
                method=str(request.method),
                url=str(request.url),
                headers=dict(request.headers),
            )
            idx = call_count[0]
            call_count[0] += 1

            if idx < len(response_queue):
                r = response_queue[idx]
                body = json.dumps(r.get("body", "")).encode() if r.get("body") is not None else b""
                headers = {"Content-Type": "application/json"}
                headers.update(r.get("headers", {}))
                return httpx.Response(r["status"], content=body, headers=headers)
            elif paginates:
                return httpx.Response(200, content=b"[]", headers={"Content-Type": "application/json"})
            else:
                return httpx.Response(500, content=b'{"error":"No more mock responses"}', headers={"Content-Type": "application/json"})

        respx.route(method=method, url__regex=f".*{re.escape(path)}.*").mock(side_effect=side_effect)

    def _auto_paginates(self) -> bool:
        return any(
            'rel="next"' in (r.get("headers", {}).get("Link", ""))
            for r in self._test.get("mockResponses", [])
        )

    def _verify_assertions(self, *, result: Any, error: Exception | None) -> TestResult:
        failures: list[str] = []

        for assertion in self._test.get("assertions", []):
            match assertion["type"]:
                case "requestCount":
                    actual = self._tracker.request_count
                    expected = assertion["expected"]
                    if self._auto_paginates():
                        if actual < expected:
                            failures.append(f"Expected >= {expected} requests, got {actual}")
                    elif actual != expected:
                        failures.append(f"Expected {expected} requests, got {actual}")

                case "delayBetweenRequests":
                    delays = self._tracker.delays_between_requests
                    min_delay = assertion.get("min")
                    if min_delay and delays and any(d < min_delay for d in delays):
                        failures.append(f"Expected minimum delay of {min_delay}ms, got {min(delays)}ms")

                case "noError":
                    if error:
                        failures.append(f"Expected no error, got: {type(error).__name__}: {error}")

                case "statusCode":
                    expected = assertion["expected"]
                    actual_status = getattr(error, "http_status", None) if error else None
                    if actual_status is not None:
                        if actual_status != expected:
                            failures.append(f"Expected status {expected}, got {actual_status}")
                    elif error and expected >= 400:
                        failures.append(f"Expected status {expected}, got error: {type(error).__name__}: {error}")
                    elif error and expected < 400:
                        failures.append(f"Expected success status {expected}, got error: {type(error).__name__}: {error}")
                    elif not error and expected >= 400:
                        failures.append(f"Expected error with status {expected}, but operation succeeded")

                case "responseBody":
                    path = assertion.get("path", "")
                    expected = assertion["expected"]
                    actual = _dig_path(result, path)
                    if actual != expected:
                        failures.append(f"Expected {path} to be {expected!r}, got {actual!r}")

                case "errorType":
                    expected_type = assertion["expected"]
                    if not error:
                        failures.append(f"Expected error type {expected_type!r}, but got no error")
                        continue
                    code_map = {
                        "not_found": "not_found",
                        "auth_required": "auth_required",
                        "forbidden": "forbidden",
                        "rate_limit": "rate_limit",
                        "validation": "validation",
                    }
                    expected_code = code_map.get(expected_type)
                    if expected_code is None:
                        failures.append(f"Unknown conformance error type {expected_type!r}")
                    elif hasattr(error, "code") and error.code != expected_code:
                        failures.append(f"Expected error code {expected_code!r}, got {error.code!r}")

                case "requestPath":
                    expected = assertion["expected"]
                    if not self._tracker.requests:
                        failures.append("Expected a request, but none recorded")
                    else:
                        actual_path = urlparse(self._tracker.requests[0]["url"]).path
                        if actual_path != expected:
                            failures.append(f"Expected request path {expected!r}, got {actual_path!r}")

                case "errorCode":
                    expected = assertion["expected"]
                    if not error:
                        failures.append(f"Expected error code {expected!r}, but got no error")
                    elif not hasattr(error, "code"):
                        failures.append(f"Expected error code {expected!r}, but error {type(error).__name__} has no code attribute")
                    elif error.code != expected:
                        failures.append(f"Expected error code {expected!r}, got {error.code!r}")

                case "errorMessage":
                    expected = assertion["expected"]
                    if not error:
                        failures.append(f"Expected error message containing {expected!r}, but got no error")
                    elif expected not in str(error):
                        failures.append(f"Expected error message containing {expected!r}, got {str(error)!r}")

                case "errorField":
                    field_path = assertion["path"]
                    expected = assertion["expected"]
                    if not error:
                        failures.append(f"Expected error field {field_path}, but got no error")
                        continue
                    actual = _get_error_field(error, field_path)
                    if actual != expected:
                        failures.append(f"Expected error.{field_path} = {expected!r}, got {actual!r}")

                case "headerInjected":
                    header_name = assertion["path"]
                    expected = assertion["expected"]
                    if not self._tracker.requests:
                        failures.append(f"Expected header {header_name}={expected!r}, but no requests recorded")
                    else:
                        actual = self._tracker.requests[0]["headers"].get(header_name.lower())
                        if actual != expected:
                            failures.append(f"Expected header {header_name}={expected!r}, got {actual!r}")

                case "headerPresent":
                    header_name = assertion["path"]
                    if not self._tracker.requests:
                        failures.append(f"Expected header {header_name} to be present, but no requests recorded")
                    else:
                        actual = self._tracker.requests[0]["headers"].get(header_name.lower())
                        if not actual:
                            failures.append(f"Expected header {header_name} to be present, but it was missing")

                case "requestScheme":
                    expected = assertion["expected"]
                    if expected == "https" and not error:
                        failures.append("Expected HTTPS enforcement error, but request succeeded")

                case "urlOrigin":
                    expected = assertion["expected"]
                    if expected == "rejected" and self._tracker.request_count > 1:
                        failures.append(f"Expected cross-origin rejection, but {self._tracker.request_count} requests made")

                case "responseMeta":
                    field_path = assertion["path"]
                    expected = assertion["expected"]
                    actual = None
                    if hasattr(result, "meta"):
                        # Convert camelCase field names to snake_case for Python attrs
                        snake_field = re.sub(r"([a-z])([A-Z])", r"\1_\2", field_path).lower()
                        actual = getattr(result.meta, snake_field, None)
                    if actual != expected:
                        failures.append(f"Expected responseMeta.{field_path} = {expected!r}, got {actual!r}")

                case unknown:
                    failures.append(f"Unknown assertion type: {unknown}")

        if failures:
            return TestResult(self._test["name"], False, "; ".join(failures))
        return TestResult(self._test["name"], True)


def _dig_path(obj: Any, path: str) -> Any:
    if not path:
        return obj
    for key in path.split("."):
        if obj is None:
            return None
        if isinstance(obj, dict):
            obj = obj.get(key)
        elif isinstance(obj, list):
            try:
                obj = obj[int(key)]
            except (ValueError, IndexError):
                return None
        else:
            obj = getattr(obj, key, None)
    return obj


def _get_error_field(error: Exception, field_path: str) -> Any:
    match field_path:
        case "httpStatus":
            return getattr(error, "http_status", None)
        case "retryable":
            return getattr(error, "retryable", None)
        case "requestId":
            return getattr(error, "request_id", None)
        case "code":
            return getattr(error, "code", None)
        case "message":
            return str(error)
        case _:
            return None


class ConformanceRunner:
    SKIPS: set[str] = {
        "maxItems caps results across pages",
    }
    SKIP_REASONS: dict[str, str] = {
        "maxItems caps results across pages": "Python SDK list methods don't expose a public max_items parameter",
    }

    def __init__(self, tests_dir: str):
        self._tests_dir = Path(tests_dir)
        self._tracker = TestTracker()

        config = Config(base_url="https://3.basecampapi.com")
        client = Client(config=config, access_token="conformance-test-token")
        self._account = client.for_account("999")
        self._mapper = OperationMapper(self._account)

    def _mapper_for_test(self, test_case: dict) -> Any:
        overrides = test_case.get("configOverrides")
        if not overrides:
            return self._mapper

        has_base_url = "baseUrl" in overrides
        has_max_pages = "maxPages" in overrides
        if not has_base_url and not has_max_pages:
            return self._mapper

        try:
            config_opts: dict[str, Any] = {"base_url": overrides["baseUrl"] if has_base_url else "https://3.basecampapi.com"}
            if has_max_pages:
                config_opts["max_pages"] = overrides["maxPages"]
            config = Config(**config_opts)
            client = Client(config=config, access_token="conformance-test-token")
            account = client.for_account("999")
            return OperationMapper(account)
        except Exception as e:
            return ErrorMapper(e)

    def run(self) -> int:
        files = sorted(self._tests_dir.glob("*.json"))
        if not files:
            print(f"No test files found in {self._tests_dir}")
            return 0

        passed = 0
        failed = 0
        skipped = 0

        for file in files:
            print(f"\n=== {file.name} ===")
            tests = json.loads(file.read_text())

            for test_case in tests:
                name = test_case["name"]

                if name in self.SKIPS:
                    skipped += 1
                    reason = self.SKIP_REASONS.get(name, "Python SDK behavior differs")
                    print(f"  SKIP: {name} ({reason})")
                    continue

                mapper = self._mapper_for_test(test_case)
                runner = TestRunner(test_case, self._tracker, mapper)
                result = runner.run()

                if result.passed:
                    passed += 1
                    print(f"  PASS: {result.name}")
                else:
                    failed += 1
                    print(f"  FAIL: {result.name}")
                    print(f"        {result.message}")

        print(f"\n{'=' * 40}")
        print(f"Results: {passed} passed, {failed} failed, {skipped} skipped")
        return 1 if failed > 0 else 0


if __name__ == "__main__":
    tests_dir = str(Path(__file__).parent.parent.parent / "tests")
    runner = ConformanceRunner(tests_dir)
    sys.exit(runner.run())
