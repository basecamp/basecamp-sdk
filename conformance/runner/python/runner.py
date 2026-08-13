#!/usr/bin/env python3
"""Conformance test runner for the Python SDK.

Reads JSON test definitions from conformance/tests/ and executes
them against the SDK using respx for HTTP stubbing.
"""
from __future__ import annotations

import json
import os
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

# Wire keys for todo write operations; identical to the Python kwarg /
# edit-attribute names, so fixtures map onto the SDK surface directly.
_TODO_WRITE_FIELDS = ("content", "description", "assignee_ids", "completion_subscriber_ids", "due_on", "starts_on", "notify")
# The todolist writable set is exactly this pair, and it is the same pair for a
# todolist group — the composite is deliberately variant-agnostic, so nothing
# downstream branches on which shape came back from the GET.
_TODOLIST_WRITE_FIELDS = ("name", "description")
_DOCUMENT_WRITE_FIELDS = ("title", "content")
_SCHEDULE_ENTRY_WRITE_FIELDS = ("summary", "starts_at", "ends_at", "description", "all_day", "participant_ids", "notify", "url", "highlighted")
# Create takes `status` too — it is a Recording column, so BC3 reads it outside
# the schedule_entry envelope and it is not a ReplaceScheduleEntry member.
_SCHEDULE_ENTRY_CREATE_FIELDS = _SCHEDULE_ENTRY_WRITE_FIELDS + ("status",)
_CARD_WRITE_FIELDS = ("title", "content", "due_on", "assignee_ids")

# Sentinel distinguishing "key absent from the JSON body" from a present None.
_MISSING = object()


# =============================================================================
# Case census (#602)
# =============================================================================
#
# Every non-live fixture case must be accounted for by the run::
#
#     passed + failed + skipped  ==  cases in conformance/tests/**/*.json
#                                    whose mode != "live"
#
# The left side is what the runner actually did. The right side is counted by
# ``count_non_live_cases`` below — a SEPARATE walk and parse, deliberately not
# the runner's own load path. That independence is the entire point: a check fed
# by the load path can only confirm the load path agrees with itself.
#
# Why ``mode != "live"`` rather than ``mode == "mock"``: all six runners select
# with "mock unless told otherwise" (``is_mock_mode`` here, and its five
# equivalents), so a typo'd ``mode: "moc"`` is dropped by every runner at once
# with nothing printed anywhere. Counting the expected side as "not explicitly
# live" turns that silent divergence into arithmetic.
#
# Catches: an unrecognized ``mode``; a fixture that failed to parse or was never
# globbed (including one nested below ``conformance/tests/``, which no runner
# discovers — hence the recursive walk); a case dropped between load and
# dispatch; a fixture emptied to ``[]`` (which the census REFUSES rather than
# counts — see ``count_non_live_cases``, and note that counting it would make
# this bullet a lie); and any future skip channel that bypasses the counters,
# because the counters are what it reads.
#
# The typo is not this check's alone to catch, and saying so is what keeps the
# rest of the list honest: ``make conformance-fixtures-check`` validates
# ``conformance/tests/*.json`` against ``conformance/schema.json``, whose
# ``mode`` is ``enum: ["mock", "live"]``, so a typo in a TOP-LEVEL fixture fails
# there first and this census is defense in depth for that one case. What that
# gate structurally cannot see is everything else above — its glob is not
# recursive, so a fixture nested below ``conformance/tests/`` is validated by
# nothing AND run by nothing (verified: such a file passes the schema gate and
# fails this census); a fixture truncated to ``[]`` is a valid array of zero
# cases; and a case dropped between load and dispatch is not a fixture-format
# question at all. Nor does that gate run when ``make conformance-<lang>`` is
# invoked alone.
#
# Does NOT catch the all-six case #602 names — one case every runner excludes
# for its own reason, which leaves each runner's own census green. That needs
# the six exclusion sets in one place, hence artifact plumbing across six CI
# jobs; #602 stays open for it.


def is_mock_mode(mode: str | None) -> bool:
    """Whether a fixture case's ``mode`` selects this runner.

    Absent means mock: live cases are TS-only (the canonical wire-capturer), and
    every other value is nobody's. Shared with the census self-tests so the rule
    the run loop applies is the rule under test, not a copy of it.

    Defaults on ``None`` ONLY, not on falsiness. ``(mode or "mock")`` reads
    ``"mode": ""`` as an absent key and runs it, where the four null-coalescing
    runners refuse it — and since the census counts ``""`` as non-live either
    way, the divergence this check exists to expose would have stayed green here.
    """
    return (mode if mode is not None else "mock") == "mock"


def _fixture_files(tests_dir: str | Path) -> list[Path]:
    """Every ``*.json`` under ``tests_dir``, recursively, sorted by path.

    ``os.walk`` with ``onerror``, NOT ``Path.rglob``: rglob suppresses the
    ``OSError`` raised while scanning, so an unreadable subdirectory is simply
    omitted. The runner's non-recursive glob omits it too, so its cases leave
    both sides of the census at once and the totals still agree — a fail-closed
    walk failing open, which is the one failure this function must not have.
    """

    def onerror(err: OSError) -> None:
        raise RuntimeError(f"could not walk {getattr(err, 'filename', tests_dir)}: {err}")

    files: list[Path] = []
    for root, _dirs, names in os.walk(tests_dir, onerror=onerror):
        files.extend(Path(root) / name for name in names if name.endswith(".json"))
    return sorted(files)


def count_non_live_cases(tests_dir: str | Path) -> int:
    """Count fixture cases whose mode is not ``"live"``, recursively.

    Fail-closed in four places, each a way the count could certify nothing while
    looking green: an unreadable tree, a fixture that does not parse, a fixture
    emptied to ``[]``, and a walk that found no fixture files at all.
    """
    files = _fixture_files(tests_dir)
    if not files:
        raise RuntimeError(f"no *.json fixture files found under {tests_dir}")

    cases = 0
    for file in files:
        try:
            parsed = json.loads(file.read_text())
        except (json.JSONDecodeError, OSError) as e:
            raise RuntimeError(f"{file}: {e}") from e
        if not isinstance(parsed, list):
            raise RuntimeError(f"{file}: fixture is not a JSON array")
        # An emptied fixture is REFUSED, not counted as zero, and this is the
        # one rejection that carries the whole-file guarantee. It is the single
        # truncation both sides of the census read identically: the runner
        # registers nothing from the file and the census expects nothing, so the
        # two totals fall together and no mismatch ever appears. Counting it
        # would make "a fixture truncated to []" a claim this check cannot keep.
        # A file declaring no cases tests nothing, so refusing it costs nothing
        # — and it closes the same hole in conformance-fixtures-check, where an
        # empty array is a schema-valid list of zero items.
        if not parsed:
            raise RuntimeError(
                f"{file}: fixture declares no cases; delete the file or restore its cases"
            )
        # Only ``mode`` is read: the census must survive a fixture whose other
        # fields this runner cannot model, or it would report a failure for a
        # case the run itself handled fine.
        cases += sum(
            1 for case in parsed if not isinstance(case, dict) or case.get("mode") != "live"
        )
    return cases


