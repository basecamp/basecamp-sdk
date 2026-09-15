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
    /// ten-minute TTL. Seconds rather than a `Date`, because the cache reads a
    /// monotonic counter: a wall clock would let an NTP step freeze the TTL.
    private final class TestClock: @unchecked Sendable {
        private let lock = NSLock()
        private var _now: TimeInterval = 10_000

        var now: TimeInterval { lock.withLock { _now } }
        func advance(_ seconds: TimeInterval) { lock.withLock { _now += seconds } }
        func reader() -> @Sendable () -> TimeInterval { { [self] in now } }
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

    /// A load nobody is waiting for is abandoned, so the key becomes reloadable.
    ///
    /// Without this, a load that never settles — a token provider that blocks,
    /// which is awaited BEFORE the request timeout applies, or a custom
    /// transport that ignores it — keeps the key claimed for the life of the
    /// client, and every later caller joins it instead of loading. Go reaches
    /// the same place from the other side: its loader runs under the owning
    /// caller's context and dies with it.
    func testALoadNobodyIsWaitingForIsAbandoned() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)
        let started = Expectation()

        let leaver = Task {
            try await cache.value(for: "k", refresh: false) {
                await started.wait()
                // A cooperative loader, which is what abandonment needs on both
                // sides: cancellation only helps a loader that looks.
                try Task.checkCancellation()
                return loads.increment()
            }
        }
        while await cache.waiterCount(for: "k") < 1 { await Task.yield() }
        leaver.cancel()
        do {
            _ = try await leaver.value
            XCTFail("a cancelled waiter must stop waiting")
        } catch is CancellationError {}

        // Detached at once, not when the cancelled load eventually returns: a
        // caller arriving in that window must not join a flight already on its
        // way to failing and be handed its CancellationError.
        let claimed = await cache.isClaimed("k")
        XCTAssertFalse(claimed, "the abandoned claim is detached immediately")

        let after = try await cache.value(for: "k", refresh: false) { loads.increment() }
        XCTAssertEqual(after.value, 1, "the key reloaded rather than joining a dead load")

        // And the abandoned load finishing afterwards must not evict the claim
        // that replaced it, nor resume anybody.
        await started.fulfill()
        await Task.yield()
        let stored = await cache.cached("k")
        XCTAssertEqual(stored?.value, 1)
    }

    /// The other half of the same rule: one caller leaving does NOT abandon a
    /// load someone else is still waiting for.
    func testALoadIsKeptWhileAnyoneIsStillWaiting() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)
        let started = Expectation()

        async let keeper = cache.value(for: "k", refresh: false) {
            await started.wait()
            try Task.checkCancellation()
            return loads.increment()
        }
        while await cache.waiterCount(for: "k") < 1 { await Task.yield() }

        let leaver = Task { try await cache.value(for: "k", refresh: false) { loads.increment() } }
        while await cache.waiterCount(for: "k") < 2 { await Task.yield() }
        leaver.cancel()
        do {
            _ = try await leaver.value
            XCTFail("the leaver must stop waiting")
        } catch is CancellationError {}

        await started.fulfill()
        let kept = try await keeper
        XCTAssertEqual(kept.value, 1, "the load was not cancelled out from under the caller left")
    }

    /// Eviction is by IDENTITY, never by key. A straggler from an abandoned
    /// flight — its cancellation landing, or its load finally returning — must
    /// not remove the claim that replaced it, because that claim is still
    /// loading and its waiters would lose their single-flight guarantee. The
    /// listing cache's key is a bare account id, so the blast radius of getting
    /// this wrong is every bucket in the account.
    func testAStragglerFromAnAbandonedFlightLeavesItsSuccessorAlone() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)
        let firstGate = Expectation()
        let secondGate = Expectation()

        // Flight one, abandoned before it can finish.
        let leaver = Task {
            try await cache.value(for: "k", refresh: false) {
                await firstGate.wait()
                return loads.increment()
            }
        }
        while await cache.waiterCount(for: "k") < 1 { await Task.yield() }
        leaver.cancel()
        do {
            _ = try await leaver.value
            XCTFail("the leaver must stop waiting")
        } catch is CancellationError {}

        // Flight two claims the same key and is still loading.
        async let keeper = cache.value(for: "k", refresh: false) {
            await secondGate.wait()
            return loads.increment()
        }
        while await cache.waiterCount(for: "k") < 1 { await Task.yield() }

        // Now let the abandoned flight run to completion underneath it.
        await firstGate.fulfill()
        await Task.yield()
        await Task.yield()
        let stillClaimed = await cache.isClaimed("k")
        XCTAssertTrue(
            stillClaimed, "the successor's claim survives the straggler finishing")

        await secondGate.fulfill()
        let kept = try await keeper
        XCTAssertEqual(kept.cached, false)
        let waiters = await cache.waiterCount(for: "k")
        XCTAssertEqual(waiters, 0, "and it completes normally")
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

    /// The single-flight slot has to be released on every path control can leave
    /// the load by, or one dead loader parks every later caller for that key for
    /// the life of the process — and waiters wait with no timeout. Go spells
    /// this with a deferred recover; here the release is in a `catch` that the
    /// only non-fatal abnormal exit (a thrown error, cancellation included) also
    /// takes.
    func testAKeyWhoseLoadFailedIsFreeToLoadAgain() async throws {
        let clock = TestClock()
        let cache = makeCache(clock: clock)
        struct Boom: Error {}

        do {
            _ = try await cache.value(for: "k", refresh: false) { throw Boom() }
            XCTFail("expected the load to fail")
        } catch is Boom {}

        let stillClaimed = await cache.waiterCount(for: "k")
        XCTAssertEqual(stillClaimed, 0, "the slot is not still claimed")
        let after = try await cache.value(for: "k", refresh: false) { 7 }
        XCTAssertEqual(after.value, 7, "a later caller is served rather than parked")
    }

    /// The same question from the other side: the caller that CLAIMED the key
    /// walks away. The load is nobody's to cancel, so it finishes, publishes and
    /// releases the slot anyway.
    func testCancellingTheCallerThatStartedTheLoadStillReleasesTheKey() async throws {
        let clock = TestClock()
        let loads = LoadCounter()
        let cache = makeCache(clock: clock)
        let started = Expectation()

        let claimer = Task {
            try await cache.value(for: "k", refresh: false) {
                await started.wait()
                return loads.increment()
            }
        }
        while await cache.waiterCount(for: "k") < 1 { await Task.yield() }
        claimer.cancel()

        do {
            _ = try await claimer.value
            XCTFail("a cancelled caller must stop waiting")
        } catch is CancellationError {}

        await started.fulfill()
        // The load publishes for whoever comes next, and the key is usable.
        var stored = await cache.cached("k")
        while stored == nil {
            await Task.yield()
            stored = await cache.cached("k")
        }
        XCTAssertEqual(stored?.value, 1)
        let stillClaimed = await cache.waiterCount(for: "k")
        XCTAssertEqual(stillClaimed, 0)
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
