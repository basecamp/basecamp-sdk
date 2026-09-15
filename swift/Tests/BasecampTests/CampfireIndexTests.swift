import XCTest

@testable import Basecamp

/// The discovery cache's own contract.
///
/// None of this is reachable from a conformance fixture: a fixture scripts one
/// call's requests, and everything below is about what a SECOND call sees — after
/// a TTL, under a refresh floor, behind another caller's load, or past the entry
/// bound. Those are the properties that decide whether a long-lived connector
/// re-reads a project dock once per line or once per ten minutes, and whether a
/// process alive for weeks grows without limit.
final class CampfireIndexTests: XCTestCase {
    /// A clock the test moves by hand. Wall-clock sleeps cannot express a
    /// ten-minute TTL.
    private final class TestClock: @unchecked Sendable {
        private let lock = NSLock()
        private var _now = Date(timeIntervalSince1970: 1_700_000_000)

        var now: Date { lock.withLock { _now } }
        func advance(_ seconds: TimeInterval) { lock.withLock { _now += seconds } }
        func reader() -> @Sendable () -> Date { { [self] in now } }
    }

    private final class LoadCounter: @unchecked Sendable {
        private let lock = NSLock()
        private var _count = 0
        var count: Int { lock.withLock { _count } }
        func increment() -> Int { lock.withLock { _count += 1; return _count } }
    }

    private func makeCache(
        clock: TestClock, ttl: TimeInterval = 600, floor: TimeInterval = 30, maxItems: Int = 1024
    ) -> TTLCache<String, Int> {
        TTLCache(clock: clock.reader(), ttl: ttl, refreshFloor: floor, maxItems: maxItems)
    }

