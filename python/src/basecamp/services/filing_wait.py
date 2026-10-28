"""Move or copy a recording and wait for Basecamp to finish it.

SPEC section 18 and Appendix F "Filing Waits", ported from
``go/pkg/basecamp/filing_wait.go``. The start is sent once and never resent,
because each copy request makes another copy; GetFiling is then polled with
backoff until the filing completes or fails, or the wait ends.
"""

from __future__ import annotations

import asyncio
import time
from collections.abc import Awaitable, Callable
from typing import Any

from basecamp.errors import BasecampError, ErrorCode

#: The kind of container a move or copy files a recording into. Every value but
#: ``"todolist"`` is a tool's dock name: the destination project's container is
#: its enabled dock item of that name. A to-do list is not on the dock; list the
#: destination project's to-do lists under its todoset.
_FILING_CONTAINERS: dict[str, str] = {
    "Kanban::Card": "kanban_board",
    "Kanban::Column": "kanban_board",
    "Message": "message_board",
    "Document": "vault",
    "Upload": "vault",
    "CloudFile": "vault",
    "GoogleDocument": "vault",
    "Vault": "vault",
    "Schedule::Entry": "schedule",
    "Question": "questionnaire",
    "Inbox::Forward": "inbox",
    "Todolist": "todoset",
    "Todo": "todolist",
}

_DEFAULT_WAIT = 60.0
_FIRST_POLL = 0.25
_MAX_POLL = 2.0


def filing_container_for(recording_type: str, parent_type: str) -> str | None:
    """The kind of container a move or copy files a recording into.

    Takes the recording's type and its parent's type as the recording's own read
    reports them (``""`` when it has no parent). A card lands in Triage, or in
    Done if it is completed. ``None`` for everything else: a recording that
    moves only with its parent, such as a comment or the columns every card
    table has, and a dock tool, such as a docked Vault, which is filed by
    ``destination_project_id``.
    """
    if recording_type == "Todolist" and parent_type == "Todolist":
        return "todolist"
    if recording_type == "Vault" and parent_type != "Vault":
        return None
    return _FILING_CONTAINERS.get(recording_type)


def filing_container_takes_folders(container: str) -> bool:
    """Whether a destination of this kind can take ``parent_id``, a folder inside it."""
    return container == "vault"


class FilingError(Exception):
    """A move or copy that did not complete.

    ``kind`` says how far it got. Only :class:`FilingRefusedError` is safe to try
    again. Like Go's, these are not ``BasecampError``: the failure that caused
    one is its ``__cause__``, so a retryable error inside one that may have
    filed cannot read as permission to send the start again.
    """

    kind = ""


class FilingRefusedError(FilingError):
    """Basecamp refused to start the move or copy (a 4xx): nothing was filed."""

    kind = "filing_refused"


class FilingMayHaveStartedError(FilingError):
    """The start failed without Basecamp refusing it, such as a 5xx or a dropped
    connection. It may have been filed, so look before trying again."""

    kind = "filing_may_have_started"


class FilingFailedError(FilingError):
    """Basecamp started the move or copy and reported it failed."""

    kind = "filing_failed"

    def __init__(self, filing: dict[str, Any]):
        super().__init__(f"filing {filing['id']} failed")
        self.filing = filing


class FilingUnsettledError(FilingError):
    """Basecamp started the move or copy and had not finished when the wait ended.

    ``filing`` is its last known state; poll ``get_filing(filing_id=filing["id"])``
    rather than starting it again.
    """

    kind = "filing_unsettled"

    def __init__(self, filing: dict[str, Any]):
        super().__init__(f"filing {filing['id']} is still {filing['status']}")
        self.filing = filing


def _start_error(err: Exception) -> FilingError:
    if isinstance(err, BasecampError):
        status = err.http_status or 0
        if 400 <= status < 500 or err.code in (ErrorCode.USAGE, ErrorCode.RATE_LIMIT):
            return FilingRefusedError(f"filing refused: {err}")
    return FilingMayHaveStartedError(f"filing may have started: {err}")


def _settled(filing: dict[str, Any]) -> dict[str, Any] | None:
    if filing.get("status") == "completed" and filing.get("destination_recording"):
        return filing
    if filing.get("status") == "failed":
        raise FilingFailedError(filing)
    return None


def file_and_wait(
    start: Callable[[], dict[str, Any]],
    get_filing: Callable[[int], dict[str, Any]],
    timeout: float | None,
) -> dict[str, Any]:
    try:
        filing = start()
    except Exception as err:
        raise _start_error(err) from err
    deadline = time.monotonic() + (timeout if timeout and timeout > 0 else _DEFAULT_WAIT)
    delay = _FIRST_POLL
    while True:
        if _settled(filing):
            return filing
        remaining = deadline - time.monotonic()
        if remaining <= delay:
            time.sleep(max(remaining, 0))
            raise FilingUnsettledError(filing)
        time.sleep(delay)
        try:
            filing = get_filing(filing["id"])
        except Exception as err:
            raise FilingUnsettledError(filing) from err
        delay = min(2 * delay, _MAX_POLL)


async def async_file_and_wait(
    start: Callable[[], Awaitable[dict[str, Any]]],
    get_filing: Callable[[int], Awaitable[dict[str, Any]]],
    timeout: float | None,
) -> dict[str, Any]:
    try:
        filing = await start()
    except Exception as err:
        raise _start_error(err) from err
    deadline = time.monotonic() + (timeout if timeout and timeout > 0 else _DEFAULT_WAIT)
    delay = _FIRST_POLL
    while True:
        if _settled(filing):
            return filing
        remaining = deadline - time.monotonic()
        if remaining <= delay:
            await asyncio.sleep(max(remaining, 0))
            raise FilingUnsettledError(filing)
        await asyncio.sleep(delay)
        try:
            filing = await get_filing(filing["id"])
        except Exception as err:
            raise FilingUnsettledError(filing) from err
        delay = min(2 * delay, _MAX_POLL)