def write_execution_manifest(runner: str, total: int, executed: int,
                             excluded: list[tuple[str, str, str]]) -> None:
    """Write one runner's exclusion set for the cross-runner gate (#602).

    The case census answers "did THIS runner account for every case". A case
    every runner excludes leaves all six censuses green, because each counted
    its own skip — only scripts/check-fixture-execution.rb, comparing these
    manifests, can see it.

    ``executed`` is recorded alongside the exclusions and asserted against the
    census total: without it, a case a runner silently dropped is simply absent
    from the exclusion set, and "absent" reads identically to "ran fine".

    Sorted, so a re-run is byte-identical.
    """
    if executed + len(excluded) != total:
        raise RuntimeError(
            f"manifest for {runner} is internally inconsistent: {executed} executed + "
            f"{len(excluded)} excluded != {total} non-live cases; the run dropped a case "
            "without recording it as either"
        )

    path = Path(__file__).resolve().parents[3] / "conformance" / "manifests"
    path.mkdir(parents=True, exist_ok=True)
    body = {
        "runner": runner,
        "total_non_live": total,
        "executed": executed,
        "excluded": [{"file": f, "name": n, "reason": r} for f, n, r in sorted(excluded)],
    }
    (path / f"{runner}.json").write_text(json.dumps(body, indent=2) + "\n")


def case_count_failure(ran: int, expected: int) -> str | None:
    """Compare what the run accounted for against the census; None when equal."""
    if ran == expected:
        return None
    if ran < expected:
        return (
            f"case census: the run accounted for {ran} case(s) (passed+failed+skipped) "
            f"but conformance/tests holds {expected} non-live case(s) — "
            f"{expected - ran} executed by nothing. An unrecognized `mode`, a fixture "
            "that failed to parse or was never globbed, or a case dropped between load "
            "and dispatch will do this."
        )
    return (
        f"case census: the run accounted for {ran} case(s) (passed+failed+skipped) "
        f"but conformance/tests holds only {expected} non-live case(s) — "
        f"{ran - expected} more than the fixtures declare."
    )


def error_raised_failure(dispatch_failed: bool) -> str | None:
    """Validate one ``errorRaised`` assertion; None when it holds.

    The inverse of noError, and deliberately code-agnostic. The
    malformed-response family (#576) is refused by a hand-written guard in
    TypeScript, Python and Ruby and by the model decoder in Go, Kotlin and
    Swift; those two mechanisms share no canonical error code, so pinning
    errorType would make the fixture unwritable. What all six agree on is that
    the call fails at all — which, paired with requestCount, is the whole
    contract: the composite refused the field instead of writing it.

    Split out of the assertion loop so the failing branch is unit-testable. NO
    COMMITTED FIXTURE CAN REACH IT: every case declaring errorRaised is one the
    SDK does refuse, so a handler that accepted everything would report green in
    all six runners at once. That is the #563 shape — a delayBetweenRequests
    check that passed vacuously because no fixture supplied a gap it could fail
    on — and the reason ``make conformance-runner-tests`` exists.

    The message is pinned verbatim by the unit tests in all six runners: a
    fixture debugged in one language should not read differently in another.
    """
    return None if dispatch_failed else "Expected the call to fail, but it succeeded"


def check_delay_gaps(
    delays: list[float], min_delay: float | None, index: int | None, request_count: int
) -> str | None:
    """Validate one ``delayBetweenRequests`` assertion; None when it holds.

    ``delays`` is the list of inter-request gaps, so N requests yield N-1
    entries and gap i is the interval between request i and request i+1.
    The contract in conformance/schema.json:

    * A NAMED index selects exactly that gap, bounds-checked unconditionally.
      A gap the run never produced is a failure, not a silent pass — the whole
      point of a timing pin is to catch a dropped backoff, and a dropped
      backoff is precisely what removes the gap.
    * An OMITTED index requires the minimum on EVERY gap. Zero gaps means
      nothing was measured, so that fails too: an "every gap" rule with no
      gaps left would otherwise wave through a run that dropped every retry.
    * Negative indexes are rejected rather than wrapping to the end the way
      the per-request assertions do. There is no sensible "last gap" when the
      point of naming one is to pin a specific backoff.

    An absent or zero ``min_delay`` still asserts that the gap EXISTS. The
    default is applied HERE rather than at the call site so a truthiness gate
    (``if min_delay:``, which discards a legitimate ``min: 0``) cannot quietly
    reduce the assertion to nothing — the false-green class this exists to kill.

    ``request_count`` is passed rather than inferred as ``len(delays) + 1``:
    that inference assumes at least one request, so a run that failed before
    dispatching anything would report "only 1 request(s) were made".
    """
    min_delay = 0 if min_delay is None else min_delay

    if index is not None:
        if index < 0:
            return f"delayBetweenRequests gap index must be non-negative, got {index}"
        if index >= len(delays):
            return f"Expected a delay at gap {index}, but only {request_count} request(s) were made"
        if delays[index] < min_delay:
            return f"Expected minimum delay of {min_delay}ms at gap {index}, got {delays[index]}ms"
        return None

    if not delays:
        return f"Expected a delay between requests, but only {request_count} request(s) were made"
    for i, delay in enumerate(delays):
        if delay < min_delay:
            return f"Expected minimum delay of {min_delay}ms at gap {i}, got {delay}ms"
    return None


def check_request_count(actual: int, expected: int) -> str | None:
    """Validate one ``requestCount`` assertion; None when it holds.

    EXACT, always — including the auto-paginating fixtures. The runner used to
    relax this to a lower bound whenever any mock response carried
    ``Link: rel="next"``, on the theory that an auto-paginating SDK would
    legitimately make more requests than the fixture named. That is backwards
    for the fixtures the relaxation covered: in conformance/tests/pagination.json,
    "Pagination stops at maxPages safety cap" and "maxItems caps results across
    pages" each queue THREE pages and expect TWO requests, because stopping
    early is the behavior under test. ``>=`` passes an SDK that ignored the cap
    and walked every page. "Auto-pagination follows Link headers across multiple
    pages" is the exposed case: its only assertions are requestCount and
    noError, so an over-fetch has nothing else to catch it.

    The one fixture where the count genuinely does not apply to an
    auto-paginating SDK — "List operation returns first page with Link header",
    which asserts a single request — carries the ``link-header`` tag, and
    ``request_count_applies`` reports False for it. Nothing that still reaches
    this function needs the relaxation.

    Swift took this in #558; #573 is the same fix for the other five runners.
    """
    if actual != expected:
        return f"Expected {expected} requests, got {actual}"
    return None


#: Marks a fixture whose requestCount counts first-page requests only, which an
#: auto-paginating SDK cannot satisfy.
LINK_HEADER_TAG = "link-header"


def request_count_applies(tags: list[str]) -> bool:
    """Whether a fixture's ``requestCount`` assertion is meaningful for this SDK.

    SCOPE: this suppresses ONE ASSERTION, not the whole test case. An earlier
    revision skipped the entire ``link-header`` case in every runner, which took
    its ``statusCode: 200`` and ``noError`` assertions down with the
    inapplicable ``requestCount`` — Kotlin and Swift had always skipped the case
    wholesale, so once Go, Python, Ruby and TypeScript joined them the fixture
    was executed by nothing at all while still sitting in
    conformance/tests/pagination.json, passing conformance-fixtures-check and
    check-fixture-coverage. That is the #572 shape ("present, run by nothing")
    one layer down. Only the count is inapplicable; the status code and the
    absence of an error are not, and they are the assertions that catch an
    auto-paginating SDK that walked the Link header into an error.
    """
    return LINK_HEADER_TAG not in tags


