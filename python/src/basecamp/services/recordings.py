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
    ApiError,
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
    _decoded_array,
    _decoded_flexible_int64,
    _decoded_int64,
    _decoded_optional_object,
    _decoded_optional_string,
    _decoded_string,
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
    """The first non-empty string among ``keys``; ``""`` when there is none.

    Each candidate is DECODED rather than tested: these are `string` fields in
    Go, so a number or an object there fails the whole read. Skipping to the
    next key instead would quietly answer with a different field's value.
    """
    # EVERY key is decoded before any is chosen. Returning on the first
    # non-empty one would leave the later keys unread, so `{"title": "ok",
    # "subject": 7}` answered "ok" where Go -- which has decoded the whole
    # struct before `firstNonEmpty` runs -- fails the read. That gap reached
    # the eleven routed types whose title or content tuple has two keys.
    decoded = [_decoded_string(record.get(key), f"the recording {key}") for key in keys]
    for value in decoded:
        if value:
            return value
    return ""


def _body(record: Any, what: str) -> dict[str, Any]:
    """A read's body as an object, applying Go's asymmetry between `null` and junk.

    `json.Unmarshal` of `null` into a struct is a NO-OP at any depth: no error,
    the zero value left in place -- so a null body is a recording with every
    field empty, not a failed read, and Go goes on to project it. Every OTHER
    non-object (an array, a string, a number, a bool) IS a decode error there
    and the read never returns.

    Both halves matter. Refusing `null` would reject a body the contract
    accepts; accepting the rest would build a summary out of something that is
    not a recording. Nothing typed sits between a dict-based SDK and the wire,
    so this is where the two are told apart -- and a bare `AttributeError` out
    of `"oops".get` is outside the SDK's error taxonomy either way.
    """
    if record is None:
        return {}
    if not isinstance(record, dict):
        raise ApiError(f"{what} was not an object: {type(record).__name__}")
    return record


def _decoded_person(value: Any, what: str) -> dict[str, Any] | None:
    """A `Person`, validated on the field the summary's consumers key on.

    Only `id` is decoded, and deliberately: `Person` carries a dozen fields and
    guessing at the rest risks refusing a body Go accepts, which is the worse
    direction. `id` is `FlexibleInt64` -- NOT the `int64` its neighbours use.
    """
    person = _decoded_optional_object(value, what)
    if person is not None and "id" in person:
        _decoded_flexible_int64(person["id"], f"{what} id")
    return person


def _decoded_parent(value: Any, what: str) -> dict[str, Any] | None:
    """A `RecordingParent`, whose whole shape is small enough to decode."""
    parent = _decoded_optional_object(value, what)
    if parent is None:
        return None
    # `Id` is a plain `int64` here, where a Person's is flexible: "7" resolves
    # for a creator and FAILS THE READ for a parent. One rule for both would be
    # wrong in one direction whichever was chosen -- measured, not assumed.
    _decoded_int64(parent.get("id"), f"{what} id")
    for field in ("title", "type", "url", "app_url"):
        _decoded_string(parent.get(field), f"{what} {field}")
    _decoded_optional_object(parent.get("bucket"), f"{what} bucket")
    return parent


def _decoded_bucket(value: Any, what: str) -> dict[str, Any] | None:
    """A `TodoBucket`: id, name, type."""
    bucket = _decoded_optional_object(value, what)
    if bucket is None:
        return None
    _decoded_int64(bucket.get("id"), f"{what} id")
    for field in ("name", "type"):
        _decoded_string(bucket.get(field), f"{what} {field}")
    return bucket


