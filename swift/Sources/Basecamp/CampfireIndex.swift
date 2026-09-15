import Foundation

/// Campfire discovery for chat lines, and the two caches it reads.
///
/// A `chat.line.created` row carries the line's id and bucket, not its
/// Campfire, and the line read is `/chats/{campfireId}/lines/{lineId}`.
/// Candidates come from two sources, tried in order and each cached
/// ``RecordingsService/campfireIndexTTL``:
///
/// 1. The bucket's project dock, whose `chat` tool is the project's Campfire:
///    one project read per bucket, and the answer for every line posted in a
///    project. Cached per bucket.
/// 2. The account-wide Campfire listing (BC3 has no per-bucket one), filtered to
///    the bucket, for buckets that are not projects or whose dock did not hold
///    the line. Cached per account, so a burst of lines costs one listing.
///
/// It lives on ``BasecampClient`` — shared by every ``AccountClient`` that
/// client hands out. A client is bound to one credential, so entries are never
/// shared across authorization contexts, and every key carries the account id
/// besides. Expired snapshots are swept at each publication and each cache is
/// bounded (``CampfireIndex/maxItems``, oldest out first), so the index holds at
/// most the buckets and accounts consulted within the last TTL, and never more
/// than the bound, whatever the client has seen.
final class CampfireIndex: Sendable {
    /// Bounds each cache's entry count. The dock cache holds one snapshot per
    /// bucket consulted within the TTL: a connector listening across every
    /// project an agent can see touches hundreds of buckets, not thousands, and
    /// a snapshot is a handful of ids, so 1024 is generous headroom at a few
    /// hundred KB. The bound exists so that a process alive for weeks can never
    /// grow past it whatever it sees. When it is reached the oldest-fetched
    /// entries go first, deterministically.
    static let maxItems = 1024

    /// A bucket's dock snapshot, keyed by the account it was read under.
    struct DockKey: Hashable, Sendable {
        let accountId: String
        let bucketId: Int
    }

    let docks: TTLCache<DockKey, [Int]>
    /// The account-wide listing, grouped by bucket, keyed by account id.
    let listings: TTLCache<String, [Int: [Int]]>

    /// `clock` reads a MONOTONIC seconds counter, not a wall clock. Go's
    /// `time.Now()` carries a monotonic reading and its `Sub`/`After`
    /// comparisons use it, so its TTL and refresh floor are immune to a clock
    /// step; `Date()` is not, and a backward NTP step would make every entry's
    /// age negative — serving a stale snapshot and refusing every refresh for
    /// the length of the step, while reporting `refreshed: false` on lines it
    /// had concluded nothing about. `systemUptime` is the reading that matches
    /// Go's on both platforms, sleep behaviour included.
    init(clock: @escaping @Sendable () -> TimeInterval = { ProcessInfo.processInfo.systemUptime }) {
        docks = TTLCache(
            clock: clock, ttl: RecordingsService.campfireIndexTTL,
            refreshFloor: RecordingsService.campfireIndexMinRefresh, maxItems: Self.maxItems)
        listings = TTLCache(
            clock: clock, ttl: RecordingsService.campfireIndexTTL,
            refreshFloor: RecordingsService.campfireIndexMinRefresh, maxItems: Self.maxItems)
    }

    /// One consultation of a discovery source: the candidate ids it holds for
    /// the bucket, when that snapshot was fetched, and whether it predated the
    /// call.
    struct SourceRead: Sendable {
        var ids: [Int]
        /// A monotonic reading, comparable only against another from the same
        /// cache — which is all anything does with it.
        var fetchedAt: TimeInterval
        var cached: Bool
    }

    /// The Campfire ids a bucket's project dock names. A bucket that is not a
    /// project (a 404 on the project read) has none; any other failure of the
    /// read is thrown.
    func dockCampfires(account: AccountClient, bucketId: Int, refresh: Bool) async throws
        -> SourceRead
    {
        let key = DockKey(accountId: account.accountId, bucketId: bucketId)
        let hit = try await docks.value(for: key, refresh: refresh) {
            let project: Project
            do {
                project = try await account.projects.get(projectId: bucketId)
            } catch let error as BasecampError {
                if case .notFound = error { return [] }
                throw error
            }
            return (project.dock ?? [])
                .filter { $0.name == "chat" && $0.id != 0 }
                .map(\.id)
        }
        return SourceRead(ids: hit.value, fetchedAt: hit.fetchedAt, cached: hit.cached)
    }