    func testAValueWithinTheTTLIsServedWithoutLoading() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)

        let first = try await cache.value(for: "k", refresh: false) { loads.increment() }
        XCTAssertFalse(first.cached, "the load this call made is this call's, not a prior snapshot")

        clock.advance(599)
        let second = try await cache.value(for: "k", refresh: false) { loads.increment() }

        XCTAssertEqual(loads.count, 1)
        XCTAssertTrue(second.cached)
        XCTAssertEqual(second.fetchedAt, first.fetchedAt, "the fetch time identifies the snapshot")
    }

    func testAValuePastTheTTLIsLoadedAgain() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)

        _ = try await cache.value(for: "k", refresh: false) { loads.increment() }
        clock.advance(600)
        let second = try await cache.value(for: "k", refresh: false) { loads.increment() }

        XCTAssertEqual(loads.count, 2)
        XCTAssertEqual(second.value, 2)
        XCTAssertFalse(second.cached)
    }

    /// The refresh floor is what keeps a run of unresolvable lines from turning
    /// into one listing per line.
    func testARefreshIsDeclinedBeneathTheFloorAndHonouredAboveIt() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)

        _ = try await cache.value(for: "k", refresh: false) { loads.increment() }

        clock.advance(29)
        let declined = try await cache.value(for: "k", refresh: true) { loads.increment() }
        XCTAssertEqual(loads.count, 1, "under the floor, a refresh reuses the snapshot")
        XCTAssertTrue(declined.cached)

        clock.advance(2)
        let honoured = try await cache.value(for: "k", refresh: true) { loads.increment() }
        XCTAssertEqual(loads.count, 2, "past the floor it re-reads, even though the TTL is not up")
        XCTAssertFalse(honoured.cached)
    }

    func testConcurrentCallersForOneKeyShareOneLoad() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)
        let started = Expectation()

        async let a = cache.value(for: "k", refresh: false) {
            await started.wait()
            return loads.increment()
        }
        async let b = cache.value(for: "k", refresh: false) {
            await started.wait()
            return loads.increment()
        }
        // Wait for both callers to have ARRIVED, rather than guessing at it with
        // a sleep: a sleep that ran long would let the first load publish and
        // the second call read a cached snapshot, and the test would be asking a
        // different question on a slow machine.
        while await cache.waiterCount(for: "k") < 2 { await Task.yield() }
        await started.fulfill()

        let (first, second) = try await (a, b)
        XCTAssertEqual(loads.count, 1, "a waiter takes the load in progress rather than starting one")
        XCTAssertEqual(first.value, second.value)
        XCTAssertFalse(second.cached, "the load it waited on is its own load, not a prior snapshot")
    }

    /// Go's waiter selects on `ctx.Done()` and leaves the moment its caller is
    /// cancelled. Awaiting an unstructured task's value does not do that in
    /// Swift, so the cache resumes waiters itself — and this is the test that
    /// says so. Without it a `summarize` under a deadline would sit out the
    /// whole HTTP read it no longer wants.
    func testACancelledWaiterStopsWaitingWhileTheLoadCarriesOnForTheRest() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)
        let started = Expectation()

        // The caller that claims the key and starts the load.
        async let keeper = cache.value(for: "k", refresh: false) {
            await started.wait()
            return loads.increment()
        }
        while await cache.waiterCount(for: "k") < 1 { await Task.yield() }

        // A second caller, which is then cancelled while the load is in flight.
        let leaver = Task {
            try await cache.value(for: "k", refresh: false) { loads.increment() }
        }
        while await cache.waiterCount(for: "k") < 2 { await Task.yield() }
        leaver.cancel()

        do {
            _ = try await leaver.value
            XCTFail("a cancelled waiter must stop waiting")
        } catch is CancellationError {}

        // The load was never the leaver's to cancel: it is still running, and it
        // still answers the caller that is waiting on it.
        await started.fulfill()
        let kept = try await keeper
        XCTAssertEqual(kept.value, 1)
        XCTAssertEqual(loads.count, 1, "one load, whoever walked away from it")

        let stored = await cache.cached("k")
        XCTAssertEqual(stored?.value, 1, "and it published, so the next caller pays nothing")
    }

    func testAFailedLoadLeavesThePreviousValueInPlace() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)
        struct Boom: Error {}

        _ = try await cache.value(for: "k", refresh: false) { loads.increment() }
        clock.advance(31)  // past the refresh floor, well inside the TTL

        do {
            _ = try await cache.value(for: "k", refresh: true) { throw Boom() }
            XCTFail("expected the load to fail")
        } catch is Boom {}

        // The failure is the caller's to handle; it does not evict what the
        // cache already holds, and the key is free to load again.
        let after = try await cache.value(for: "k", refresh: false) { loads.increment() }
        XCTAssertEqual(after.value, 1, "the snapshot the failed refresh could not replace")
        XCTAssertTrue(after.cached)
        XCTAssertEqual(loads.count, 1)
    }

    func testTheEntryBoundEvictsTheOldestFetchedFirst() async throws {
        let clock = TestClock()
        let cache = makeCache(clock: clock, maxItems: 2)

        _ = try await cache.value(for: "oldest", refresh: false) { 1 }
        clock.advance(1)
        _ = try await cache.value(for: "middle", refresh: false) { 2 }
        clock.advance(1)
        _ = try await cache.value(for: "newest", refresh: false) { 3 }

        let count = await cache.count
        XCTAssertEqual(count, 2, "the bound holds whatever the process has seen")
        let evicted = await cache.cached("oldest")
        XCTAssertNil(evicted, "the entry nearest its TTL is the one least worth keeping")
        let kept = await cache.cached("newest")
        XCTAssertEqual(kept?.value, 3)
    }

    func testExpiredEntriesAreSweptAtPublication() async throws {
        let clock = TestClock()
        let cache = makeCache(clock: clock)

        _ = try await cache.value(for: "stale", refresh: false) { 1 }
        clock.advance(601)
        _ = try await cache.value(for: "fresh", refresh: false) { 2 }

        let count = await cache.count
        XCTAssertEqual(
            count, 1,
            "a snapshot nobody asks for again is dropped at the next load, not held for the process's life")
    }

    func testCachedDoesNotLoadAndExpires() async throws {
        let clock = TestClock()
        let cache = makeCache(clock: clock)

        var peeked = await cache.cached("k")
        XCTAssertNil(peeked, "peeking an absent key must not fetch it")

        _ = try await cache.value(for: "k", refresh: false) { 5 }
        peeked = await cache.cached("k")
        XCTAssertEqual(peeked?.value, 5)
        XCTAssertEqual(peeked?.cached, true)

        clock.advance(600)
        peeked = await cache.cached("k")
        XCTAssertNil(peeked)
    }
}

/// A one-shot gate, so a test can hold two loads open at once without guessing
/// at a sleep for the part that matters.
private actor Expectation {
    private var fulfilled = false
    private var waiters: [CheckedContinuation<Void, Never>] = []

    func fulfill() {
        guard !fulfilled else { return }
        fulfilled = true
        for waiter in waiters { waiter.resume() }
        waiters.removeAll()
    }

    func wait() async {
        if fulfilled { return }
        await withCheckedContinuation { waiters.append($0) }
    }
}