def _project(record: Any, read: _Read, *, campfire_id: int | None = None) -> RecordingSummary:
    record = _body(record, "the recording")
    content = _text(record, read.content)
    return RecordingSummary(
        # 0, not None, where the payload carries no id: `id` is declared `int`
        # and Go cannot produce anything else -- nor can it produce a bool or a
        # string, which its typed decode refuses and a dict does not. No VALUE
        # test: Go hands `cf.ID` to the summary untouched, so a negative id is
        # reported as itself rather than flattened to 0.
        id=_decoded_int64(record.get("id"), "the recording id"),
        status=_text(record, ("status",)),
        type=_text(record, ("type",)),
        title=_text(record, read.title),
        app_url=_text(record, ("app_url",)),
        # `*Parent`, `*Bucket`, `*Person`: null stays None, an object stays an
        # object, anything else fails the read as it does one level up. The
        # guard was top-level only, so `{"parent": "oops"}` sailed through.
        parent=_decoded_parent(record.get("parent"), "the recording parent") if read.parent else None,
        bucket=_decoded_bucket(record.get("bucket"), "the recording bucket"),
        creator=_decoded_person(record.get("creator"), "the recording creator"),
        # `[]Person`. `list("oops")` INVENTED four assignees out of a string
        # and `list(7)` raised a bare TypeError; a null element is Go's zero
        # Person, which is an empty object rather than None.
        assignees=(
            [
                _decoded_person(person, "an assignee") or {}
                for person in _decoded_array(record.get("assignees"), "the recording assignees")
            ]
            if read.assignees
            else []
        ),
        mentioned_person_ids=mentioned_person_ids(content),
        content=content,
        # `time.Time` in Go, so a number or an object fails the read. The port
        # keeps the API's own string rather than parsing an instant (Appendix
        # F), so the TYPE is checked and the VALUE is not: a string Go could
        # not parse as RFC 3339 still rides through here.
        updated_at=_decoded_optional_string(record.get("updated_at"), "the recording updated_at"),
        campfire_id=campfire_id,
    )


def _project_chat_line(line: Any, campfire_id: int) -> RecordingSummary:
    line = _body(line, "the chat line")
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
    # `_project` already decoded this as a `*Bucket`, so it is a dict or None
    # and an unreadable one failed the read rather than arriving here. Go's
    # check begins `summary.Bucket != nil`: null or absent skips it.
    bucket = summary.get("bucket")
    if bucket is None:
        return
    # `2085958499.0 == 2085958499` is True in Python, so an untyped id would
    # WAVE THROUGH a recording from another project -- the one thing this check
    # exists to stop. It is a decode failure, not a mismatch: reporting it as
    # `BucketMismatchError(bucket_id=0)` named a bucket that does not exist and
    # put a str or a float into a field declared `int`.
    found = _decoded_int64(bucket.get("id"), "the recording bucket id")
    # Go: `summary.Bucket.ID != 0 && summary.Bucket.ID != ref.BucketID`.
    if found != 0 and found != bucket_id:
        raise BucketMismatchError(bucket_id=found, recording_id=recording_id, requested_bucket_id=bucket_id)


def _too_many_candidates() -> str:
    return f"more than {MAX_CAMPFIRE_CANDIDATES} visible campfires in the bucket"


def _budget_spent_before_listing() -> str:
    return f"the candidate budget of {MAX_CAMPFIRE_CANDIDATES} was spent before the account listing was consulted"


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
        # No budget left means no re-read: a source already consulted cannot
        # hand this call a candidate it may try, so its refresh is skipped and
        # the conclusion stands on what was seen (refreshed stays False). A
        # source never consulted is different -- candidates may exist there
        # unsearched -- so running out of budget before it makes the verdict
        # incomplete rather than unresolved.
        if search.budget > 0 and dock.cached:
            again = index.dock_campfires(account, bucket_id, refresh=True)
            if again.fetched > dock.fetched or not again.cached:
                refreshed = True
            dock = again
            hit = self._try_candidates(search, dock.ids, line_id)
            if hit is not None:
                return hit

        if search.budget <= 0:
            if cached_listing is None:
                raise incomplete(_budget_spent_before_listing())
        else:
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
        # No budget left means no re-read: a source already consulted cannot
        # hand this call a candidate it may try, so its refresh is skipped and
        # the conclusion stands on what was seen (refreshed stays False). A
        # source never consulted is different -- candidates may exist there
        # unsearched -- so running out of budget before it makes the verdict
        # incomplete rather than unresolved.
        if search.budget > 0 and dock.cached:
            again = await index.dock_campfires(account, bucket_id, refresh=True)
            if again.fetched > dock.fetched or not again.cached:
                refreshed = True
            dock = again
            hit = await self._try_candidates(search, dock.ids, line_id)
            if hit is not None:
                return hit

        if search.budget <= 0:
            if cached_listing is None:
                raise incomplete(_budget_spent_before_listing())
        else:
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