    /// The Campfire ids the *cached* account-wide listing shows in a bucket,
    /// without fetching: nil when the listing is not cached or has expired.
    ///
    /// It exists so a caller can consult what the source already holds before
    /// deciding whether to pay for a fetch of it.
    func cachedListedCampfires(accountId: String, bucketId: Int) async -> SourceRead? {
        guard let hit = await listings.cached(accountId) else { return nil }
        return SourceRead(ids: hit.value[bucketId] ?? [], fetchedAt: hit.fetchedAt, cached: true)
    }

    /// The Campfire ids the account-wide listing shows in a bucket. A listing
    /// that overflows ``RecordingsService/maxCampfireListing`` is not cached and
    /// is reported by the caller as incomplete discovery, never as absence.
    func listedCampfires(account: AccountClient, bucketId: Int, refresh: Bool) async throws
        -> SourceRead
    {
        let hit = try await listings.value(for: account.accountId, refresh: refresh) {
            let listing = try await account.campfires.list(
                options: ListCampfireOptions(maxItems: RecordingsService.maxCampfireListing))
            if listing.meta.truncated { throw CampfireListingOverflow() }
            var byBucket: [Int: [Int]] = [:]
            for campfire in listing.items where campfire.bucket.id != 0 {
                byBucket[campfire.bucket.id, default: []].append(campfire.id)
            }
            return byBucket
        }
        return SourceRead(ids: hit.value[bucketId] ?? [], fetchedAt: hit.fetchedAt, cached: hit.cached)
    }
}

/// The load failure for a listing past its cap.
/// `RecordingsService.resolveChatLine` turns it into the typed
/// ``RecordingSummaryError/campfireDiscoveryIncomplete(_:)``; it never reaches a
/// caller as itself.
struct CampfireListingOverflow: Error, CustomStringConvertible {
    var description: String {
        "campfire listing exceeds \(RecordingsService.maxCampfireListing)"
    }
}

// MARK: - TTL cache

