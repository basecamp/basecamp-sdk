"""Campfire discovery for chat lines, and the TTL caches that hold it.

A ``chat.line.created`` row carries the line's id and bucket, not its Campfire,
and the line read is ``/chats/{campfire_id}/lines/{line_id}``. Candidates come
from two sources, tried in order and each cached :data:`CAMPFIRE_INDEX_TTL`:

1. The bucket's project dock, whose ``chat`` tool is the project's Campfire: one
   project read per bucket, and the answer for every line posted in a project.
   Cached per bucket.
2. The account-wide Campfire listing (BC3 has no per-bucket one), filtered to
   the bucket, for buckets that are not projects or whose dock did not hold the
   line. Cached per account, so a burst of lines costs one listing.

The search tries the line under each candidate until one answers, within one
total budget of :data:`MAX_CAMPFIRE_CANDIDATES` per call.

Two failure shapes are kept apart on purpose. A candidate that answers anything
but 404 -- 401, 403, 5xx, a transport failure -- stops the search and is raised
as that error: the read failed, and trying the next Campfire would only hide it.
A 404 means "not here", so the search moves on. Only when every candidate said
"not here" is the line unresolved
(:class:`~basecamp.errors.RecordingUnresolvedError`) -- and before concluding
that, the cached sources are refreshed (subject to a floor) so a Campfire
created after the cache filled is tried too. Discovery that could not be
completed -- a listing cut off at its cap, a bucket with more candidates than the
budget -- is :class:`~basecamp.errors.CampfireDiscoveryIncompleteError`, never
"unresolved": nothing unsearched is ever reported absent.

What HTTP cannot tell apart: BC3 answers 404 both for a line that is not in a
Campfire and for a Campfire the caller may no longer see. "Unresolved" therefore
means "under no Campfire the caller can currently see", and the error reports
the cached candidates that the refreshed sources no longer list
(``stale_campfire_ids``) so a consumer can see when visibility, not existence,
is what changed.

The index lives on the ``Client`` (shared by every account client the ``Client``
hands out); a ``Client`` is bound to one credential, so entries are never shared
across authorization contexts, and every key carries the account id.

**What bounds a load.** The caches load under single flight, and a waiter waits
on the load without a deadline of its own -- so what stops a load that never
settles from parking every later caller for that key is the HTTP client
underneath it. Every request a loader makes goes through the same ``Config``
timeout (30 s by default, validated positive so it cannot be switched off) and,
for the paginated listing, a bounded ``max_pages``. Go's waiters escape through
their own ``ctx.Done()``; Python's escape because the load itself ends. That is
a dependency worth stating rather than assuming: a transport that could block
forever would turn this cache into a permanent hang, which is also why the
slot's release covers every way a load can end.
"""

from __future__ import annotations

import asyncio
import threading
import time
from collections.abc import Awaitable, Callable, Hashable
from dataclasses import dataclass, field
from typing import Any, Generic, NoReturn, TypeVar

from basecamp._decoding import decoded_array, decoded_object, decoded_string
from basecamp._person_id import Refusal, parse_int64
from basecamp.errors import ApiError, CampfireIndexLoadAbortedError, NotFoundError

#: How long a cached discovery source -- a bucket's project dock, the account's
#: Campfire listing -- is reused before it is read again.
CAMPFIRE_INDEX_TTL = 600.0

#: Bounds the refresh-on-miss: a line found under no candidate re-reads the
#: cached sources, but not more often than this per source, so a run of
#: unresolvable lines cannot turn into a listing per line.
#: ``RecordingUnresolvedError.refreshed`` says whether the floor applied.
CAMPFIRE_INDEX_MIN_REFRESH = 30.0

#: How many Campfires one ``summarize`` call tries, across both sources and the
#: refresh. A project has one Campfire and a handful of pings; a bucket past this
#: bound is not a shape BC3 produces, and the call reports discovery incomplete
#: rather than calling the rest absent.
MAX_CAMPFIRE_CANDIDATES = 50