@dataclass
class TestTracker:
    requests: list[dict] = field(default_factory=list)

    def record_request(self, *, monotonic_time: float, method: str, url: str, headers: dict, body: Any = None) -> None:
        self.requests.append({"monotonic_time": monotonic_time, "method": method, "url": url, "headers": headers, "body": body})

    def reset(self) -> None:
        self.requests.clear()

    @property
    def request_count(self) -> int:
        return len(self.requests)

    @property
    def delays_between_requests(self) -> list[int]:
        # Elapsed ms between consecutive requests, from monotonic captures.
        # Wall-clock (time.time) can step or slew mid-test and read a sleep
        # as shorter than it was — the delay-flake class from #496. Monotonic
        # deltas mirror the Go runner's time.Time subtraction.
        if len(self.requests) < 2:
            return []
        return [
            int((b["monotonic_time"] - a["monotonic_time"]) * 1000)
            for a, b in zip(self.requests, self.requests[1:])
        ]


class ErrorMapper:
    """Used when client construction itself fails."""

    def __init__(self, error: Exception):
        self._error = error

    def __call__(self, *args: Any, **kwargs: Any) -> Any:
        raise self._error


# The date window every GetUpcomingSchedule case is dispatched with. Fixed in
# the runner because no mock runner consumes query_params and no assertion type
# can pin a query string — every runner records the path with the query stripped.
UPCOMING_WINDOW_START = "2026-06-01"
UPCOMING_WINDOW_END = "2026-06-30"


# attachable_sgid is passed explicitly by the dispatch (it is required), so it
# is deliberately absent here — this list is only the presence-bearing members,
# where "sent as empty" and "not sent" are different writes.
_UPLOAD_VERSION_WRITE_FIELDS = ("base_name", "description", "notify", "subscriptions")


def _summarize_upload_versions(versions: list) -> dict:
    """Flatten the versions array into top-level scalars.

    GET /uploads/{id}/versions.json returns an ARRAY, and a responseBody path
    resolves as a top-level key only, so the assertions cannot walk into it.
    Same shape as _summarize_upcoming, for the same reason.
    """
    summary: dict[str, Any] = {
        "versions_count": len(versions),
        "current_count": sum(1 for v in versions if (v.get("upload") or {}).get("current")),
    }
    if versions:
        first = versions[0]
        first_upload = first.get("upload") or {}
        summary["first_action"] = first["action"]
        summary["first_filename"] = first_upload.get("filename")
        summary["first_content_type"] = first_upload.get("content_type")
        summary["first_byte_size"] = first_upload.get("byte_size")
        summary["first_current"] = first_upload.get("current")

        last = versions[-1]
        summary["last_action"] = last["action"]
        # A version whose recordable no longer resolves omits the upload object
        # entirely — the optionality UploadVersion.upload declares.
        summary["last_has_upload"] = last.get("upload") is not None
    return summary


def _summarize_upcoming(envelope: dict) -> dict:
    """Flatten the upcoming-schedule envelope into top-level scalars.

    Go and TypeScript resolve a responseBody path as a top-level key only, so
    the assertions have to read scalars rather than walk into the arrays. Python
    is lenient — `upcoming` hands back the parsed body verbatim — so this reads
    the wire keys directly; the strict tiers (Swift, Kotlin) build the same
    summary out of decoded models, which is where the contract is enforced.
    """
    entries = envelope["schedule_entries"]
    occurrences = envelope["recurring_schedule_entry_occurrences"]
    assignables = envelope["assignables"]

    summary: dict[str, Any] = {
        "schedule_entries_count": len(entries),
        "recurring_occurrences_count": len(occurrences),
        "assignables_count": len(assignables),
    }
    if entries:
        summary["entry_summary"] = entries[0]["summary"]
        summary["entry_recurring"] = entries[0]["recurring"]
        summary["entry_bucket_name"] = entries[0]["bucket"]["name"]
    if occurrences:
        summary["occurrence_recurring"] = occurrences[0]["recurring"]
        summary["occurrence_all_day"] = occurrences[0]["all_day"]
        summary["occurrence_starts_at"] = occurrences[0]["starts_at"]
    if assignables:
        summary["assignable_content"] = assignables[0]["content"]
        summary["assignable_type"] = assignables[0]["type"]
        summary["assignable_parent_title"] = assignables[0]["parent"]["title"]
        summary["assignable_completion_url"] = assignables[0]["completion_url"]
    return summary


def _normalize_body(body: Any, status: int | None) -> Any:
    """Normalize a mock response body for SDK compatibility.

    Conformance test fixtures may wrap arrays in objects (e.g.,
    ``{"projects": [...]}``), but the Python SDK's list operations expect a raw
    JSON array. When the body is a JSON object with a single key whose value is
    an array, unwrap it — matching the other runners' semantics.

    Success bodies only: an error body with one array-valued key is the
    unwrapped field map (``{"payload_url": ["is invalid"]}``), and unwrapping
    it would rewrite the fixture on the wire.
    """
    if (status or 200) < 400 and isinstance(body, dict) and len(body) == 1:
        (value,) = body.values()
        if isinstance(value, list):
            return value
    return body


def _project_id(item: Any) -> Any:
    """The ``id`` of a list item, or 0 when the item is not an object.

    Single-key envelope bodies (retry.json's legacy ``{"projects": []}``) no
    longer reach the SDK — ``_normalize_body`` unwraps them at the mock, in
    parity with the other runners — so list items are expected to be dicts.
    The guard remains as a backstop for any future fixture whose items are not
    objects.
    """
    return item.get("id", 0) if isinstance(item, dict) else 0


def _summarize_projects(result: Any) -> dict[str, Any]:
    """Flatten an accumulated project list into top-level scalars.

    Flat and scalar because that is the only path form every runner can resolve:
    Go and TypeScript read a responseBody path as a top-level key with no dot
    splitting, and the Swift and Kotlin navigators descend through objects only,
    so neither a dotted path nor an array index is portable.

    It exists so a fixture can prove the items of a followed page were
    ACCUMULATED, not merely fetched. requestCount only sees that the second
    request happened, and meta.totalCount is the X-Total-Count header rather
    than the item count, so an SDK that fetched page 2 and discarded its body
    satisfies both.

    The returned dict replaces the ListResult for BOTH assertion families, so it
    carries the two whitelisted responseMeta fields under their JSON names as
    well; the responseMeta arm falls back to a mapping lookup when the result
    has no `.meta` (mirroring the Ruby runner's Hash fallback).
    """
    items = list(result)
    return {
        "project_count": len(items),
        "first_project_id": _project_id(items[0]) if items else 0,
        "last_project_id": _project_id(items[-1]) if items else 0,
        "totalCount": result.meta.total_count,
        "truncated": result.meta.truncated,
    }


# The query every Search case is dispatched with. Fixed in the runner for the
# same reason UPCOMING_WINDOW_START is: no mock runner consumes query_params and
# no assertion type can pin a query string.
SEARCH_QUERY = "Leto"