/// A per-key cache with single-flight loading: concurrent callers for one key
/// wait on the one load in progress rather than loading again, a failed load
/// leaves the previous value in place, and a refresh is honoured only once the
/// value is older than a floor.
///
/// The load runs in an unstructured `Task`, which does **not** inherit the
/// cancellation of whichever caller happened to start it, and every caller
/// waits on a continuation the cache resumes. Those two together are how this
/// differs from the Go cache it is ported from, and the pair is deliberate:
///
///   * Go's loader runs under the loading caller's `context`, so that cache has
///     to decide whether a failure belongs to the load or to the caller who
///     owned it, and let a still-live waiter go round again. Here the load
///     belongs to no caller, so every waiter shares its one outcome and there
///     is no owner-attributed failure to re-run. A caller that goes away leaves
///     the load to finish for whoever else is waiting — one short read, and the
///     snapshot it publishes is the one the next call would have paid for
///     anyway.
///
///     There is therefore no attribution question to get wrong. Go has to
///     decide whether a failed load was the owning caller's doing — and has to
///     decide it from *whose* context ended, not from the error, because a
///     transport timeout looks exactly like a cancellation while the caller is
///     still alive; a port that reads the error type instead re-runs a load Go
///     would have shared, once per waiter. Here the load runs under no caller's
///     task, so every failure is the load's own and is shared, and neither
///     `CancellationError` nor `URLError.cancelled` can be mistaken for
///     anything. Publication is uncancellable for the same reason: nothing
///     holds the load task to cancel it, and neither the publish nor the
///     release suspends.
///
///     What bounds a load that never settles is mostly the transport: every
///     request carries `BasecampConfig.timeoutInterval` (30 s by default) as
///     its `URLRequest.timeoutInterval`. That is not the whole story, though,
///     and the gap is worth naming — a token provider is awaited BEFORE the
///     request is built, so a provider that blocks is outside that timeout, as
///     is a custom transport that ignores it. Go's loader inherits the owning
///     caller's context and dies with it; here the load is abandoned when the
///     last waiter leaves, which is the same guarantee reached the other way
///     round. Both still depend on the loader observing cancellation at all.
///
///   * A cancelled caller still stops waiting *at once*, which is the half that
///     does not come for free: `await someTask.value` is not interrupted by the
///     awaiting task's cancellation, so a waiter that simply awaited the load
///     task would sit there until the HTTP read returned. Go's waiter selects
///     on `ctx.Done()` and leaves immediately, and a composite with a deadline
///     needs the same, so each waiter registers a continuation and a
///     cancellation handler resumes it with `CancellationError`. The load
///     carries on for the waiters that remain.
actor TTLCache<Key: Hashable & Sendable, Value: Sendable> {
    /// What a cache read hands back: the value, when it was fetched, and whether
    /// it predated the call (as opposed to being loaded during it, by this
    /// caller or by one it waited on). The fetch time is what lets a caller tell
    /// a snapshot it already consulted from a newer one, whoever loaded it.
    struct Hit: Sendable {
        let value: Value
        let fetchedAt: TimeInterval
        let cached: Bool
    }

    private struct Entry {
        let value: Value
        let fetchedAt: TimeInterval
        /// Publication order: the tie-breaker when fetch times are equal, so
        /// eviction is total rather than whatever dictionary order visits first.
        let sequence: UInt64
    }

    /// One claimed key: the callers waiting on it, and the load running behind
    /// them.
    ///
    /// The task is held so the claim can be ABANDONED when the last waiter
    /// leaves. Without that, a load nobody is waiting for keeps the key claimed
    /// until it returns, and every later caller joins it rather than starting a
    /// load of their own — so a load that never settles (a token provider that
    /// blocks, a transport that ignores the request timeout) parks that key for
    /// the life of the client. Go reaches the same place from the other side:
    /// its loader runs under the owning caller's context and dies with it.
    private final class Claim: @unchecked Sendable {
        var waiters: [Waiter] = []
        var task: Task<Void, Never>?
    }

    /// One suspended caller.
    ///
    /// A reference type so that the cancellation handler and the registration
    /// can name the same waiter without a registry to clean up: whichever runs
    /// first leaves its mark here, the other reads it, and the whole thing dies
    /// with the call. An earlier shape kept a set of cancelled ids on the actor
    /// instead, which grew by one entry for every cancel that raced a
    /// completion and was never swept — the unbounded growth this cache's entry
    /// bound exists to prevent, reintroduced beside it.
    ///
    /// Every member is read and written on the actor and nowhere else, which is
    /// what the unchecked conformance stands on.
    private final class Waiter: @unchecked Sendable {
        var continuation: CheckedContinuation<Hit, any Error>?
        var cancelled = false
    }

    private let clock: @Sendable () -> TimeInterval
    private let ttl: TimeInterval
    private let refreshFloor: TimeInterval
    private let maxItems: Int

    private var entries: [Key: Entry] = [:]
    /// Keys with a load in progress, and the callers waiting on each. A waiter
    /// is a continuation rather than an `await` on the load task, so cancelling
    /// one caller does not mean waiting out the read (see the type comment).
    private var waiting: [Key: Claim] = [:]
    private var sequence: UInt64 = 0

    init(
        clock: @escaping @Sendable () -> TimeInterval, ttl: TimeInterval,
        refreshFloor: TimeInterval, maxItems: Int
    ) {
        self.clock = clock
        self.ttl = ttl
        self.refreshFloor = refreshFloor
        self.maxItems = maxItems
    }

    /// Returns the value for `key`, loading it when absent or older than the
    /// TTL — or, with `refresh` set, older than the floor.
    func value(
        for key: Key, refresh: Bool, load: @escaping @Sendable () async throws -> Value
    ) async throws -> Hit {
        try Task.checkCancellation()

        if let entry = entries[key] {
            let age = clock() - entry.fetchedAt
            if age < ttl, !refresh || age < refreshFloor {
                return Hit(value: entry.value, fetchedAt: entry.fetchedAt, cached: true)
            }
        }
        // The load this call waited on is this call's load: its value is handed
        // over as fresh, not as something that predated the call.
        return try await waitForLoad(of: key, load: load)
    }

    /// Suspends until the load for `key` publishes — or until this caller is
    /// cancelled, whichever comes first.
    private func waitForLoad(
        of key: Key, load: @escaping @Sendable () async throws -> Value
    ) async throws -> Hit {
        let waiter = Waiter()
        return try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                register(waiter, continuation, on: key, load: load)
            }
        } onCancel: {
            Task { await self.stopWaiting(waiter, on: key) }
        }
    }

    /// Registers one waiter, claiming the key and starting the load when this is
    /// the first caller to arrive.
    ///
    /// Claiming, registering and starting all happen here, in one synchronous
    /// actor-isolated step. That is what makes the wait safe: there is no
    /// suspension between them for a load to publish into, so a waiter can never
    /// register against a key whose load has already finished, and a key can
    /// never be claimed without a load being started behind it.
    private func register(
        _ waiter: Waiter, _ continuation: CheckedContinuation<Hit, any Error>, on key: Key,
        load: @escaping @Sendable () async throws -> Value
    ) {
        guard !waiter.cancelled else {
            // Cancelled before this ran. It never claimed the key, so there is
            // nothing to release and nobody else to disturb.
            continuation.resume(throwing: CancellationError())
            return
        }
        waiter.continuation = continuation

        let claim: Claim
        if let existing = waiting[key] {
            claim = existing
        } else {
            claim = Claim()
            waiting[key] = claim
        }
        claim.waiters.append(waiter)
        guard claim.task == nil else { return }

        // Unstructured on purpose: the load must outlive any ONE caller — but
        // not all of them, which is what `stopWaiting` cancels it for.
        // `Task.init` inherits this actor's isolation, so `publish` and `finish`
        // run under the actor without a second hop; awaiting the loader suspends
        // the task, not the actor. Nothing between the load returning and the
        // key being released suspends, so a cancellation cannot land in it.
        claim.task = Task {
            do {
                let hit = self.publish(key, try await load())
                self.finish(key, .success(hit))
            } catch {
                self.finish(key, .failure(error))
            }
        }
    }

    private func stopWaiting(_ waiter: Waiter, on key: Key) {
        waiter.cancelled = true
        // No continuation means one of two things, and neither needs anything
        // more than the flag above: this waiter has not registered yet (the
        // registration will read the flag and refuse), or the load already
        // resumed it (nothing left to cancel).
        guard let continuation = waiter.continuation else { return }
        waiter.continuation = nil
        if let claim = waiting[key] {
            claim.waiters.removeAll { $0 === waiter }
            // Nobody is waiting for this load any more, so it is abandoned. The
            // key stays claimed until the cancelled load returns and `finish`
            // releases it, which is as prompt as cancellation can be — and is
            // the same dependence on a cooperative loader that Go's
            // context-bound load has.
            if claim.waiters.isEmpty { claim.task?.cancel() }
        }
        continuation.resume(throwing: CancellationError())
    }

    /// Hands one load's outcome to everyone waiting on it, and releases the key.
    private func finish(_ key: Key, _ outcome: Result<Hit, any Error>) {
        for waiter in waiting.removeValue(forKey: key)?.waiters ?? [] {
            guard let continuation = waiter.continuation else { continue }
            waiter.continuation = nil
            continuation.resume(with: outcome)
        }
    }

    /// Returns the cached value for `key` when one is within the TTL, without
    /// loading.
    func cached(_ key: Key) -> Hit? {
        guard let entry = entries[key], clock() - entry.fetchedAt < ttl else { return nil }
        return Hit(value: entry.value, fetchedAt: entry.fetchedAt, cached: true)
    }

    /// Entries currently held. Test seam for the bound and the sweep.
    var count: Int { entries.count }

    /// Callers currently suspended on a load of `key`. Test seam: it is what
    /// lets a single-flight test wait for both callers to have ARRIVED rather
    /// than guess at it with a sleep.
    func waiterCount(for key: Key) -> Int { waiting[key]?.waiters.count ?? 0 }

    /// Whether `key` is claimed at all — a load is running behind it, waiters or
    /// no waiters. Test seam for the abandonment path.
    func isClaimed(_ key: Key) -> Bool { waiting[key] != nil }

    private func publish(_ key: Key, _ value: Value) -> Hit {
        sweep()
        makeRoom(for: key)
        sequence += 1
        let fetchedAt = clock()
        entries[key] = Entry(value: value, fetchedAt: fetchedAt, sequence: sequence)
        return Hit(value: value, fetchedAt: fetchedAt, cached: false)
    }

    /// Drops every entry past its TTL. It runs at each publication — the one
    /// moment the cache does work proportional to a miss anyway — so a
    /// long-lived client that has seen many buckets keeps a snapshot for at most
    /// a TTL past its last use plus the interval to the next load on any key,
    /// rather than for its lifetime.
    private func sweep() {
        let now = clock()
        entries = entries.filter { now - $0.value.fetchedAt < ttl }
    }

    /// Evicts the oldest-fetched entries until the one about to be stored for
    /// `key` fits under `maxItems` — oldest by fetch time, and by publication
    /// order among equals. Oldest-first is also the right order: the entry
    /// nearest its TTL is the one least worth keeping.
    private func makeRoom(for key: Key) {
        guard maxItems > 0, entries[key] == nil else { return }  // an overwrite takes no new room
        while entries.count >= maxItems {
            guard
                let oldest = entries.min(by: { lhs, rhs in
                    lhs.value.fetchedAt == rhs.value.fetchedAt
                        ? lhs.value.sequence < rhs.value.sequence
                        : lhs.value.fetchedAt < rhs.value.fetchedAt
                })
            else { return }
            entries.removeValue(forKey: oldest.key)
        }
    }
}