#: Caps the account-wide Campfire listing the fallback source reads. A listing
#: that overflows it is not cached and the call reports discovery incomplete:
#: the dock covers every project, so the listing only ever serves the leftover,
#: and an account with more Campfires than this should not pay a full walk per
#: TTL for it.
MAX_CAMPFIRE_LISTING = 1000

#: Bounds each discovery cache's entry count. The dock cache holds one snapshot
#: per bucket consulted within the TTL: a connector listening across every
#: project an agent can see touches hundreds of buckets, not thousands, and a
#: snapshot is a handful of ids, so this is generous headroom at a few hundred
#: KB. The bound exists so a process alive for weeks can never grow past it
#: whatever it sees. When it is reached the oldest-fetched entries go first,
#: deterministically.
CAMPFIRE_INDEX_MAX_ITEMS = 1024

K = TypeVar("K", bound=Hashable)
V = TypeVar("V")


#: The reason text a listing overflow reports. Go spells the CONSTANT rather
#: than its value, and this is a public field on
#: :class:`~basecamp.errors.CampfireDiscoveryIncompleteError` that a shared
#: fixture can pin, so the two say the same thing.
_LISTING_OVERFLOW_REASON = "campfire listing exceeds MaxCampfireListing"


class CampfireListingOverflow(Exception):
    """The account-wide listing exceeded :data:`MAX_CAMPFIRE_LISTING`.

    Internal: the resolver turns it into the typed
    :class:`~basecamp.errors.CampfireDiscoveryIncompleteError`. It is never
    cached, so the next call pays for the listing again rather than inheriting a
    verdict taken from a truncated one.
    """


@dataclass(frozen=True)
class SourceRead:
    """One consultation of a discovery source.

    The candidate ids it holds for the bucket, when that snapshot was fetched,
    and whether it predated the call. The fetch time is what lets a caller tell
    a snapshot it already consulted from a newer one, whoever loaded it.
    """

    ids: list[int]
    fetched: float
    cached: bool


@dataclass
class _Entry(Generic[V]):
    value: V
    fetched: float
    #: Publication order: the tie-breaker when fetch times are equal, so
    #: eviction order is total rather than whatever the dict happens to yield.
    seq: int


@dataclass
class _Hit(Generic[V]):
    value: V
    fetched: float
    #: Whether the value predated the call, as opposed to being loaded during it
    #: -- by this caller or by one it waited on.
    cached: bool


@dataclass
class _Load(Generic[V]):
    value: Any = None
    error: BaseException | None = None
    fetched: float = 0.0
    #: The load ended because the task that OWNED it was cancelled, as opposed
    #: to failing on its own account. Decided from WHOSE cancellation ended the
    #: load, never from the exception's type: a transport timeout can surface
    #: as the same class as a caller's cancellation, and reading one as the
    #: other makes every waiter re-run a load it should have shared.
    owner_cancelled: bool = False


@dataclass
class _SyncLoad(_Load[V]):
    done: threading.Event = field(default_factory=threading.Event)


@dataclass
class _AsyncLoad(_Load[V]):
    done: asyncio.Event = field(default_factory=asyncio.Event)


def _record_outcome(
    pending: _Load[V],
    store: _Store[K, V],
    value: Any,
    error: BaseException | None,
    *,
    owner_cancelled: bool = False,
) -> None:
    """Write a finished load's outcome onto its record.

    Everything a waiter reads lives here. The error is set first, by a bare
    assignment that cannot fail, so a waiter woken at any later point reads an
    answer rather than a record that still says "succeeded, value None".
    """
    pending.error = error
    pending.owner_cancelled = owner_cancelled
    if error is None:
        pending.value = value
        pending.fetched = store.now()


def _owner_cancellation(error: BaseException) -> bool:
    """Whether the CURRENT task's own cancellation is what ended a load.

    Go asks two questions -- was the loading caller's context done, and is the
    error that context's own -- and both halves matter. This is their Python
    pair: the error is a cancellation AND this task actually has one pending.
    An ``httpx`` timeout, or a ``CancelledError`` escaping a scope the load
    itself owns, is the load's own failure: the waiters are not sent to reload
    it, exactly as in Go. (What they are HANDED is a separate question, settled
    by :func:`_raise_to_waiter` -- a cancellation is never re-raised into a task
    that did not ask for it, whoever it belonged to.)

    It decides one thing only: whether a waiter RELOADS. It must never decide
    whether a waiter is PROTECTED -- see :func:`_raise_to_waiter` -- because
    this heuristic can read either way and being wrong about protection cancels
    a task nobody cancelled.
    """
    if not _contains_cancelled(error):
        return False
    task = asyncio.current_task()
    return task is not None and task.cancelling() > 0