def _summarize_search(result: Any) -> dict[str, Any]:
    """Flatten a search result list into top-level scalars, one group per branch
    of BC3's polymorphic search projection.

    Flat and scalar for the reason _summarize_projects gives. Boolean for a
    second reason: the response is an ARRAY and no assertion type expresses
    absence inside one — there is headerAbsent and requestBodyAbsent, but no
    responseBodyAbsent — and the file-attachment branch is recognized precisely
    BY the absence of the five envelope keys. Encoding that as a boolean is the
    established idiom (last_has_upload).

    Each hit is selected by predicate rather than by index, so a fixture can
    present one branch alone and still assert the others report honestly.

    Python is lenient — `search` hands back the parsed dicts verbatim — so this
    reads the wire keys directly; the strict tiers (Swift, Kotlin) build the same
    summary out of decoded models, which is where the contract is enforced.
    """
    results = list(result)

    def find(pred) -> dict:
        return next((r for r in results if pred(r)), {})

    generic = find(lambda r: r.get("type") is not None)
    attachment = find(lambda r: r.get("type") is None)
    upload_line = find(lambda r: r.get("type") == "Chat::Lines::Upload")
    needle = find(lambda r: r.get("type") == "Gauge::Needle")
    kanban = find(lambda r: r.get("type") == "Kanban::Column")

    upload_attachment = (upload_line.get("attachments") or [{}])[0]
    needle_attachment = (needle.get("attachments") or [{}])[0]

    def narrow(value) -> int:
        # Narrowed HERE, not by the SDK: Python has no typed search model, so a
        # float-spelled 1920.0 reaches the caller as a float. The narrowing is
        # load-bearing in the statically-typed tiers (Go, Kotlin, Swift), where
        # the model declares an integer and a plain int decode would throw.
        return int(value) if value is not None else 0

    return {
        "result_count": len(results),
        "bubble_up_url_count": sum(1 for r in results if r.get("bubble_up_url") is not None),
        # The generic recording envelope — the control group.
        "generic_type": generic.get("type") or "",
        "generic_has_id": generic.get("id") is not None,
        "generic_has_title": generic.get("title") is not None,
        "generic_has_type": generic.get("type") is not None,
        "generic_has_url": generic.get("url") is not None,
        "generic_has_app_url": generic.get("app_url") is not None,
        # The file-attachment branch: searches/_attachment.json.jbuilder writes
        # its own projection, so the absence of a type IS the discriminator.
        "attachment_has_id": attachment.get("id") is not None,
        "attachment_has_title": attachment.get("title") is not None,
        "attachment_has_type": attachment.get("type") is not None,
        "attachment_has_url": attachment.get("url") is not None,
        "attachment_has_app_url": attachment.get("app_url") is not None,
        "attachment_has_content": attachment.get("content") is not None,
        "attachment_has_description": attachment.get("description") is not None,
        "attachment_filename": attachment.get("filename") or "",
        "attachment_content_type": attachment.get("content_type") or "",
        "attachment_byte_size": attachment.get("byte_size") or 0,
        "attachment_previewable": attachment.get("previewable") or False,
        "attachment_width": narrow(attachment.get("width")),
        "attachment_height": narrow(attachment.get("height")),
        # The chat upload line: a bespoke six-key attachments aggregate carrying
        # title/url and NONE of the rich-text id/sgid/preview keys.
        "upload_line_type": upload_line.get("type") or "",
        "upload_boosts_count": upload_line.get("boosts_count") or 0,
        "upload_attachment_filename": upload_attachment.get("filename") or "",
        "upload_attachment_has_title": upload_attachment.get("title") is not None,
        "upload_attachment_has_id": upload_attachment.get("id") is not None,
        "upload_attachment_has_sgid": upload_attachment.get("sgid") is not None,
        # The gauge needle: the same attachments key carrying the OTHER variant
        # — the rich-text one, with id and sgid populated.
        "needle_type": needle.get("type") or "",
        "needle_color": needle.get("color") or "",
        "needle_position": needle.get("position") or 0,
        "needle_comments_count": needle.get("comments_count") or 0,
        "needle_comment_count": needle.get("comment_count") or 0,
        "needle_boosts_count": needle.get("boosts_count") or 0,
        "needle_attachment_has_id": needle_attachment.get("id") is not None,
        "needle_attachment_has_sgid": needle_attachment.get("sgid") is not None,
        "needle_attachment_width": narrow(needle_attachment.get("width")),
        # The kanban list: list-partial keys over the envelope, on_hold nested,
        # and a color emitted unconditionally with a null value.
        "kanban_type": kanban.get("type") or "",
        "kanban_position": kanban.get("position") or 0,
        "kanban_cards_count": kanban.get("cards_count") or 0,
        "kanban_comment_count": kanban.get("comment_count") or 0,
        "kanban_subscriber_count": len(kanban.get("subscribers") or []),
        "kanban_has_color": kanban.get("color") is not None,
        "kanban_has_on_hold": kanban.get("on_hold") is not None,
        "kanban_on_hold_cards_count": (kanban.get("on_hold") or {}).get("cards_count") or 0,
    }


