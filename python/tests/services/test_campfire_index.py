"""Tests for the TTL caches behind Campfire discovery.

The cache is what makes ``recordings.summarize`` cheap for a burst of chat-line
pointers, and it is the part of the composite the conformance fixture cannot
see at all: a fixture is one call. Its lifetime, its single-flight loading, its
refresh floor and its bound are pinned here.
"""

from __future__ import annotations

import asyncio
import threading

import pytest

from basecamp.services._campfire_index import AsyncTTLCache, TTLCache


class Clock:
    def __init__(self) -> None:
        self.now = 0.0

    def __call__(self) -> float:
        return self.now


def _cache(clock: Clock, *, ttl: float = 100.0, floor: float = 10.0, max_items: int = 8) -> TTLCache:
    return TTLCache(ttl=ttl, floor=floor, max_items=max_items, now=clock)


def _async_cache(clock: Clock, *, ttl: float = 100.0, floor: float = 10.0, max_items: int = 8) -> AsyncTTLCache:
    return AsyncTTLCache(ttl=ttl, floor=floor, max_items=max_items, now=clock)


class TestLifetime:
    def test_a_hit_within_the_ttl_does_not_load(self):
        clock = Clock()
        cache = _cache(clock)
        loads = []

        for _ in range(3):
            cache.get("k", refresh=False, load=lambda: loads.append(1) or "v")

        assert len(loads) == 1

    def test_the_first_load_is_not_reported_as_cached(self):
        # `cached` is what tells the resolver whether a source predated the
        # call, which is what decides whether re-reading it could help.
        clock = Clock()
        cache = _cache(clock)

        assert cache.get("k", refresh=False, load=lambda: "v").cached is False
        assert cache.get("k", refresh=False, load=lambda: "v").cached is True

    def test_a_value_past_the_ttl_is_reloaded(self):
        clock = Clock()
        cache = _cache(clock, ttl=100.0)
        cache.get("k", refresh=False, load=lambda: "first")

        clock.now = 100.0

        assert cache.get("k", refresh=False, load=lambda: "second").value == "second"

    def test_refresh_reloads_past_the_floor_and_not_before(self):
        clock = Clock()
        cache = _cache(clock, ttl=100.0, floor=10.0)
        cache.get("k", refresh=False, load=lambda: "first")

        clock.now = 9.0
        assert cache.get("k", refresh=True, load=lambda: "second").value == "first"

        clock.now = 10.0
        assert cache.get("k", refresh=True, load=lambda: "second").value == "second"

    def test_peek_never_loads(self):
        clock = Clock()
        cache = _cache(clock)

        assert cache.peek("k") is None

        cache.get("k", refresh=False, load=lambda: "v")
        assert cache.peek("k").value == "v"

        clock.now = 100.0
        assert cache.peek("k") is None, "an expired entry is not a hit"


class TestFailures:
    def test_a_failed_load_leaves_the_previous_value_in_place(self):
        clock = Clock()
        cache = _cache(clock, floor=0.0)
        cache.get("k", refresh=False, load=lambda: "first")

        def boom():
            raise RuntimeError("upstream is down")

        with pytest.raises(RuntimeError):
            cache.get("k", refresh=True, load=boom)

        assert cache.peek("k").value == "first"

    def test_a_failed_load_releases_the_key(self):
        clock = Clock()
        cache = _cache(clock)

        def boom():
            raise RuntimeError("upstream is down")

        with pytest.raises(RuntimeError):
            cache.get("k", refresh=False, load=boom)
        assert cache.get("k", refresh=False, load=lambda: "v").value == "v"


class TestBound:
    def test_the_oldest_entry_goes_first(self):
        clock = Clock()
        cache = _cache(clock, max_items=3)

        for index in range(3):
            clock.now = float(index)
            cache.get(index, refresh=False, load=lambda index=index: index)

        clock.now = 3.0
        cache.get(3, refresh=False, load=lambda: 3)

        assert cache.peek(0) is None, "the entry nearest its TTL is the one least worth keeping"
        assert cache.peek(1).value == 1
        assert cache.peek(3).value == 3

    def test_an_overwrite_takes_no_new_room(self):
        clock = Clock()
        cache = _cache(clock, max_items=2, floor=0.0)
        cache.get("a", refresh=False, load=lambda: "a")
        clock.now = 1.0
        cache.get("b", refresh=False, load=lambda: "b")

        clock.now = 2.0
        cache.get("a", refresh=True, load=lambda: "a2")

        assert cache.peek("a").value == "a2"
        assert cache.peek("b").value == "b"

    def test_expired_entries_are_swept_at_each_publication(self):
        # A long-lived client that has seen many buckets must not hold a
        # snapshot for its whole lifetime.
        clock = Clock()
        cache = _cache(clock, ttl=10.0, max_items=1000)
        cache.get("stale", refresh=False, load=lambda: "v")

        clock.now = 50.0
        cache.get("fresh", refresh=False, load=lambda: "v")

        assert cache._store._entries.keys() == {"fresh"}