def _cancellation_pending() -> bool:
    """Whether the current task has a cancellation on the books.

    Go's waiter checks ``ctx.Err() == nil`` before deciding to load for itself.
    This is that check: an exact equivalent does not exist, but a task with an
    outstanding cancellation should not be starting an HTTP request.
    """
    task = asyncio.current_task()
    return task is not None and task.cancelling() > 0


def _contains_cancelled(error: BaseException) -> bool:
    """Whether a cancellation is in here, looking inside a group as Go looks inside ``Unwrap() []error``."""
    if isinstance(error, asyncio.CancelledError):
        return True
    if isinstance(error, BaseExceptionGroup):
        return error.subgroup(asyncio.CancelledError) is not None
    return False


def _raise_to_waiter(error: BaseException) -> NoReturn:
    """Hand a finished load's failure to a caller that merely waited on it.

    Go hands the waiter ``pending.err`` as a VALUE, so its goroutine is
    untouched whatever the error was. Python has no such separation: raising
    the owner's ``CancelledError`` into a waiter tells that task -- and any
    enclosing ``TaskGroup`` -- that IT was cancelled, and the group then exits
    cleanly with the waiter's work silently missing. A ``KeyboardInterrupt`` or
    ``SystemExit`` from a loading thread is the same hazard in the sync twin.

    So the rule is about the class, not about who was cancelled: an exception
    that is not an ``Exception`` is the owning caller's own exit and is never
    re-raised into someone else's task. Everything else -- a 403, a transport
    timeout -- is the load's own failure and is shared verbatim, so N waiters
    never re-run one failed load N times.
    """
    if isinstance(error, Exception):
        raise error
    if isinstance(error, BaseExceptionGroup):
        # A mixed group carries real failures alongside the owner's exit. The
        # failures are the load's own and belong to the waiters; only the exit
        # is withheld. Substituting the whole group would throw away a 403's
        # class, code and status along with it.
        shareable = error.subgroup(Exception)
        if shareable is not None:
            raise shareable from error
    raise CampfireIndexLoadAbortedError() from error


class _Store(Generic[K, V]):
    """The bookkeeping both caches share: entries, TTL, sweeping, eviction.

    Holds no lock of its own; each cache serializes access with the primitive
    its concurrency model provides.
    """

    def __init__(self, *, ttl: float, floor: float, max_items: int, now: Callable[[], float]) -> None:
        self._ttl = ttl
        self._floor = floor
        self._max_items = max_items
        self._now = now
        self._entries: dict[K, _Entry[V]] = {}
        self._seq = 0

    def now(self) -> float:
        return self._now()

    def fresh(self, key: K, *, refresh: bool) -> _Hit[V] | None:
        """The entry for key when it may still be used, else ``None``.

        ``refresh`` narrows "still usable" from the TTL to the refresh floor, so
        a caller concluding "not found" re-reads a source it has not re-read
        lately and reuses one it has.
        """
        entry = self._entries.get(key)
        if entry is None:
            return None
        age = self._now() - entry.fetched
        if age >= self._ttl or (refresh and age >= self._floor):
            return None
        return _Hit(entry.value, entry.fetched, cached=True)

    def peek(self, key: K) -> _Hit[V] | None:
        """What the cache already holds for key, without loading."""
        return self.fresh(key, refresh=False)

    def store(self, key: K, value: V, fetched: float) -> None:
        self._sweep()
        self._make_room(key)
        self._seq += 1
        self._entries[key] = _Entry(value, fetched, self._seq)

    def _sweep(self) -> None:
        """Drop every entry past its TTL.

        Runs at each publication -- the one moment the cache does work
        proportional to a miss anyway -- so a long-lived client that has seen
        many buckets keeps a snapshot for at most a TTL past its last use plus
        the interval to the next load on any key, rather than for its lifetime.
        """
        now = self._now()
        for key in [key for key, entry in self._entries.items() if now - entry.fetched >= self._ttl]:
            del self._entries[key]

    def _make_room(self, key: K) -> None:
        """Evict oldest-fetched entries until the one about to be stored fits.

        Oldest by fetch time, and by publication order among equals. Oldest-first
        is also the right order: the entry nearest its TTL is the one least worth
        keeping.
        """
        if self._max_items <= 0 or key in self._entries:
            return  # an overwrite takes no new room
        while len(self._entries) >= self._max_items:
            oldest = min(self._entries, key=lambda k: (self._entries[k].fetched, self._entries[k].seq))
            del self._entries[oldest]