class OperationMapper:
    """Maps conformance operation names to SDK calls."""

    def __init__(self, account_client):
        self._account = account_client

    def __call__(
        self,
        operation: str,
        *,
        path_params: dict,
        query_params: dict,
        body: dict | None,
        path: str = "",
        max_items: int | None = None,
        page: int | None = None,
    ) -> Any:
        match operation:
            case "DownloadURL":
                if not path:
                    raise ValueError("DownloadURL test case requires a non-empty path")
                raw_url = "https://storage.3.basecamp.com" + path
                return self._account.download_url(raw_url)
            case "ListProjects":
                # A pinned page and a max_items cap are independent knobs;
                # the plain no-argument arity stays exercised when the fixture
                # carries neither.
                if max_items or page:
                    return _summarize_projects(self._account.projects.list(max_items=max_items, page=page))
                return _summarize_projects(self._account.projects.list())
            case "Search":
                # Consumed and summarized HERE for the same reason ListProjects
                # is: the summary is what lets a fixture assert on the hits.
                return _summarize_search(self._account.search.search(q=SEARCH_QUERY))
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
            case "CreateTodosetTodo":
                return self._account.todos.create_todoset_todo(
                    bucket_id=path_params["bucketId"], todoset_id=path_params["todosetId"], content=body["content"]
                )
            case "CompleteTodo":
                return self._account.todos.complete(todo_id=path_params["todoId"])
            case "Subscribe":
                return self._account.subscriptions.subscribe(recording_id=path_params["recordingId"])
            case "ListMyBookmarks":
                return self._account.bookmarks.list_my_bookmarks()
            case "ListMyDrafts":
                return self._account.drafts.list_my_drafts()
            case "GetMyNote":
                return self._account.my_notes.get_my_note()
            case "PrioritizeAssignment":
                return self._account.my_assignments.prioritize_assignment(id=body["id"])
            case "DeprioritizeAssignment":
                return self._account.my_assignments.deprioritize_assignment(recording_id=path_params["recordingId"])
            case "ReorderUpNext":
                return self._account.my_assignments.reorder_up_next(
                    source_id=body["source_id"], position=body["position"]
                )
            case "GetCalendar":
                return self._account.calendars.get_calendar(calendar_id=path_params["calendarId"])
            case "UpdateCalendar":
                return self._account.calendars.update_calendar(
                    calendar_id=path_params["calendarId"], calendar=body["calendar"]
                )
            case "UpdateMyNote":
                return self._account.my_notes.update_my_note(note=body["note"])
            case "GetBookmark":
                return self._account.bookmarks.get_bookmark(recording_id=path_params["recordingId"])
            case "CreateBookmark":
                return self._account.bookmarks.create_bookmark(recording_id=path_params["recordingId"])
            case "DeleteBookmark":
                return self._account.bookmarks.delete_bookmark(recording_id=path_params["recordingId"])
            case "ListFolders":
                return self._account.folders.list_folders()
            case "GetFolder":
                return self._account.folders.get_folder(folder_id=path_params["folderId"])
            case "CreateFolder":
                return self._account.folders.create_folder(
                    name=body.get("name"), project_ids=body.get("project_ids")
                )
            case "UpdateFolder":
                return self._account.folders.update_folder(
                    folder_id=path_params["folderId"], name=body["name"]
                )
            case "DeleteFolder":
                return self._account.folders.delete_folder(folder_id=path_params["folderId"])
            case "UpdateTodo":
                return self._account.todos.update(
                    todo_id=path_params["todoId"],
                    **{k: body[k] for k in _TODO_WRITE_FIELDS if k in body},
                )
            case "UpdateTodolist":
                # Synthetic scenario key (not a wire op): the merge-safe
                # composite, GET then PUT of the full {name, description}.
                return self._account.todolists.update(
                    id=path_params["id"],
                    **{k: body[k] for k in _TODOLIST_WRITE_FIELDS if k in body},
                )
            # `url`, `highlighted` and `status` are the three #641 members. The
            # write spelling is `url`; `join_url` is read-only and BC3 drops it
            # from a write body without complaining.
            case "CreateScheduleEntry":
                return self._account.schedules.create_entry(
                    schedule_id=path_params["scheduleId"],
                    **{k: body[k] for k in _SCHEDULE_ENTRY_CREATE_FIELDS if k in body},
                )
            case "ReplaceScheduleEntry":
                # The raw single PUT, no read-before-write. Presence-bearing:
                # only keys the fixture carries are passed, so an unaddressed
                # carve-out (participant_ids, url, highlighted) stays off the
                # wire and BC3 preserves it, while an explicit [] / "" / false
                # survives _compact and clears.
                return self._account.schedules.replace_entry(
                    entry_id=path_params["entryId"],
                    **{k: body[k] for k in _SCHEDULE_ENTRY_WRITE_FIELDS if k in body},
                )
            case "UpdateScheduleEntry":
                # Synthetic scenario key (not a wire op): the merge-safe
                # composite, GET then PUT of the full state, with only the
                # addressed carve-outs merged in.
                return self._account.schedules.update_entry(
                    entry_id=path_params["entryId"],
                    **{k: body[k] for k in _SCHEDULE_ENTRY_WRITE_FIELDS if k in body},
                )
            case "EditScheduleEntry":
                # Synthetic scenario key (not a wire op): drive the edit
                # context manager, assigning each fixture requestBody key
                # onto the same-named attribute. Assignment is what marks a
                # carve-out dirty, so absence stays absence.
                with self._account.schedules.edit_entry(entry_id=path_params["entryId"]) as entry:
                    for key in _SCHEDULE_ENTRY_WRITE_FIELDS:
                        if key in body:
                            setattr(entry, key, body[key])
                return entry.result
            case "UpdateCard":
                # Merge-safe composite: GET then PUT, resending the fetched due_on.
                return self._account.cards.update(
                    card_id=path_params["cardId"],
                    **{k: body[k] for k in _CARD_WRITE_FIELDS if k in body},
                )
            case "UpdateCardVerbatim":
                # Raw single PUT, no read-before-write.
                return self._account.cards.update_verbatim(
                    card_id=path_params["cardId"],
                    **{k: body[k] for k in _CARD_WRITE_FIELDS if k in body},
                )
            case "EditTodo":
                # Synthetic scenario key (not a wire op): drive the edit
                # context manager, assigning each fixture requestBody key
                # onto the same-named attribute.
                with self._account.todos.edit(todo_id=path_params["todoId"]) as t:
                    for key in _TODO_WRITE_FIELDS:
                        if key in body:
                            setattr(t, key, body[key])
                return t.result
            case "ReplaceTodo":
                return self._account.todos.replace(
                    todo_id=path_params["todoId"],
                    **{k: body[k] for k in _TODO_WRITE_FIELDS if k in body},
                )
            case "EditTodolist":
                # Synthetic scenario key (not a wire op): drive the edit
                # context manager, assigning each fixture requestBody key
                # onto the same-named attribute.
                with self._account.todolists.edit(id=path_params["id"]) as tl:
                    for key in _TODOLIST_WRITE_FIELDS:
                        if key in body:
                            setattr(tl, key, body[key])
                return tl.result
            case "ReplaceTodolist":
                # The raw single PUT, no read-before-write. Scenario key only:
                # the wire op is UpdateTodolistOrGroup, renamed to `replace`
                # so the plain `update` can be the merge-safe composite.
                return self._account.todolists.replace(
                    id=path_params["id"],
                    **{k: body[k] for k in _TODOLIST_WRITE_FIELDS if k in body},
                )
            case "UpdateDocument":
                # Synthetic scenario key (not a wire op): the merge-safe
                # composite, GET then PUT of the full {title, content}.
                return self._account.documents.update(
                    document_id=path_params["documentId"],
                    **{k: body[k] for k in _DOCUMENT_WRITE_FIELDS if k in body},
                )
            case "EditDocument":
                # Synthetic scenario key (not a wire op): drive the edit
                # context manager, assigning each fixture requestBody key
                # onto the same-named attribute.
                with self._account.documents.edit(document_id=path_params["documentId"]) as doc:
                    for key in _DOCUMENT_WRITE_FIELDS:
                        if key in body:
                            setattr(doc, key, body[key])
                return doc.result
            case "ReplaceDocument":
                # The raw single PUT, no read-before-write: an omitted field is
                # omitted on the wire and the server clears it.
                return self._account.documents.replace(
                    document_id=path_params["documentId"],
                    **{k: body[k] for k in _DOCUMENT_WRITE_FIELDS if k in body},
                )
            case "GetTimesheetEntry":
                return self._account.timesheets.get(entry_id=path_params["entryId"])
            case "DestroyTimesheetEntry":
                return self._account.timesheets.destroy(entry_id=path_params["entryId"])
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
            # The window is fixed here rather than read from the case: no mock
            # runner consumes query_params, and no assertion type can pin a
            # query string. Both bounds are required, so the call cannot be made
            # without them. The flat summary keeps the responseBody assertions
            # portable to Go and TypeScript, which resolve only top-level keys.
            case "GetUpcomingSchedule":
                return _summarize_upcoming(
                    self._account.reports.upcoming(
                        window_starts_on=UPCOMING_WINDOW_START,
                        window_ends_on=UPCOMING_WINDOW_END,
                    )
                )
            case "GetTool":
                return self._account.tools.get(tool_id=path_params["toolId"])
            case "CreateTool":
                return self._account.tools.create(
                    bucket_id=path_params["bucketId"],
                    tool_type=body["tool_type"],
                    title=body.get("title"),
                )
            case "EnableTool":
                return self._account.tools.enable(tool_id=path_params["toolId"])
            case "UploadsDownload":
                return self._account.uploads.download(upload_id=path_params["uploadId"])
            case "CreateUploadVersion":
                # Presence-bearing, like ReplaceScheduleEntry: a key the fixture
                # omits is never passed, so an unaddressed description stays off
                # the wire while an explicit "" survives _compact (which strips
                # None only) and clears.
                return self._account.uploads.create_version(
                    upload_id=path_params["uploadId"],
                    attachable_sgid=body["attachable_sgid"],
                    **{k: body[k] for k in _UPLOAD_VERSION_WRITE_FIELDS if k in body},
                )
            case "UpdateUpload":
                return self._account.uploads.update(
                    upload_id=path_params["uploadId"],
                    **{k: body[k] for k in _UPLOAD_VERSION_WRITE_FIELDS if k in body},
                )
            case "ListUploadVersions":
                return _summarize_upload_versions(
                    self._account.uploads.list_versions(upload_id=path_params["uploadId"])
                )
            case "GetEverythingMessages":
                return self._account.everything.get_everything_messages()
            case "GetEverythingComments":
                return self._account.everything.get_everything_comments()
            case "GetEverythingCheckins":
                return self._account.everything.get_everything_checkins()
            case "GetEverythingForwards":
                return self._account.everything.get_everything_forwards()
            case "GetEverythingFiles":
                return self._account.everything.get_everything_files()
            case "GetEverythingOverdueTodos":
                return self._account.everything.get_everything_overdue_todos()
            case "GetEverythingOverdueCards":
                return self._account.everything.get_everything_overdue_cards()
            case "GetEverythingOpenTodos":
                return self._account.everything.get_everything_open_todos()
            case "GetEverythingCompletedTodos":
                return self._account.everything.get_everything_completed_todos()
            case "GetEverythingUnassignedTodos":
                return self._account.everything.get_everything_unassigned_todos()
            case "GetEverythingNoDueDateTodos":
                return self._account.everything.get_everything_no_due_date_todos()
            case "GetEverythingOpenCards":
                return self._account.everything.get_everything_open_cards()
            case "GetEverythingCompletedCards":
                return self._account.everything.get_everything_completed_cards()
            case "GetEverythingUnassignedCards":
                return self._account.everything.get_everything_unassigned_cards()
            case "GetEverythingNoDueDateCards":
                return self._account.everything.get_everything_no_due_date_cards()
            case "GetEverythingNotNowCards":
                return self._account.everything.get_everything_not_now_cards()
            case "ListForwards":
                return self._account.forwards.list(inbox_id=path_params["inboxId"])
            # #588: nine flat spellings bc3 only draws bucket-scoped. Each pins
            # the bucketId segment on the wire — the segment whose absence 404'd.
            case "ListChatbots":
                return self._account.campfires.list_chatbots(
                    bucket_id=path_params["bucketId"],
                    campfire_id=path_params["campfireId"],
                )
            case "GetChatbot":
                return self._account.campfires.get_chatbot(
                    bucket_id=path_params["bucketId"],
                    campfire_id=path_params["campfireId"],
                    chatbot_id=path_params["chatbotId"],
                )
            case "CreateChatbot":
                return self._account.campfires.create_chatbot(
                    bucket_id=path_params["bucketId"],
                    campfire_id=path_params["campfireId"],
                    service_name=body["service_name"],
                    command_url=body["command_url"],
                )
            case "UpdateChatbot":
                return self._account.campfires.update_chatbot(
                    bucket_id=path_params["bucketId"],
                    campfire_id=path_params["campfireId"],
                    chatbot_id=path_params["chatbotId"],
                    service_name=body["service_name"],
                    command_url=body["command_url"],
                )
            case "DeleteChatbot":
                return self._account.campfires.delete_chatbot(
                    bucket_id=path_params["bucketId"],
                    campfire_id=path_params["campfireId"],
                    chatbot_id=path_params["chatbotId"],
                )
            case "ListClientApprovals":
                return self._account.client_approvals.list(bucket_id=path_params["bucketId"])
            case "ListClientCorrespondences":
                return self._account.client_correspondences.list(bucket_id=path_params["bucketId"])
            case "ListClientReplies":
                return self._account.client_replies.list(
                    bucket_id=path_params["bucketId"],
                    recording_id=path_params["recordingId"],
                )
            case "GetClientReply":
                return self._account.client_replies.get(
                    bucket_id=path_params["bucketId"],
                    recording_id=path_params["recordingId"],
                    reply_id=path_params["replyId"],
                )
            case "RepositionTodolistGroup":
                return self._account.todolist_groups.reposition(
                    group_id=path_params["groupId"], position=body["position"]
                )
            case "GetTodolistOrGroup":
                # One flat shape for both variants (#544): the same GET answers
                # for a to-do list and for a group inside one. Returned as the
                # case result so the responseBody assertions read what the SDK
                # actually handed back — a read that yielded nothing fails here
                # rather than passing quietly.
                return self._account.todolists.get(id=path_params["id"])
            case "ListTodolistGroups":
                # The group list is an array of that same flat shape. Dispatch
                # convention, stated in the fixture's own description: the FIRST
                # element is the result, so the responseBody paths read element
                # 0. An empty list is a failure rather than a silent pass —
                # every one of those assertions would otherwise read None off a
                # missing element and only the expected values would differ.
                groups = self._account.todolist_groups.list(todolist_id=path_params["todolistId"])
                if not groups:
                    raise ValueError("ListTodolistGroups returned no groups; the first element is the case result")
                return groups[0]
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

        # Defense-in-depth backstop for the operationally-harmful mockResponses
        # shapes (neither mode set → served as a normal HTTP response; or both
        # active). The AUTHORITATIVE oneOf enforcement is
        # `make conformance-fixtures-check` (check-jsonschema against
        # conformance/schema.json), which runs before the runners; it rejects
        # {status, networkError:false} and non-true networkError values that this
        # truthiness backstop intentionally lets through for cross-runner parity.
        for i, r in enumerate(self._test.get("mockResponses", [])):
            has_status = "status" in r
            has_network_error = r.get("networkError") is True
            if has_status == has_network_error:
                return TestResult(
                    self._test["name"],
                    False,
                    f"mockResponses[{i}] must set exactly one of status or networkError",
                )

        with respx.mock:
            self._setup_mock_responses()

            try:
                result = self._mapper(
                    self._test["operation"],
                    path_params=self._test.get("pathParams", {}),
                    query_params=self._test.get("queryParams", {}),
                    body=self._test.get("requestBody"),
                    path=self._test.get("path", ""),
                    max_items=(self._test.get("configOverrides") or {}).get("maxItems"),
                    page=(self._test.get("configOverrides") or {}).get("page"),
                )
                return self._verify_assertions(result=result, error=None)
            except Exception as e:
                return self._verify_assertions(result=None, error=e)

    def _setup_mock_responses(self) -> None:
        responses = self._test.get("mockResponses", [])
        if not responses:
            return

        paginates = self._auto_paginates()
        response_queue = list(responses)
        call_count = [0]

        def side_effect(request: httpx.Request) -> httpx.Response:
            try:
                request_body = json.loads(request.content) if request.content else None
            except ValueError:
                request_body = None
            self._tracker.record_request(
                monotonic_time=time.monotonic(),
                method=str(request.method),
                url=str(request.url),
                headers=dict(request.headers),
                body=request_body,
            )
            idx = call_count[0]
            call_count[0] += 1

            if idx < len(response_queue):
                r = response_queue[idx]
                # Genuine transport failure for this queued entry: raise a
                # connection error the way a real network fault would. The
                # request is already recorded above, so requestCount is correct.
                if r.get("networkError"):
                    raise httpx.ConnectError("simulated network error")
                body = json.dumps(_normalize_body(r["body"], r.get("status"))).encode() if r.get("body") is not None else b""
                headers = {"Content-Type": "application/json"}
                headers.update(r.get("headers", {}))
                return httpx.Response(r["status"], content=body, headers=headers)
            elif paginates:
                return httpx.Response(200, content=b"[]", headers={"Content-Type": "application/json"})
            else:
                return httpx.Response(500, content=b'{"error":"No more mock responses"}', headers={"Content-Type": "application/json"})

        # A single method-agnostic queue on the active client's origin
        # (derived from configOverrides.baseUrl when present): every hop of
        # a scenario — reads before writes, redirects resolved onto the same
        # host — is served in sequence, while a misroute to a different host
        # fails instead of consuming a queued response. Path fidelity is
        # enforced by the implicit invariants in _verify_assertions.
        overrides = self._test.get("configOverrides") or {}
        base_url = overrides.get("baseUrl", "https://3.basecampapi.com")
        parsed = urlparse(base_url)
        # Normalize to httpx's canonical request-URL form (lowercase
        # scheme/host, default port dropped) so a mixed-case or
        # explicit-default-port baseUrl still matches the mock route.
        scheme = parsed.scheme.lower()
        host = (parsed.hostname or "").lower()
        # urlparse's hostname strips IPv6 brackets; restore them, matching
        # how httpx renders the request URL (http://[::1]:3000).
        if ":" in host:
            host = f"[{host}]"
        default_port = {"http": 80, "https": 443}.get(scheme)
        port = f":{parsed.port}" if parsed.port and parsed.port != default_port else ""
        origin = f"{scheme}://{host}{port}"
        respx.route(url__regex=rf"{re.escape(origin)}/.*").mock(side_effect=side_effect)

    def _auto_paginates(self) -> bool:
        return any(
            'rel="next"' in (r.get("headers", {}).get("Link", ""))
            for r in self._test.get("mockResponses", [])
        )

    def _request_at(self, index: int) -> dict | None:
        """Return the captured request at index (0-based; negative counts from end), or None if out of range."""
        requests = self._tracker.requests
        n = len(requests)
        if n == 0:
            return None
        if index < 0:
            index += n
        if index < 0 or index >= n:
            return None
        return requests[index]

    def _request_headers_at(self, index: int) -> dict | None:
        """Return captured headers at index (0-based; negative counts from end), or None if out of range."""
        request = self._request_at(index)
        return request["headers"] if request is not None else None

    def _verify_assertions(self, *, result: Any, error: Exception | None) -> TestResult:
        failures: list[str] = []

        # Implicit invariants: the mock route is origin-wide, so a misroute
        # to a different path on the same origin would silently consume the
        # queue. For DownloadURL, hop 1 must hit the test case path exactly;
        # for every other operation with a path, the first request must
        # contain the pathParams-substituted fixture path.
        if self._test["operation"] == "DownloadURL" and self._tracker.requests:
            expected_path = self._test["path"]
            actual_path = urlparse(self._tracker.requests[0]["url"]).path
            if actual_path != expected_path:
                failures.append(f"DownloadURL hop 1 expected path {expected_path!r}, got {actual_path!r}")
        elif self._test.get("path") and self._tracker.requests:
            expected_path = self._test["path"]
            for key, value in self._test.get("pathParams", {}).items():
                expected_path = expected_path.replace(f"{{{key}}}", str(value))
            actual_path = urlparse(self._tracker.requests[0]["url"]).path
            if expected_path not in actual_path:
                failures.append(f"Expected first request path to contain {expected_path!r}, got {actual_path!r}")

        # Implicit method invariant: the mock queue is method-agnostic, so a
        # wrong-verb request (e.g. a PUT regressing to POST) would consume a
        # queued response silently. When the fixture declares a method and
        # carries no explicit requestMethod assertions, the first request
        # must use the fixture method.
        has_method_assertions = any(a["type"] == "requestMethod" for a in self._test.get("assertions", []))
        if self._test.get("method") and not has_method_assertions and self._tracker.requests:
            expected_method = self._test["method"].upper()
            actual_method = self._tracker.requests[0]["method"].upper()
            if actual_method != expected_method:
                failures.append(f"Expected first request method {expected_method!r}, got {actual_method!r}")

        for assertion in self._test.get("assertions", []):
            match assertion["type"]:
                case "requestCount":
                    # The Python SDK auto-paginates list operations, so a
                    # fixture that counts first-page requests only is
                    # inapplicable — but ONLY its count is. The rest of the
                    # case still runs. See request_count_applies (#573).
                    if not request_count_applies(self._test.get("tags", [])):
                        continue
                    failure = check_request_count(
                        self._tracker.request_count, assertion["expected"]
                    )
                    if failure is not None:
                        failures.append(failure)

                case "delayBetweenRequests":
                    # Not all gaps are retry gaps — the download flow's final
                    # gap is the redirect hop to the signed URL, which is
                    # deliberately un-delayed — so those fixtures name a gap
                    # with an index. See check_delay_gaps for the contract.
                    #
                    # An absent or zero `min` still asserts that the gap EXISTS,
                    # so it must not be truthiness-gated: `if min_delay:` skips
                    # `min: 0` entirely, degrading the assertion to nothing —
                    # the very false-green class this check exists to kill.
                    failure = check_delay_gaps(
                        self._tracker.delays_between_requests,
                        assertion.get("min"),
                        assertion.get("index"),
                        self._tracker.request_count,
                    )
                    if failure:
                        failures.append(failure)

                case "noError":
                    if error:
                        failures.append(f"Expected no error, got: {type(error).__name__}: {error}")

                # The inverse of noError, and deliberately code-agnostic. See
                # error_raised_failure for the contract and for why the branch
                # lives there rather than inline: no committed fixture can reach
                # its failing side, so it is unit-tested instead.
                case "errorRaised":
                    failure = error_raised_failure(error is not None)
                    if failure:
                        failures.append(failure)

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
                        "network": "network",
                    }
                    expected_code = code_map.get(expected_type)
                    if expected_code is None:
                        failures.append(f"Unknown conformance error type {expected_type!r}")
                    else:
                        # Require a canonical code that exists and matches — an
                        # error carrying no .code must fail, not silently pass.
                        actual_code = getattr(error, "code", None)
                        if actual_code is None:
                            failures.append(
                                f"Expected error code {expected_code!r}, but {type(error).__name__} carries no code: {error}"
                            )
                        elif actual_code != expected_code:
                            failures.append(f"Expected error code {expected_code!r}, got {actual_code!r}")

                case "requestPath":
                    expected = assertion["expected"]
                    idx = assertion.get("index", 0)
                    request = self._request_at(idx)
                    if request is None:
                        failures.append(f"Expected request path {expected!r} on request index {idx}, but only {self._tracker.request_count} requests were recorded")
                    else:
                        actual_path = urlparse(request["url"]).path
                        if actual_path != expected:
                            failures.append(f"Expected request path {expected!r} on request index {idx}, got {actual_path!r}")

                case "requestMethod":
                    expected = assertion["expected"]
                    idx = assertion.get("index", 0)
                    request = self._request_at(idx)
                    if request is None:
                        failures.append(f"Expected request method {expected!r} on request index {idx}, but only {self._tracker.request_count} requests were recorded")
                    elif request["method"] != expected:
                        failures.append(f"Expected request method {expected!r} on request index {idx}, got {request['method']!r}")

                case "requestBody":
                    body_path = assertion["path"]
                    expected = assertion["expected"]
                    idx = assertion.get("index", 0)
                    request = self._request_at(idx)
                    if request is None:
                        failures.append(f"Expected request body {body_path} = {expected!r} on request index {idx}, but only {self._tracker.request_count} requests were recorded")
                    elif request["body"] is None:
                        failures.append(f"Expected request body {body_path} = {expected!r} on request index {idx}, but the request had no JSON body")
                    else:
                        actual = _dig_body(request["body"], body_path)
                        if actual is _MISSING:
                            failures.append(f"Expected request body {body_path} = {expected!r} on request index {idx}, but the key is absent")
                        elif actual != expected:
                            failures.append(f"Expected request body {body_path} = {expected!r} on request index {idx}, got {actual!r}")

                case "requestBodyAbsent":
                    body_path = assertion["path"]
                    idx = assertion.get("index", 0)
                    request = self._request_at(idx)
                    if request is None:
                        failures.append(f"Expected request body {body_path} absent on request index {idx}, but only {self._tracker.request_count} requests were recorded")
                    elif request["body"] is not None:
                        actual = _dig_body(request["body"], body_path)
                        if actual is not _MISSING:
                            failures.append(f"Expected request body {body_path} absent on request index {idx}, got {actual!r}")

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
                    idx = assertion.get("index", 0)
                    headers = self._request_headers_at(idx)
                    if headers is None:
                        failures.append(f"Expected header {header_name}={expected!r} on request index {idx}, but only {self._tracker.request_count} requests were recorded")
                    else:
                        actual = headers.get(header_name.lower())
                        if actual != expected:
                            failures.append(f"Expected header {header_name}={expected!r} on request index {idx}, got {actual!r}")

                case "headerPresent":
                    header_name = assertion["path"]
                    idx = assertion.get("index", 0)
                    headers = self._request_headers_at(idx)
                    if headers is None:
                        failures.append(f"Expected header {header_name} on request index {idx}, but only {self._tracker.request_count} requests were recorded")
                    else:
                        actual = headers.get(header_name.lower())
                        if not actual:
                            failures.append(f"Expected header {header_name} on request index {idx}, but it was empty or missing")

                case "headerAbsent":
                    header_name = assertion["path"]
                    idx = assertion.get("index", 0)
                    headers = self._request_headers_at(idx)
                    if headers is None:
                        failures.append(f"Expected header {header_name} absent on request index {idx}, but only {self._tracker.request_count} requests were recorded")
                    else:
                        actual = headers.get(header_name.lower())
                        if actual:
                            failures.append(f"Expected header {header_name} absent on request index {idx}, got {actual!r}")

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
                    elif isinstance(result, dict):
                        # Mirrors the Ruby runner's Hash fallback. A dispatch arm
                        # that reduces a ListResult to a flat summary dict (see
                        # _summarize_projects) has no `.meta` left to read, so it
                        # carries the meta fields as top-level keys under their
                        # JSON names. A lookup, not a truthiness test: an `or`
                        # chain would read a present False (`truncated`) as a
                        # miss.
                        actual = result.get(field_path)
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


