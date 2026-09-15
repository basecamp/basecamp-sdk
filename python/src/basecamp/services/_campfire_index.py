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
"""

from __future__ import annotations

import asyncio
import threading
import time
from collections.abc import Awaitable, Callable, Hashable
from dataclasses import dataclass, field
from typing import Any, Generic, TypeVar

from basecamp.errors import NotFoundError

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


@dataclass
class _SyncLoad(_Load[V]):
    done: threading.Event = field(default_factory=threading.Event)


@dataclass
class _AsyncLoad(_Load[V]):
    done: asyncio.Event = field(default_factory=asyncio.Event)


def _record_outcome(pending: _Load[V], store: _Store[K, V], value: Any, error: BaseException | None) -> None:
    """Write a finished load's outcome onto its record.

    Everything a waiter reads lives here, and it is all set in one go with
    nothing between that can suspend or raise, so a waiter woken at any later
    point sees a complete answer rather than a half-written one.
    """
    pending.error = error
    if error is None:
        pending.value = value
        pending.fetched = store.now()


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
                raise pending.error
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
        # Written before the lock, and released after it, for the reasons the
        # async twin's _publish spells out — kept identical here so the two
        # cannot drift into different failure behaviour.
        _record_outcome(pending, self._store, value, error)
        try:
            with self._lock:
                if error is None:
                    self._store.store(key, value, pending.fetched)
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
                    # The load ran in the owning task. If that task was
                    # cancelled, the failure is its, not this caller's: a waiter
                    # whose own task is live goes round again and loads for
                    # itself (the key is free, so it becomes the loader and any
                    # other waiters queue behind it -- one load, not a
                    # stampede). Any other error is the load's own and is
                    # shared.
                    if isinstance(pending.error, asyncio.CancelledError) and not reacquired:
                        reacquired = True
                        continue
                    raise pending.error
                return _Hit(pending.value, pending.fetched, cached=False)

            try:
                value = await load()
            except BaseException as error:
                await self._publish(key, pending, None, error)
                raise
            await self._publish(key, pending, value, None)
            return _Hit(value, pending.fetched, cached=False)

    async def _publish(self, key: K, pending: _AsyncLoad[V], value: Any, error: BaseException | None) -> None:
        # The outcome is written onto the load record BEFORE anything that can
        # suspend. A waiter reads it the instant the event is set, and this
        # task can be cancelled at the lock: were the fields written after,
        # an interrupted publication would wake every waiter onto a record
        # saying "succeeded, value None".
        _record_outcome(pending, self._store, value, error)
        try:
            async with self._lock:
                if error is None:
                    self._store.store(key, value, pending.fetched)
        finally:
            # Neither of these may be skipped, whatever happened above: a key
            # left in flight behind a finished load parks every later caller on
            # it forever, and an event nobody sets parks the current ones. Both
            # are single statements with no suspension between them, so the
            # lock the store needs is not needed here — and the identity check
            # means a load that replaced this one can never be dropped by it.
            if self._inflight.get(key) is pending:
                del self._inflight[key]
            pending.done.set()


def _dock_campfire_ids(project: dict[str, Any]) -> list[int]:
    """The Campfire ids a project's dock names."""
    ids: list[int] = []
    for item in project.get("dock") or ():
        if not isinstance(item, dict):
            continue
        if item.get("name") == "chat" and item.get("id"):
            ids.append(item["id"])
    return ids


def _campfires_by_bucket(campfires: list[dict[str, Any]]) -> dict[int, list[int]]:
    by_bucket: dict[int, list[int]] = {}
    for campfire in campfires:
        bucket = campfire.get("bucket") or {}
        bucket_id = bucket.get("id")
        if not bucket_id:
            continue
        by_bucket.setdefault(bucket_id, []).append(campfire["id"])
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
                raise CampfireListingOverflow(f"campfire listing exceeds {MAX_CAMPFIRE_LISTING}")
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
                raise CampfireListingOverflow(f"campfire listing exceeds {MAX_CAMPFIRE_LISTING}")
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
        self._budget = budget
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
            if self._budget <= 0:
                self.skipped = True
                return
            self._budget -= 1
            yield campfire_id

    def record_miss(self, campfire_id: int) -> None:
        """Record a candidate that answered 404: not here, try the next."""
        self.tried.append(campfire_id)
