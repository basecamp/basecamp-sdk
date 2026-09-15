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

    init(clock: @escaping @Sendable () -> Date = { Date() }) {
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
        var fetchedAt: Date
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
/// cancellation of whichever caller happened to start it. That is the one place
/// this diverges from the Go cache it is ported from, and it is a
/// simplification rather than a gap: Go's loader runs under the loading
/// caller's `context`, so that cache has to decide whether a failure belongs to
/// the load or to the caller who owned it, and let a still-live waiter try
/// again. Here the load belongs to no caller, so every waiter shares its one
/// outcome and there is no owner-attributed failure to re-run. A caller that
/// goes away leaves the load to finish for whoever else is waiting — one short
/// read, and the snapshot it publishes is the one the next call would have paid
/// for anyway.
actor TTLCache<Key: Hashable & Sendable, Value: Sendable> {
    /// What a cache read hands back: the value, when it was fetched, and whether
    /// it predated the call (as opposed to being loaded during it, by this
    /// caller or by one it waited on). The fetch time is what lets a caller tell
    /// a snapshot it already consulted from a newer one, whoever loaded it.
    struct Hit: Sendable {
        let value: Value
        let fetchedAt: Date
        let cached: Bool
    }

    private struct Entry {
        let value: Value
        let fetchedAt: Date
        /// Publication order: the tie-breaker when fetch times are equal, so
        /// eviction is total rather than whatever dictionary order visits first.
        let sequence: UInt64
    }

    private let clock: @Sendable () -> Date
    private let ttl: TimeInterval
    private let refreshFloor: TimeInterval
    private let maxItems: Int

    private var entries: [Key: Entry] = [:]
    private var inFlight: [Key: Task<Hit, any Error>] = [:]
    private var sequence: UInt64 = 0

    init(
        clock: @escaping @Sendable () -> Date, ttl: TimeInterval, refreshFloor: TimeInterval,
        maxItems: Int
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
            let age = clock().timeIntervalSince(entry.fetchedAt)
            if age < ttl, !refresh || age < refreshFloor {
                return Hit(value: entry.value, fetchedAt: entry.fetchedAt, cached: true)
            }
        }
        if let pending = inFlight[key] {
            // The load this call waited on is this call's load: its value is
            // handed over as fresh, not as something that predated the call.
            return try await pending.value
        }

        // Task.init inherits this actor's isolation, so `publish` and the
        // in-flight bookkeeping below run under the actor without a second hop;
        // awaiting the loader suspends the task, not the actor.
        let task = Task<Hit, any Error> {
            do {
                return self.publish(key, try await load())
            } catch {
                self.inFlight[key] = nil
                throw error
            }
        }
        inFlight[key] = task
        return try await task.value
    }

    /// Returns the cached value for `key` when one is within the TTL, without
    /// loading.
    func cached(_ key: Key) -> Hit? {
        guard let entry = entries[key], clock().timeIntervalSince(entry.fetchedAt) < ttl else {
            return nil
        }
        return Hit(value: entry.value, fetchedAt: entry.fetchedAt, cached: true)
    }

    /// Entries currently held. Test seam for the bound and the sweep.
    var count: Int { entries.count }

    private func publish(_ key: Key, _ value: Value) -> Hit {
        inFlight[key] = nil
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
        entries = entries.filter { now.timeIntervalSince($0.value.fetchedAt) < ttl }
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