class TTLCache(Generic[K, V]):
    """A per-key cache with single-flight loading (sync).

    Concurrent callers for one key wait on the one load in progress rather than
    loading again, a failed load leaves the previous value in place and is
    shared with the waiters (so N waiters never re-run one failed load N times),
    and a refresh is honoured only once the value is older than a floor.
    """

    def __init__(
        self,
        *,
        ttl: float,
        floor: float,
        max_items: int,
        now: Callable[[], float] = time.monotonic,
    ) -> None:
        self._store: _Store[K, V] = _Store(ttl=ttl, floor=floor, max_items=max_items, now=now)
        self._lock = threading.Lock()
        self._inflight: dict[K, _SyncLoad[V]] = {}

    def peek(self, key: K) -> _Hit[V] | None:
        with self._lock:
            return self._store.peek(key)

    def get(self, key: K, *, refresh: bool, load: Callable[[], V]) -> _Hit[V]:
        with self._lock:
            hit = self._store.fresh(key, refresh=refresh)
            if hit is not None:
                return hit
            pending = self._inflight.get(key)
            owns = pending is None
            if pending is None:
                pending = _SyncLoad()
                self._inflight[key] = pending

        if not owns:
            pending.done.wait()
            if pending.error is not None:
                _raise_to_waiter(pending.error)
            # The load this call waited on is this call's load: its value is
            # fresh, not something that predated the call -- and it is read off
            # the load record, so a sweep or the bound evicting the entry in the
            # meantime cannot take it from this waiter.
            return _Hit(pending.value, pending.fetched, cached=False)

        try:
            value = load()
        except BaseException as error:
            self._publish(key, pending, None, error)
            raise
        self._publish(key, pending, value, None)
        return _Hit(value, pending.fetched, cached=False)

    def _publish(self, key: K, pending: _SyncLoad[V], value: Any, error: BaseException | None) -> None:
        # Structured exactly like the async twin's, so the two cannot drift
        # into different failure behaviour. See it for why.
        recorded = False
        try:
            _record_outcome(pending, self._store, value, error)
            recorded = True
            with self._lock:
                if error is None:
                    self._store.store(key, value, pending.fetched)
        except BaseException as failure:
            if recorded:
                # The outcome is already on the record and every waiter will
                # read it. What failed is the SHARED store -- in the async twin
                # this is a cancellation delivered while acquiring the lock,
                # which Go cannot suffer because its publish takes the mutex
                # uncancellably. Overwriting here would tell waiters a load that
                # SUCCEEDED was abandoned, or turn a 403 into a retryable
                # error; the only real cost is that the entry misses the cache
                # and the next caller loads again.
                raise
            # Recording the outcome itself failed -- a clock that raised. The
            # waiters must hear that rather than read a half-written record.
            pending.error = failure
            pending.value = None
            pending.owner_cancelled = False
            raise
        finally:
            if self._inflight.get(key) is pending:
                del self._inflight[key]
            pending.done.set()