def _dig_body(obj: Any, path: str) -> Any:
    """Dig a dot-notation path into a JSON body; _MISSING when any key is absent."""
    for key in path.split("."):
        if isinstance(obj, dict):
            if key not in obj:
                return _MISSING
            obj = obj[key]
        elif isinstance(obj, list):
            try:
                obj = obj[int(key)]
            except (ValueError, IndexError):
                return _MISSING
        else:
            return _MISSING
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
    SKIPS: set[str] = set()
    SKIP_REASONS: dict[str, str] = {}

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
        # maxRetries has to be in this list or a case overriding ONLY the retry
        # cap silently gets the shared default client and passes while testing
        # nothing.
        has_max_retries = "maxRetries" in overrides
        if not has_base_url and not has_max_pages and not has_max_retries:
            return self._mapper

        try:
            config_opts: dict[str, Any] = {"base_url": overrides["baseUrl"] if has_base_url else "https://3.basecampapi.com"}
            if has_max_pages:
                config_opts["max_pages"] = overrides["maxPages"]
            if has_max_retries:
                config_opts["max_retries"] = overrides["maxRetries"]
            config = Config(**config_opts)
            client = Client(config=config, access_token="conformance-test-token")
            account = client.for_account("999")
            return OperationMapper(account)
        except Exception as e:
            return ErrorMapper(e)

    def run(self) -> int:
        # Case census (#602) — see count_non_live_cases. Taken up front, by its
        # own walk, so a fixture tree this runner's glob cannot see is reported
        # before the run rather than inferred from a short count afterwards.
        try:
            expected_cases = count_non_live_cases(self._tests_dir)
        except RuntimeError as e:
            print(f"Error taking fixture census: {e}", file=sys.stderr)
            return 1

        # No early return on an empty glob. The census walks recursively and
        # this glob does not, so "the census found fixtures but this runner
        # globbed none" is exactly the nested-fixture under-count the census
        # exists to reject — and returning success here would step over the
        # comparison that rejects it. Falling through runs zero cases and lets
        # the count check fail, which is the correct answer.
        files = sorted(self._tests_dir.glob("*.json"))
        if not files:
            print(f"No test files found in {self._tests_dir}")

        passed = 0
        failed = 0
        skipped = 0
        # Recorded from the same branch that increments `skipped`, so the
        # manifest cannot claim a different set than the run took.
        excluded: list[tuple[str, str, str]] = []

        for file in files:
            tests = json.loads(file.read_text())
            # Live tests are TS-only (canonical wire-capturer); filter them out
            # before mock dispatch so unresolved ${PROJECT_ID} fixtures and
            # live-only operations don't surface here.
            tests = [t for t in tests if is_mock_mode(t.get("mode"))]
            if not tests:
                continue

            print(f"\n=== {file.name} ===")

            for test_case in tests:
                name = test_case["name"]

                if name in self.SKIPS:
                    skipped += 1
                    reason = self.SKIP_REASONS.get(name, "Python SDK behavior differs")
                    excluded.append((file.name, name, reason))
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
        print(
            f"Results: {passed} passed, {failed} failed, {skipped} skipped "
            f"(fixtures declare {expected_cases} non-live case(s))"
        )

        count_failure = case_count_failure(passed + failed + skipped, expected_cases)
        if count_failure is not None:
            print(f"\nFAIL: {count_failure}", file=sys.stderr)

        # Written even when the run failed: a failing runner still has a
        # truthful exclusion set, and a missing manifest reads to the gate as
        # "this runner did not report", turning one failure into two.
        manifest_failure = None
        try:
            write_execution_manifest("python", expected_cases, passed + failed, excluded)
        except (RuntimeError, OSError) as e:
            manifest_failure = e
            print(f"\nFAIL: could not write execution manifest: {e}", file=sys.stderr)

        return 1 if failed > 0 or count_failure is not None or manifest_failure else 0


if __name__ == "__main__":
    tests_dir = str(Path(__file__).parent.parent.parent / "tests")
    runner = ConformanceRunner(tests_dir)
    sys.exit(runner.run())
