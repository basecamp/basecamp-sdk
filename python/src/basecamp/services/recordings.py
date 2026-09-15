"""Recordings service with ``summarize``, a compact projection of one recording.

``summarize`` resolves the pointer an account event feed row or a webhook
carries -- bucket id, recording id, and the event type or recording type --
through the typed read that type names. It exists for consumers that must decide
something about a recording without paying for its full payload: an agent
connector's admission step, an MCP tool answering "what is this?".

The SDK has no untyped recording read (BC3 has no such route), so the type is
the routing key: ``comment.created`` reads a comment, ``card.created`` reads a
card, and so on -- one typed read per type. Chat lines are the exception,
because their read needs the Campfire id and the pointer does not carry it;
``summarize`` discovers the Campfire first (see
:mod:`basecamp.services._campfire_index`).

This is hand-written composition over the generated services (AGENTS.md;
SPEC.md section 18, Appendix F). It makes no wire request of its own, and it
mints no operation identity: hooks see the constituent reads under their own
per-service names (SPEC.md section 18 rule 3).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, TypedDict

from basecamp.errors import (
    BucketMismatchError,
    CampfireDiscoveryIncompleteError,
    NoRecordingTypeError,
    NotFoundError,
    RecordingUnresolvedError,
    UnknownRecordingTypeError,
    UsageError,
)
from basecamp.generated.services.recordings import AsyncRecordingsService as _GeneratedAsyncRecordingsService
from basecamp.generated.services.recordings import RecordingsService as _GeneratedRecordingsService
from basecamp.mentions import go_trim_space, mentioned_person_ids
from basecamp.services._campfire_index import (
    MAX_CAMPFIRE_CANDIDATES,
    MAX_CAMPFIRE_LISTING,
    CampfireListingOverflow,
    ChatLineSearch,
    SourceRead,
)

__all__ = [
    "MAX_CAMPFIRE_CANDIDATES",
    "MAX_CAMPFIRE_LISTING",
    "AsyncRecordingsService",
    "RecordingSummary",
    "RecordingsService",
    "summarizable_event_types",
    "summarizable_recording_types",
]


class RecordingSummary(TypedDict):
    """The projection ``summarize`` returns.

    Fields a type does not have are empty: a comment has no ``assignees``, a
    vault no ``content``. The key is always present, so a consumer reads a shape
    rather than probing for one.
    """

    id: int
    status: str
    #: The recording type as BC3 spells it ("Comment", "Kanban::Card").
    type: str
    title: str
    app_url: str
    #: The recording this one hangs off -- the commented recording for a
    #: comment, the Campfire for a chat line, the column for a card. Present
    #: and ``None`` where the type has none, never absent: the projection
    #: always writes every key, and a type saying otherwise would push
    #: `.get()` guards onto callers for a case that cannot arise.
    parent: dict[str, Any] | None
    bucket: dict[str, Any] | None
    creator: dict[str, Any] | None
    #: Set for the assignable types (to-dos, cards, card steps).
    assignees: list[dict[str, Any]]
    #: The people ``content`` mentions, per :func:`basecamp.mentions.mentioned_person_ids`.
    mentioned_person_ids: list[int]
    #: The recording's rich text, in full: the comment body, the message body, a
    #: to-do's description, a card's content, the chat line.
    content: str
    updated_at: str | None
    #: The Campfire a chat line was found under -- the reply destination for a
    #: chat trigger. ``None`` for every other type.
    campfire_id: int | None


@dataclass(frozen=True)
class _Read:
    """One routed read: the generated operation that serves a type, and how it projects.

    ``service``/``method``/``param`` name a PUBLIC generated service method --
    no path is built here and no verb chosen (SPEC section 18 rule 1), and the
    call reaches hooks under that service's own identity (rule 3). ``title`` and
    ``content`` are the payload keys to read in order, first non-empty winning,
    because BC3 spells the same two ideas differently per type (a message's
    ``subject``, an upload's ``filename``, a to-do's rich text living in
    ``description`` while its ``content`` is the plain title).
    """

    service: str
    method: str
    param: str
    title: tuple[str, ...] = ("title",)
    content: tuple[str, ...] = ()
    parent: bool = True
    assignees: bool = False


_CHAT_LINE = "chat_line"

#: The subject of an account event feed type -- everything before its final "."
#: -- to a read. This is the feed's catalog (bc3 ``Event::EventType``) minus
#: ``boost``, which names no recording type and is refused explicitly rather
#: than left to fall through as unknown.
_EVENT_SUBJECTS: dict[str, str] = {
    "comment": "Comment",
    "message": "Message",
    "todo": "Todo",
    "card": "Kanban::Card",
    "chat.line": _CHAT_LINE,
}

#: BC3's recording type strings to the read that serves them. This is the
#: routing contract, and it is a DELIBERATE set, not an exhaustive one: the
#: recording types the account event feed's trigger matrix names (comment,
#: message, to-do, card, chat line), plus the content and tool recordings a
#: consumer reasoning about those is likely to hold an id for. Chat lines are
#: matched by prefix (``Chat::Lines::Text``, ``::RichText``, ``::Code``,
#: ``::Upload``, ``::Integration`` all read through the same route); everything
#: else exactly.
#:
#: A type outside this set is :class:`~basecamp.errors.UnknownRecordingTypeError`
#: by design, whether or not the SDK has an id-only read for it --
#: ``Timesheet::Entry`` and ``Gauge::Needle`` do, and are not routed;
#: ``Client::Reply`` and ``Forward::Reply`` cannot be, since their reads need a
#: parent id the pointer does not carry. Widening the set is a product decision,
#: not a gap: add the type here, a routing row in the native test, and a case in
#: ``conformance/tests/recording_summary.json``, the fixture a port implements.
_READS: dict[str, _Read] = {
    "Comment": _Read("comments", "get", "comment_id", content=("content",)),
    "Message": _Read("messages", "get", "message_id", title=("title", "subject"), content=("content",)),
    # A to-do's content is its plain title; the rich text -- where mentions live
    # -- is the description.
    "Todo": _Read("todos", "get", "todo_id", title=("title", "content"), content=("description",), assignees=True),
    "Kanban::Card": _Read("cards", "get", "card_id", content=("content", "description"), assignees=True),
    "Document": _Read("documents", "get", "document_id", content=("content",)),
    "Upload": _Read("uploads", "get", "upload_id", title=("title", "filename"), content=("description",)),
    "Schedule::Entry": _Read(
        "schedules", "get_entry", "entry_id", title=("title", "summary"), content=("description",)
    ),
    "Question": _Read("checkins", "get_question", "question_id"),
    "Question::Answer": _Read("checkins", "get_answer", "answer_id", content=("content",)),
    "Todolist": _Read("todolists", "get", "id", title=("title", "name"), content=("description",)),
    "Vault": _Read("vaults", "get", "vault_id"),
    "Inbox::Forward": _Read("forwards", "get", "forward_id", title=("title", "subject"), content=("content",)),
    "Client::Approval": _Read(
        "client_approvals", "get", "approval_id", title=("title", "subject"), content=("content",)
    ),
    "Client::Correspondence": _Read(
        "client_correspondences", "get", "correspondence_id", title=("title", "subject"), content=("content",)
    ),
    "GoogleDocument": _Read("google_documents", "get_google_document", "google_document_id", content=("description",)),
    "CloudFile": _Read("cloud_files", "get_cloud_file", "cloud_file_id", content=("description",)),
    "Kanban::Step": _Read("card_steps", "get", "step_id", assignees=True),
    "Questionnaire": _Read("checkins", "get_questionnaire", "questionnaire_id", title=("title", "name"), parent=False),
    "Schedule": _Read("schedules", "get", "schedule_id", parent=False),
    "Todoset": _Read("todosets", "get", "todoset_id", title=("title", "name"), parent=False),
    "Message::Board": _Read("message_boards", "get", "board_id", parent=False),
    "Kanban::Board": _Read("card_tables", "get", "card_table_id", parent=False),
    "Kanban::Column": _Read("card_columns", "get", "column_id", content=("description",)),
    "Inbox": _Read("forwards", "get_inbox", "inbox_id", parent=False),
    "Chat::Transcript": _Read("campfires", "get", "campfire_id", parent=False),
    # The line read also needs the Campfire the pointer does not carry, so the
    # resolver calls it directly; the row is here for its projection and so the
    # routing table names every read.
    _CHAT_LINE: _Read("campfires", "get_line", "line_id", content=("content",)),
}

_CHAT_LINE_TYPE_PREFIX = "Chat::Lines::"

#: The chat line subtypes that carry rich text -- the two that declare
#: ``rich_text_attribute :content`` in BC3, and so the only two whose content can
#: hold a mention. A Text line's content is HTML-escaped on the way out
#: (``content_helper.rb``, ``format_chat_line_with``), a Code line's is served
#: verbatim -- a snippet that happens to contain a ``bc-attachment`` tag -- and an
#: Upload line has no content.
_RICH_TEXT_CHAT_LINES = frozenset({"Chat::Lines::RichText", "Chat::Lines::Integration"})


def summarizable_recording_types() -> list[str]:
    """The recording types ``summarize`` routes by recording type, sorted.

    The ``Chat::Lines`` subtypes are represented by their shared prefix. The set
    is deliberate rather than exhaustive -- see :data:`_READS` -- and any other
    type raises :class:`~basecamp.errors.UnknownRecordingTypeError` by design.
    """
    return sorted([t for t in _READS if t != _CHAT_LINE] + [f"{_CHAT_LINE_TYPE_PREFIX}*"])


def summarizable_event_types() -> list[str]:
    """The account event feed subjects ``summarize`` routes by event type, sorted.

    An event type is ``<subject>.<action>``, and any action on a listed subject
    routes to that subject's read. ``boost`` is absent on purpose: it raises
    :class:`~basecamp.errors.NoRecordingTypeError`.
    """
    return sorted(f"{subject}.*" for subject in _EVENT_SUBJECTS)


def _route(event_type: str | None, recording_type: str | None) -> str:
    """Pick the read for a pointer. The recording type wins when set, being the more exact."""
    # Routing reads the trimmed value; a refusal REPORTS the value as given,
    # the way Go's RecordingRoutingError names the field it was handed. The key
    # is the recording type whenever the caller supplied one at all, even a
    # blank that routing then falls through.
    given = recording_type or ""
    key = given or (event_type or "")

    exact = go_trim_space(given)
    if exact:
        if exact.startswith(_CHAT_LINE_TYPE_PREFIX):
            return _CHAT_LINE
        if exact in _READS and exact != _CHAT_LINE:
            return exact
        raise UnknownRecordingTypeError(key)

    subject_and_action = go_trim_space(event_type or "")
    if not subject_and_action:
        raise UnknownRecordingTypeError(key)
    # A feed type is "<subject>.<action>"; the subject names the recording type.
    # A string with no action is not a feed type and is not routed.
    separator = subject_and_action.rfind(".")
    if separator <= 0 or separator == len(subject_and_action) - 1:
        raise UnknownRecordingTypeError(key)
    subject = subject_and_action[:separator]
    if subject == "boost":
        raise NoRecordingTypeError(key)
    if subject in _EVENT_SUBJECTS:
        return _EVENT_SUBJECTS[subject]
    raise UnknownRecordingTypeError(key)


def _text(record: dict[str, Any], keys: tuple[str, ...]) -> str:
    """The first non-empty string among ``keys``; ``""`` when there is none."""
    for key in keys:
        value = record.get(key)
        if isinstance(value, str) and value:
            return value
    return ""


def _project(record: dict[str, Any], read: _Read, *, campfire_id: int | None = None) -> RecordingSummary:
    content = _text(record, read.content)
    return RecordingSummary(
        # 0, not None, where the payload carries no id: `id` is declared `int`
        # and Go cannot produce anything else.
        id=record.get("id") or 0,
        status=_text(record, ("status",)),
        type=_text(record, ("type",)),
        title=_text(record, read.title),
        app_url=_text(record, ("app_url",)),
        parent=record.get("parent") if read.parent else None,
        bucket=record.get("bucket"),
        creator=record.get("creator"),
        assignees=list(record.get("assignees") or ()) if read.assignees else [],
        mentioned_person_ids=mentioned_person_ids(content),
        content=content,
        updated_at=record.get("updated_at"),
        campfire_id=campfire_id,
    )


def _project_chat_line(line: dict[str, Any], campfire_id: int) -> RecordingSummary:
    summary = _project(line, _READS[_CHAT_LINE], campfire_id=campfire_id)
    if line.get("type") not in _RICH_TEXT_CHAT_LINES:
        # A plain-text or code line's content is text BC3 never read as markup,
        # so a literal "<bc-attachment>" in it mentions nobody.
        summary["mentioned_person_ids"] = []
    return summary


def _check_pointer(bucket_id: int, recording_id: int) -> None:
    """Refuse a pointer that names no recording, before anything is routed.

    The type is checked as well as the value: `bool` is an `int` in Python, so
    a bare range test would read ``True`` as recording 1, and a string id would
    raise a bare ``TypeError`` from the comparison instead of the SDK's own
    usage error.
    """
    for value in (bucket_id, recording_id):
        if not isinstance(value, int) or isinstance(value, bool) or value <= 0:
            raise UsageError("bucket id and recording id are required")


def _check_bucket(summary: RecordingSummary, bucket_id: int, recording_id: int) -> None:
    """Refuse a read whose bucket disagrees with the pointer's.

    The pointer's bucket scopes the Campfire discovery for chat lines, and this
    check is what keeps a pointer from one project from ever resolving to a
    recording in another.
    """
    bucket = summary.get("bucket") or {}
    found = bucket.get("id")
    if found and found != bucket_id:
        raise BucketMismatchError(bucket_id=found, recording_id=recording_id, requested_bucket_id=bucket_id)


def _too_many_candidates() -> str:
    return f"more than {MAX_CAMPFIRE_CANDIDATES} visible campfires in the bucket"


def _stale_candidates(tried: list[int], dock: SourceRead, listed: SourceRead) -> list[int]:
    """Tried candidates the refreshed sources no longer list."""
    current = set(dock.ids) | set(listed.ids)
    return [campfire_id for campfire_id in tried if campfire_id not in current]


class RecordingsService(_GeneratedRecordingsService):
    """Sync recordings service with the ``summarize`` composite."""

    def summarize(
        self,
        *,
        bucket_id: int,
        recording_id: int,
        event_type: str | None = None,
        recording_type: str | None = None,
    ) -> RecordingSummary:
        """Resolve a recording pointer into a :class:`RecordingSummary`.

        ``recording_type`` is the recording's own type as BC3 spells it --
        ``"Comment"``, ``"Kanban::Card"``, ``"Chat::Lines::Text"``. It takes
        precedence over ``event_type``, the account event feed type that named
        the recording -- ``"comment.created"``, ``"card.assignment_changed"`` --
        whose segment before the action names the recording type.

        Raises :class:`~basecamp.errors.NoRecordingTypeError` or
        :class:`~basecamp.errors.UnknownRecordingTypeError` before any request
        when the pointer cannot be routed; the read's own error otherwise -- a
        404 is :class:`~basecamp.errors.NotFoundError`, as from the typed read
        itself. For chat lines,
        :class:`~basecamp.errors.RecordingUnresolvedError` when every visible
        Campfire answered 404, which is distinct from a read that failed (any
        non-404 from a candidate is raised as that error, and the search stops
        there) and from
        :class:`~basecamp.errors.CampfireDiscoveryIncompleteError` (candidates
        were left unsearched). :class:`~basecamp.errors.BucketMismatchError`
        when the read returned a recording from another bucket.
        """
        _check_pointer(bucket_id, recording_id)
        kind = _route(event_type, recording_type)

        if kind == _CHAT_LINE:
            line, campfire_id = self._resolve_chat_line(bucket_id, recording_id)
            summary = _project_chat_line(line, campfire_id)
        else:
            read = _READS[kind]
            service = getattr(self._client, read.service)
            record = getattr(service, read.method)(**{read.param: recording_id})
            summary = _project(record, read)

        _check_bucket(summary, bucket_id, recording_id)
        return summary

    def _try_candidates(
        self, search: ChatLineSearch, candidates: list[int], line_id: int
    ) -> tuple[dict[str, Any], int] | None:
        """Read the line under each untried candidate; the hit, or ``None`` on a miss.

        Any answer but 404 is raised as itself: a permission failure never
        masquerades as "not here", and the next candidate is not tried.
        """
        for campfire_id in search.candidates(candidates):
            try:
                line = self._client.campfires.get_line(campfire_id=campfire_id, line_id=line_id)
            except NotFoundError:
                search.record_miss(campfire_id)
                continue
            return line, campfire_id
        return None

    def _resolve_chat_line(self, bucket_id: int, line_id: int) -> tuple[dict[str, Any], int]:
        account = self._client
        index = account.campfire_index
        search = ChatLineSearch()

        def incomplete(reason: str) -> CampfireDiscoveryIncompleteError:
            return CampfireDiscoveryIncompleteError(bucket_id=bucket_id, recording_id=line_id, reason=reason)

        # Pass 1: what the sources already hold -- the dock (read if it must be),
        # then the listing only if it is cached. A listing fetch is the
        # expensive, slow request, and it is not made until the dock -- including
        # its refresh -- has had its say, so a listing that is down or over its
        # cap never stands between a project's line and the one project read
        # that finds it.
        dock = index.dock_campfires(account, bucket_id, refresh=False)
        hit = self._try_candidates(search, dock.ids, line_id)
        if hit is not None:
            return hit
        cached_listing = index.cached_listed_campfires(account.account_id, bucket_id)
        listed = cached_listing if cached_listing is not None else SourceRead([], 0.0, False)
        if cached_listing is not None:
            hit = self._try_candidates(search, listed.ids, line_id)
            if hit is not None:
                return hit

        # Pass 2: re-read the dock if it was served from cache (the floor may
        # decline), then fetch or refresh the listing. Whatever comes back is the
        # current snapshot of that source, whoever loaded it -- another caller
        # may have populated or refreshed it in the meantime -- so it always
        # replaces the pass-1 one; "refreshed" is whether a source the
        # conclusion had consulted is now newer than when it was consulted.
        #
        # Not when the budget is already spent: a re-read could return no
        # candidate this call may try, so it would cost a request that cannot
        # help -- and a failure on it would replace the deterministic
        # "incomplete" verdict with a transient error a consumer retries forever.
        refreshed = False
        if search.skipped:
            raise incomplete(_too_many_candidates())
        if dock.cached:
            again = index.dock_campfires(account, bucket_id, refresh=True)
            if again.fetched > dock.fetched or not again.cached:
                refreshed = True
            dock = again
            hit = self._try_candidates(search, dock.ids, line_id)
            if hit is not None:
                return hit
        if search.skipped:
            raise incomplete(_too_many_candidates())

        try:
            again = index.listed_campfires(account, bucket_id, refresh=cached_listing is not None)
        except CampfireListingOverflow as overflow:
            raise incomplete(str(overflow)) from overflow
        if cached_listing is not None and (again.fetched > listed.fetched or not again.cached):
            refreshed = True
        listed = again
        hit = self._try_candidates(search, listed.ids, line_id)
        if hit is not None:
            return hit
        if search.skipped:
            raise incomplete(_too_many_candidates())

        raise RecordingUnresolvedError(
            bucket_id=bucket_id,
            recording_id=line_id,
            campfire_ids=search.tried,
            refreshed=refreshed,
            stale_campfire_ids=_stale_candidates(search.tried, dock, listed) if refreshed else [],
        )


class AsyncRecordingsService(_GeneratedAsyncRecordingsService):
    """Async recordings service with the ``summarize`` composite."""

    async def summarize(
        self,
        *,
        bucket_id: int,
        recording_id: int,
        event_type: str | None = None,
        recording_type: str | None = None,
    ) -> RecordingSummary:
        _check_pointer(bucket_id, recording_id)
        kind = _route(event_type, recording_type)

        if kind == _CHAT_LINE:
            line, campfire_id = await self._resolve_chat_line(bucket_id, recording_id)
            summary = _project_chat_line(line, campfire_id)
        else:
            read = _READS[kind]
            service = getattr(self._client, read.service)
            record = await getattr(service, read.method)(**{read.param: recording_id})
            summary = _project(record, read)

        _check_bucket(summary, bucket_id, recording_id)
        return summary

    summarize.__doc__ = RecordingsService.summarize.__doc__

    async def _try_candidates(
        self, search: ChatLineSearch, candidates: list[int], line_id: int
    ) -> tuple[dict[str, Any], int] | None:
        for campfire_id in search.candidates(candidates):
            try:
                line = await self._client.campfires.get_line(campfire_id=campfire_id, line_id=line_id)
            except NotFoundError:
                search.record_miss(campfire_id)
                continue
            return line, campfire_id
        return None

    async def _resolve_chat_line(self, bucket_id: int, line_id: int) -> tuple[dict[str, Any], int]:
        # The sync twin above carries the contract this mirrors step for step.
        account = self._client
        index = account.campfire_index
        search = ChatLineSearch()

        def incomplete(reason: str) -> CampfireDiscoveryIncompleteError:
            return CampfireDiscoveryIncompleteError(bucket_id=bucket_id, recording_id=line_id, reason=reason)

        dock = await index.dock_campfires(account, bucket_id, refresh=False)
        hit = await self._try_candidates(search, dock.ids, line_id)
        if hit is not None:
            return hit
        cached_listing = await index.cached_listed_campfires(account.account_id, bucket_id)
        listed = cached_listing if cached_listing is not None else SourceRead([], 0.0, False)
        if cached_listing is not None:
            hit = await self._try_candidates(search, listed.ids, line_id)
            if hit is not None:
                return hit

        refreshed = False
        if search.skipped:
            raise incomplete(_too_many_candidates())
        if dock.cached:
            again = await index.dock_campfires(account, bucket_id, refresh=True)
            if again.fetched > dock.fetched or not again.cached:
                refreshed = True
            dock = again
            hit = await self._try_candidates(search, dock.ids, line_id)
            if hit is not None:
                return hit
        if search.skipped:
            raise incomplete(_too_many_candidates())

        try:
            again = await index.listed_campfires(account, bucket_id, refresh=cached_listing is not None)
        except CampfireListingOverflow as overflow:
            raise incomplete(str(overflow)) from overflow
        if cached_listing is not None and (again.fetched > listed.fetched or not again.cached):
            refreshed = True
        listed = again
        hit = await self._try_candidates(search, listed.ids, line_id)
        if hit is not None:
            return hit
        if search.skipped:
            raise incomplete(_too_many_candidates())

        raise RecordingUnresolvedError(
            bucket_id=bucket_id,
            recording_id=line_id,
            campfire_ids=search.tried,
            refreshed=refreshed,
            stale_campfire_ids=_stale_candidates(search.tried, dock, listed) if refreshed else [],
        )