class AsyncTTLCache(Generic[K, V]):
    """The async twin of :class:`TTLCache`.

    Adds the one rule a coroutine needs and a thread does not: when the load
    failed because the OWNER was cancelled, a waiter whose own task is still
    live goes round again and loads for itself. Once only -- a second
    owner-cancelled failure is raised rather than chased, so a run of cancelled
    owners cannot become a queue of sequential loads behind one waiter.
    """

    def __init__(
        self,
        *,
        ttl: float,
        floor: float,
        max_items: int,
        now: Callable[[], float] = time.monotonic,
    ) -> None:
        self._store: _Store[K, V] = _Store(ttl=ttl, floor=floor, max_items=max_items, now=now)
        self._lock = asyncio.Lock()
        self._inflight: dict[K, _AsyncLoad[V]] = {}

    async def peek(self, key: K) -> _Hit[V] | None:
        async with self._lock:
            return self._store.peek(key)

    async def get(self, key: K, *, refresh: bool, load: Callable[[], Awaitable[V]]) -> _Hit[V]:
        reacquired = False
        while True:
            async with self._lock:
                hit = self._store.fresh(key, refresh=refresh)
                if hit is not None:
                    return hit
                pending = self._inflight.get(key)
                owns = pending is None
                if pending is None:
                    pending = _AsyncLoad()
                    self._inflight[key] = pending

            if not owns:
                await pending.done.wait()
                if pending.error is not None:
                    # The OWNER's cancellation says nothing about the source, so
                    # a waiter goes round again and loads for itself: the key is
                    # free, so it becomes the loader and any other waiters queue
                    # behind it -- one load, not a stampede. Once only; a second
                    # owner-cancelled failure is reported rather than chased, so
                    # a run of cancelled owners cannot become a queue of
                    # sequential loads behind one waiter.
                    # Go's condition is `callerDone && ctx.Err() == nil &&
                    # !reacquired`: a waiter whose OWN context is done does not
                    # go and load. `cancelling()` is this task's nearest
                    # equivalent, and it is checked for the same reason -- a
                    # task that swallowed a cancellation without `uncancel()`
                    # would otherwise issue a live request on its way out.
                    if pending.owner_cancelled and not reacquired and not _cancellation_pending():
                        reacquired = True
                        continue
                    _raise_to_waiter(pending.error)
                return _Hit(pending.value, pending.fetched, cached=False)

            try:
                value = await load()
            except BaseException as error:
                await self._publish(key, pending, None, error, owner_cancelled=_owner_cancellation(error))
                raise
            await self._publish(key, pending, value, None)
            return _Hit(value, pending.fetched, cached=False)

    async def _publish(
        self,
        key: K,
        pending: _AsyncLoad[V],
        value: Any,
        error: BaseException | None,
        *,
        owner_cancelled: bool = False,
    ) -> None:
        # The outcome is written before anything that can suspend: a waiter
        # reads it the instant the event is set, and this task can be cancelled
        # at the lock. Were the fields written after, an interrupted
        # publication would wake every waiter onto a record saying "succeeded,
        # value None".
        recorded = False
        try:
            _record_outcome(pending, self._store, value, error, owner_cancelled=owner_cancelled)
            recorded = True
            async with self._lock:
                if error is None:
                    self._store.store(key, value, pending.fetched)
        except BaseException as failure:
            if recorded:
                # The outcome is already on the record and every waiter will
                # read it. What failed is the SHARED store -- in the async twin
                # this is a cancellation delivered while acquiring the lock,
                # which Go cannot suffer because its publish takes the mutex
                # uncancellably. Overwriting here would tell waiters a load that
                # SUCCEEDED was abandoned, or turn a 403 into a retryable
                # error; the only real cost is that the entry misses the cache
                # and the next caller loads again.
                raise
            # Publication itself failed -- a clock or a store that raised. The
            # waiters hear about that rather than reading a half-written
            # record; without this they would see "succeeded, value None".
            pending.error = failure
            pending.value = None
            raise
        finally:
            # THE release point, and it covers every way control can leave the
            # load -- return, raise, cancellation, a fault in publication
            # itself. Waiters wait with no timeout and the listing's key is a
            # bare account id, so a slot left in flight would park every later
            # caller for that whole account for the life of the client.
            #
            # Nothing here can suspend or raise, which is what makes it
            # uncancellable: two dict operations and an event. The lock the
            # store needs is not needed for them, and the identity check means
            # a load that replaced this one can never be dropped by it.
            if self._inflight.get(key) is pending:
                del self._inflight[key]
            pending.done.set()