class TestSingleFlight:
    def test_concurrent_callers_share_one_load(self):
        clock = Clock()
        cache = _cache(clock)
        started = threading.Event()
        release = threading.Event()
        loads = []
        results = []

        def load():
            loads.append(1)
            started.set()
            release.wait(5)
            return "v"

        def call():
            results.append(cache.get("k", refresh=False, load=load).value)

        threads = [threading.Thread(target=call) for _ in range(4)]
        threads[0].start()
        assert started.wait(5)
        for thread in threads[1:]:
            thread.start()
        # Give the waiters a moment to park on the in-flight load rather than
        # racing ahead and loading for themselves.
        release.set()
        for thread in threads:
            thread.join(5)

        assert len(loads) == 1
        assert results == ["v"] * 4

    def test_a_failed_load_is_shared_with_its_waiters(self):
        # N waiters must never re-run one failed load N times.
        clock = Clock()
        cache = _cache(clock)
        started = threading.Event()
        release = threading.Event()
        loads = []
        errors = []

        def load():
            loads.append(1)
            started.set()
            release.wait(5)
            raise RuntimeError("upstream is down")

        def call():
            try:
                cache.get("k", refresh=False, load=load)
            except RuntimeError as error:
                errors.append(error)

        threads = [threading.Thread(target=call) for _ in range(3)]
        threads[0].start()
        assert started.wait(5)
        for thread in threads[1:]:
            thread.start()
        release.set()
        for thread in threads:
            thread.join(5)

        assert len(loads) == 1
        assert len(errors) == 3


@pytest.mark.asyncio
class TestAsyncCache:
    async def test_a_hit_within_the_ttl_does_not_load(self):
        clock = Clock()
        cache = _async_cache(clock)
        loads = []

        async def load():
            loads.append(1)
            return "v"

        for _ in range(3):
            await cache.get("k", refresh=False, load=load)

        assert len(loads) == 1

    async def test_peek_never_loads(self):
        clock = Clock()
        cache = _async_cache(clock)

        async def load():
            return "v"

        assert await cache.peek("k") is None
        await cache.get("k", refresh=False, load=load)
        assert (await cache.peek("k")).value == "v"

    async def test_concurrent_callers_share_one_load(self):
        clock = Clock()
        cache = _async_cache(clock)
        loads = []
        gate = asyncio.Event()

        async def load():
            loads.append(1)
            await gate.wait()
            return "v"

        tasks = [asyncio.create_task(cache.get("k", refresh=False, load=load)) for _ in range(4)]
        await asyncio.sleep(0)
        gate.set()
        hits = await asyncio.gather(*tasks)

        assert len(loads) == 1
        assert [hit.value for hit in hits] == ["v"] * 4

    async def test_a_failed_load_is_shared_with_its_waiters(self):
        clock = Clock()
        cache = _async_cache(clock)
        loads = []
        gate = asyncio.Event()

        async def load():
            loads.append(1)
            await gate.wait()
            raise RuntimeError("upstream is down")

        tasks = [asyncio.create_task(cache.get("k", refresh=False, load=load)) for _ in range(3)]
        await asyncio.sleep(0)
        gate.set()
        outcomes = await asyncio.gather(*tasks, return_exceptions=True)

        assert len(loads) == 1
        assert all(isinstance(outcome, RuntimeError) for outcome in outcomes)

    async def test_a_live_waiter_reloads_when_the_owner_was_cancelled(self):
        # The owner's cancellation is the owner's, not the waiter's: a waiter
        # whose own task is live loads for itself rather than inheriting a
        # failure that says nothing about the source.
        clock = Clock()
        cache = _async_cache(clock)
        loads = []
        first_started = asyncio.Event()
        gate = asyncio.Event()

        async def load():
            loads.append(1)
            if len(loads) == 1:
                first_started.set()
                await gate.wait()
            return "v"

        owner = asyncio.create_task(cache.get("k", refresh=False, load=load))
        await first_started.wait()
        waiter = asyncio.create_task(cache.get("k", refresh=False, load=load))
        await asyncio.sleep(0)
        owner.cancel()

        with pytest.raises(asyncio.CancelledError):
            await owner
        assert (await waiter).value == "v"
        assert len(loads) == 2

    async def test_the_waiters_of_a_cancelled_owner_are_not_left_parked(self):
        clock = Clock()
        cache = _async_cache(clock)
        started = asyncio.Event()
        gate = asyncio.Event()

        async def blocking_load():
            started.set()
            await gate.wait()
            return "v"

        async def failing_load():
            raise RuntimeError("upstream is down")

        owner = asyncio.create_task(cache.get("k", refresh=False, load=blocking_load))
        await started.wait()
        waiter = asyncio.create_task(cache.get("k", refresh=False, load=failing_load))
        await asyncio.sleep(0)
        owner.cancel()

        with pytest.raises(asyncio.CancelledError):
            await owner
        # The waiter goes round once (the owner's cancellation is not its), and
        # its OWN load's failure is its own to raise.
        with pytest.raises(RuntimeError):
            await asyncio.wait_for(waiter, timeout=5)