# --- Reading a payload the way Go's typed decode reads it -------------------
#
# Go hands every one of these reads to `json.Unmarshal` against a struct, and
# the port has to reproduce BOTH of its answers, because they are different
# answers and the difference is load-bearing:
#
#   - `null` is a NO-OP at any depth. No error; the zero value stays. A null
#     body, a null `dock`, a null element, a null `id` -- each is an empty
#     thing the read goes on to use, not a failure.
#   - Anything else of the wrong type is a DECODE ERROR, and the read never
#     returns. An id of `true`, `"7"`, `7.0` or past int64 fails the whole
#     response, not the one entry carrying it.
#
# Skipping the entry instead looks harmless and is not: a skipped candidate
# spends none of the discovery budget, so the same payload reaches a different
# VERDICT -- "found under no campfire you can see" where Go says "I could not
# finish looking". Those are the two answers this composite exists to keep
# apart. A bare AttributeError off `"oops".get` is no better: this runs inside
# a cache loader whose failure is shared with every waiter on the key.
_INT64_MIN = -(2**63)
_INT64_MAX = 2**63 - 1


def _decoded_flexible_int64(value: Any, what: str) -> int:
    """``types.FlexibleInt64``: a JSON number, or a string holding one.

    BC3 serializes person ids as strings in some responses and numbers in
    others, so this field is not `_decoded_int64` and differs from it in two
    directions that are easy to get backwards:

      - a NON-NUMERIC string is 0, not an error. That 0 is the system-actor
        sentinel -- "basecamp", "campfire" -- so reading it as a person id
        would name a real person where Go names no one.
      - ``null`` IS an error here, where a plain int64 field reads it as 0.
        Measured, not assumed: `FlexibleInt64.UnmarshalJSON` is called for
        null and decodes it as a number.

    The string path is `basecamp._person_id.parse_int64`, shared with the
    pre-decode normalizer in `generated/services/_base.py` rather than copied:
    the same string reaching the two by different routes has to read the same
    way, and `FlexibleInt64.UnmarshalJSON` (`go/pkg/types/flexible_int64.go:34`)
    and `coercePersonID` (`go/pkg/basecamp/normalize.go:45`) are the same
    `ParseInt` call. Only what each does with the two refusals differs, and both
    of those are here: a syntax error is 0 (`flexible_int64.go:46`), a range
    error is raised (`:43-44`).
    """
    if isinstance(value, str):
        parsed = parse_int64(value)
        if parsed is Refusal.RANGE:
            raise ApiError(f"{what} overflows int64: {value!r}")
        return 0 if parsed is Refusal.SYNTAX else parsed
    if value is None or not isinstance(value, int) or isinstance(value, bool):
        raise ApiError(f"{what} was not an int64: {value!r}")
    if not (_INT64_MIN <= value <= _INT64_MAX):
        raise ApiError(f"{what} was not an int64: {value!r}")
    return value


def _decoded_optional_string(value: Any, what: str) -> str | None:
    """A string field whose absence stays ``None`` rather than becoming ``""``."""
    if value is None:
        return None
    if not isinstance(value, str):
        raise ApiError(f"{what} was not a string: {type(value).__name__}")
    return value


def _decoded_optional_bool(value: Any, what: str) -> bool | None:
    """A `*bool` field: null stays None, a bool passes, anything else fails.

    `1` does NOT pass. Python would read it as truthy; Go refuses a number for
    a bool outright, and this is the read side of a payload nobody here wrote.
    """
    if value is None:
        return None
    if not isinstance(value, bool):
        raise ApiError(f"{what} was not a bool: {type(value).__name__}")
    return value


def _decoded_int64(value: Any, what: str) -> int:
    """An ``int64`` field: 0 for null, the int itself, else a decode error.

    `bool` is an `int` in Python, so `True` would otherwise become a request
    path. The RANGE is part of the type: Go refuses 2**63 exactly as it refuses
    a string, and this is the only place that fact is enforced -- there is no
    ceiling downstream to catch it.
    """
    if value is None:
        return 0
    if not isinstance(value, int) or isinstance(value, bool) or not (_INT64_MIN <= value <= _INT64_MAX):
        raise ApiError(f"{what} was not an int64: {value!r}")
    return value


def _dock_campfire_ids(project: Any) -> list[int]:
    """The Campfire ids a project's dock names."""
    ids: list[int] = []
    for item in decoded_array(decoded_object(project, "the project").get("dock"), "the project dock"):
        entry = decoded_object(item, "a dock item")
        # Decoded BEFORE the name test, because Go decodes the whole body
        # before its loop sees any of it: a malformed id fails the read even
        # on an item this loop would go on to skip.
        campfire_id = _decoded_int64(entry.get("id"), "a dock item id")
        if decoded_string(entry.get("name"), "a dock item name") != "chat":
            continue
        # Go's rule is `item.ID != 0`, and a missing or null id decodes to 0.
        # A NEGATIVE id IS a candidate there, so it is one here: it spends a
        # unit of the candidate budget and 404s, both of which are observable.
        if campfire_id != 0:
            ids.append(campfire_id)
    return ids


def _campfires_by_bucket(campfires: Any) -> dict[int, list[int]]:
    by_bucket: dict[int, list[int]] = {}
    for entry in decoded_array(campfires, "the campfire listing"):
        campfire = decoded_object(entry, "a campfire")
        # Go checks the BUCKET id only -- `c.Bucket == nil || c.Bucket.ID == 0`
        # -- and appends `c.ID` with NO test whatever. So a campfire id of 0 is
        # a candidate and gets its request, and a null or absent `id` decodes
        # to exactly that 0. Screening those out cost a request Go makes and a
        # unit of the budget Go spends, which turned "I could not finish
        # looking" into "it is not there" on the same payload.
        campfire_id = _decoded_int64(campfire.get("id"), "a campfire id")
        bucket = decoded_object(campfire.get("bucket"), "a campfire bucket")
        bucket_id = _decoded_int64(bucket.get("id"), "a campfire bucket id")
        if bucket_id == 0:
            continue
        by_bucket.setdefault(bucket_id, []).append(campfire_id)
    return by_bucket


class CampfireIndex:
    """The two discovery sources, cached (sync)."""

    def __init__(self, *, now: Callable[[], float] = time.monotonic) -> None:
        self._docks: TTLCache[tuple[str, int], list[int]] = TTLCache(
            ttl=CAMPFIRE_INDEX_TTL,
            floor=CAMPFIRE_INDEX_MIN_REFRESH,
            max_items=CAMPFIRE_INDEX_MAX_ITEMS,
            now=now,
        )
        self._listings: TTLCache[str, dict[int, list[int]]] = TTLCache(
            ttl=CAMPFIRE_INDEX_TTL,
            floor=CAMPFIRE_INDEX_MIN_REFRESH,
            max_items=CAMPFIRE_INDEX_MAX_ITEMS,
            now=now,
        )

    def dock_campfires(self, account: Any, bucket_id: int, *, refresh: bool) -> SourceRead:
        """The Campfire ids a bucket's project dock names.

        A bucket that is not a project (a 404 on the project read) has none; any
        other failure of the read is raised.
        """

        def load() -> list[int]:
            try:
                project = account.projects.get(project_id=bucket_id)
            except NotFoundError:
                return []
            return _dock_campfire_ids(project)

        hit = self._docks.get((account.account_id, bucket_id), refresh=refresh, load=load)
        return SourceRead(list(hit.value), hit.fetched, hit.cached)

    def cached_listed_campfires(self, account_id: str, bucket_id: int) -> SourceRead | None:
        """What the cached account-wide listing shows in a bucket, without fetching.

        ``None`` when the listing is not cached or has expired.
        """
        hit = self._listings.peek(account_id)
        if hit is None:
            return None
        return SourceRead(list(hit.value.get(bucket_id, ())), hit.fetched, True)

    def listed_campfires(self, account: Any, bucket_id: int, *, refresh: bool) -> SourceRead:
        """The Campfire ids the account-wide listing shows in a bucket.

        A listing that overflows :data:`MAX_CAMPFIRE_LISTING` is not cached and
        raises :class:`CampfireListingOverflow`.
        """

        def load() -> dict[int, list[int]]:
            listing = account.campfires.list(max_items=MAX_CAMPFIRE_LISTING)
            if listing.meta.truncated:
                raise CampfireListingOverflow(_LISTING_OVERFLOW_REASON)
            return _campfires_by_bucket(list(listing))

        hit = self._listings.get(account.account_id, refresh=refresh, load=load)
        return SourceRead(list(hit.value.get(bucket_id, ())), hit.fetched, hit.cached)


class AsyncCampfireIndex:
    """The two discovery sources, cached (async)."""

    def __init__(self, *, now: Callable[[], float] = time.monotonic) -> None:
        self._docks: AsyncTTLCache[tuple[str, int], list[int]] = AsyncTTLCache(
            ttl=CAMPFIRE_INDEX_TTL,
            floor=CAMPFIRE_INDEX_MIN_REFRESH,
            max_items=CAMPFIRE_INDEX_MAX_ITEMS,
            now=now,
        )
        self._listings: AsyncTTLCache[str, dict[int, list[int]]] = AsyncTTLCache(
            ttl=CAMPFIRE_INDEX_TTL,
            floor=CAMPFIRE_INDEX_MIN_REFRESH,
            max_items=CAMPFIRE_INDEX_MAX_ITEMS,
            now=now,
        )

    async def dock_campfires(self, account: Any, bucket_id: int, *, refresh: bool) -> SourceRead:
        async def load() -> list[int]:
            try:
                project = await account.projects.get(project_id=bucket_id)
            except NotFoundError:
                return []
            return _dock_campfire_ids(project)

        hit = await self._docks.get((account.account_id, bucket_id), refresh=refresh, load=load)
        return SourceRead(list(hit.value), hit.fetched, hit.cached)

    async def cached_listed_campfires(self, account_id: str, bucket_id: int) -> SourceRead | None:
        hit = await self._listings.peek(account_id)
        if hit is None:
            return None
        return SourceRead(list(hit.value.get(bucket_id, ())), hit.fetched, True)

    async def listed_campfires(self, account: Any, bucket_id: int, *, refresh: bool) -> SourceRead:
        async def load() -> dict[int, list[int]]:
            listing = await account.campfires.list(max_items=MAX_CAMPFIRE_LISTING)
            if listing.meta.truncated:
                raise CampfireListingOverflow(_LISTING_OVERFLOW_REASON)
            return _campfires_by_bucket(list(listing))

        hit = await self._listings.get(account.account_id, refresh=refresh, load=load)
        return SourceRead(list(hit.value.get(bucket_id, ())), hit.fetched, hit.cached)


class ChatLineSearch:
    """One ``summarize`` call's discovery bookkeeping.

    Holds the candidates already tried, the budget still unspent, and whether a
    candidate was left untried for want of budget. The read itself lives in the
    service, because the sync and async paths differ there and nowhere else.
    """

    def __init__(self, budget: int = MAX_CAMPFIRE_CANDIDATES) -> None:
        #: Candidates this call may still try. Read by the resolver, which
        #: refuses to re-read a source it cannot spend a candidate on.
        self.budget = budget
        #: Candidates that answered 404, in order.
        self.tried: list[int] = []
        #: A candidate was left untried for want of budget.
        self.skipped = False

    def candidates(self, ids: list[int]):
        """Yield each candidate not yet tried, spending one unit of budget each.

        Stops -- and sets :attr:`skipped` -- as soon as the budget is gone, so
        nothing unsearched is later reported absent.
        """
        for campfire_id in ids:
            if campfire_id in self.tried:
                continue
            if self.budget <= 0:
                self.skipped = True
                return
            self.budget -= 1
            yield campfire_id

    def record_miss(self, campfire_id: int) -> None:
        """Record a candidate that answered 404: not here, try the next."""
        self.tried.append(campfire_id)
