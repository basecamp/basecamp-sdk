package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.PaginationOptions
import com.basecamp.sdk.generated.campfires
import com.basecamp.sdk.generated.projects
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.isActive
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlin.coroutines.cancellation.CancellationException
import kotlin.time.TimeSource

/**
 * How long a cached discovery source — a bucket's project dock, the account's
 * Campfire listing — is reused before it is read again.
 */
const val CAMPFIRE_INDEX_TTL_MILLIS: Long = 10 * 60 * 1000

/**
 * Bounds the refresh-on-miss: a line found under no candidate re-reads the
 * cached sources, but not more often than this per source, so a run of
 * unresolvable lines cannot turn into a listing per line.
 * [BasecampException.RecordingSummaryFailure.refreshed] says whether the floor
 * applied.
 */
internal const val CAMPFIRE_INDEX_MIN_REFRESH_MILLIS: Long = 30 * 1000

/**
 * Bounds how many Campfires one `summarize` call tries, across both sources and
 * the refresh. A project has one Campfire and a handful of pings; a bucket past
 * this bound is not a shape BC3 produces, and the call reports
 * `campfire_discovery_incomplete` rather than calling the rest absent.
 */
const val MAX_CAMPFIRE_CANDIDATES: Int = 50

/**
 * Caps the account-wide Campfire listing the fallback source reads. A listing
 * that overflows it is not cached and the call reports
 * `campfire_discovery_incomplete`: the dock covers every project, so the listing
 * only ever serves the leftover, and an account with more Campfires than this
 * should not pay a full walk per ten minutes for it.
 */
const val MAX_CAMPFIRE_LISTING: Int = 1000

/**
 * Bounds each discovery cache's entry count. The dock cache holds one snapshot
 * per bucket consulted within the TTL: a connector listening across every
 * project an agent can see touches hundreds of buckets, not thousands, and a
 * snapshot is a handful of ids, so 1024 is generous headroom at a few hundred
 * KB, and the bound exists so that a process alive for weeks can never grow past
 * it whatever it sees. When the bound is reached the oldest-fetched entries go
 * first, deterministically.
 */
private const val CAMPFIRE_INDEX_MAX_ITEMS = 1024

/**
 * The cache's clock: milliseconds since an arbitrary fixed origin, read from a
 * MONOTONIC source rather than the wall clock.
 *
 * Go gets this for free — a `time.Time` from `time.Now()` carries a monotonic
 * reading and `Sub` uses it — and the cache depends on it in three places that
 * all fail quietly under a backwards wall-clock step: an entry whose computed
 * age goes negative outlives its TTL and survives the sweep, a refresh is
 * declined below a floor it has actually passed, and the fetched-time comparison
 * that decides `refreshed` (and therefore the stale-candidate report) reads the
 * wrong way. None of those raise; they just answer wrongly.
 */
internal fun monotonicMillis(): Long = MONOTONIC_ORIGIN.elapsedNow().inWholeMilliseconds

private val MONOTONIC_ORIGIN = TimeSource.Monotonic.markNow()

/**
 * What a cache read hands back: the value, when it was fetched, and whether it
 * predated the call (as opposed to being loaded during it, by this caller or by
 * one it waited on). The fetch time is what lets a caller tell a snapshot it
 * already consulted from a newer one, whoever loaded it.
 */
internal class TtlHit<V>(val value: V, val fetched: Long, val cached: Boolean)

/**
 * A per-key cache with single-flight loading: concurrent callers for one key
 * wait on the one load in progress rather than loading again, a failed load
 * leaves the previous value in place, and a refresh is honoured only once the
 * value is older than a floor.
 */
internal class TtlCache<K, V>(
    private val now: () -> Long,
    private val ttlMillis: Long,
    private val floorMillis: Long,
    private val maxItems: Int,
) {
    private class Entry<V>(val value: V, val fetched: Long, val seq: Long)

    private class Load<V> {
        val done = CompletableDeferred<V>()

        /** Set at publication, so the loader's own hit reports a real fetch time. */
        var fetched: Long = 0

        /**
         * Records that the failure is attributable to the loading caller's own
         * cancellation: its job was no longer active when the load ended AND the
         * failure is a cancellation.
         *
         * Both halves are needed, and the second one is why: Ktor's
         * `HttpRequestTimeoutException` IS a `CancellationException` and arrives
         * with the owner's job still active. Classifying on the type alone would
         * make that the owner's cancellation and turn one timed-out request into
         * one request per waiter.
         *
         * It is a PROXY for Go's `callerDone`, not a translation of it. Go asks
         * whether the error is the context's own (`errors.Is(err, ctxErr)`);
         * this asks whether the job is still active, which answers the same
         * question everywhere except one race — a transport timeout landing in
         * the same instant as an unrelated cancellation of the owner reads as
         * owner-attributed here and as shared in Go. The cost is bounded to one
         * extra load by the `reacquired` latch, and Go's own comment concedes an
         * overlap it cannot separate either.
         */
        var ownerCancelled: Boolean = false
    }

    private val mutex = Mutex()
    private val entries = mutableMapOf<K, Entry<V>>()
    private val inflight = mutableMapOf<K, Load<V>>()

    /** Publications so far; stamps entries so eviction order is total. */
    private var seq: Long = 0

    /**
     * Returns the value for [key], loading it when absent or older than the TTL
     * — or, with [refresh] set, older than the floor.
     */
    suspend fun get(key: K, refresh: Boolean, load: suspend () -> V): TtlHit<V> {
        var reacquired = false
        while (true) {
            // A cancelled caller gets no answer, cached or otherwise: the mutex
            // below may be free, and a fresh hit would otherwise let a cancelled
            // call run to a verdict without ever suspending.
            currentCoroutineContext().ensureActive()
            var fresh: TtlHit<V>? = null
            var pending: Load<V>? = null
            var owned: Load<V>? = null
            mutex.withLock {
                val entry = entries[key]
                if (entry != null) {
                    val age = now() - entry.fetched
                    if (age < ttlMillis && (!refresh || age < floorMillis)) {
                        fresh = TtlHit(entry.value, entry.fetched, cached = true)
                    }
                }
                if (fresh == null) {
                    val existing = inflight[key]
                    if (existing != null) {
                        pending = existing
                    } else {
                        val load0 = Load<V>()
                        inflight[key] = load0
                        owned = load0
                    }
                }
            }
            fresh?.let { return it }

            val waited = pending
            if (waited != null) {
                val value = try {
                    waited.done.await()
                } catch (e: CancellationException) {
                    // This caller's OWN cancellation is its own to throw, and it
                    // takes precedence over anything the load did.
                    currentCoroutineContext().ensureActive()
                    // The load ran under the loading caller's coroutine. If that
                    // caller was cancelled, the failure is its, not this one's: a
                    // waiter whose own job is live goes round again and loads for
                    // itself (the key is free, so it becomes the loader and any
                    // other waiters queue behind it — one load, not a stampede).
                    // Once only, so a run of cancelled owners cannot become a
                    // queue of sequential loads behind one waiter.
                    if (waited.ownerCancelled && !reacquired) {
                        reacquired = true
                        continue
                    }
                    if (waited.ownerCancelled) {
                        // Past the one retry, and the failure belongs to a job
                        // that is not this one. Relaying a foreign
                        // CancellationException would end THIS live coroutine as
                        // "cancelled" and do it silently: JobSupport.childCancelled
                        // short-circuits on a CancellationException, so no
                        // CoroutineExceptionHandler ever runs and a `launch { }`
                        // caller loses the failure entirely. Go hands its waiter an
                        // ordinary error value; this is the nearest thing Kotlin
                        // has to that.
                        throw BasecampException.Api(
                            "the campfire discovery read this call was waiting on was cancelled by the caller that started it",
                            httpStatus = null,
                            hint = "retry; another caller's cancellation is not this one's failure",
                            cause = e,
                        )
                    }
                    // Anything else is the load's OWN failure and is shared —
                    // a transport timeout included, which Ktor spells as a
                    // CancellationException. The owner sees that same shape
                    // natively, so a waiter seeing it too is the honest answer.
                    throw e
                }
                // The load this call waited on is this call's load: hand its value
                // over as fresh, not as something that predated the call — read off
                // the load record, so a sweep or the bound evicting the entry in the
                // meantime cannot take it from this waiter.
                return TtlHit(value, waited.fetched, cached = false)
            }

            val load0 = requireNotNull(owned)
            // Every exit from here releases the key. `publishSuccess` is INSIDE
            // the try, so a failure in publication itself lands in the catch and
            // republishes as a failure rather than leaking the slot; and the
            // catch is on Throwable, so an Error and a cancellation are covered
            // too. That totality is what Go gets from its deferred publish, and
            // it is load-bearing rather than tidy: waiters wait with no timeout,
            // so a slot never released parks every later caller for that key for
            // the life of the client — and the listing's key is the bare account
            // id, which makes that an account rather than a bucket.
            //
            // A `finally` would add nothing reachable on top of that. The only
            // ways past a Throwable catch are a throw from inside the catch
            // itself, and a coroutine that is never resumed at all — and a
            // coroutine that is never resumed does not run its `finally` either,
            // so the guard could not help where it would be needed. Go's
            // goroutine has the identical hole.
            try {
                val value = load()
                publishSuccess(key, load0, value)
                return TtlHit(value, load0.fetched, cached = false)
            } catch (t: Throwable) {
                load0.ownerCancelled = t is CancellationException && !currentCoroutineContext().isActive
                publishFailure(key, load0, t)
                throw t
            }
        }
    }

    /**
     * Returns the cached value for [key] when one is within the TTL, without
     * loading. It lets a caller consult what a source already holds before
     * deciding whether to pay for a fetch of it.
     */
    suspend fun peek(key: K): TtlHit<V>? = mutex.withLock {
        val entry = entries[key] ?: return@withLock null
        if (now() - entry.fetched >= ttlMillis) return@withLock null
        TtlHit(entry.value, entry.fetched, cached = true)
    }

    /**
     * Publication is NonCancellable, on both paths and for the same reason Go
     * publishes from a `defer`: it always runs. A cancellation landing between
     * the loader returning and the key being released would otherwise throw out
     * of `withLock`, leaving the key in flight and its waiters suspended on a
     * deferred nobody will ever complete — a key poisoned for the life of the
     * client. The failure path needs it most, since it is reached AFTER a
     * cancellation.
     */
    private suspend fun publishSuccess(key: K, load: Load<V>, value: V) = withContext(NonCancellable) {
        mutex.withLock {
            releaseLocked(key, load)
            load.fetched = now()
            sweepLocked()
            makeRoomLocked(key)
            seq++
            entries[key] = Entry(value, load.fetched, seq)
        }
        load.done.complete(value)
    }

    private suspend fun publishFailure(key: K, load: Load<V>, cause: Throwable) = withContext(NonCancellable) {
        // The key is released so a later caller can load again; the previous
        // value, if any, stays in place.
        mutex.withLock { releaseLocked(key, load) }
        load.done.completeExceptionally(cause)
    }

    /**
     * Releases the key BY IDENTITY: the slot is cleared only when the record
     * sitting in it is the one being published.
     *
     * A publisher that removed whatever it found would drop a LATER caller's
     * single-flight guarantee while that caller was still loading — two
     * concurrent loads for one key, and the listing's key is the bare account
     * id, so that is an account-wide duplicate rather than a bucket's.
     *
     * That is not reachable as this cache is written today: a second caller
     * finding the slot occupied becomes a waiter rather than a registrant, and
     * every publish clears the slot BEFORE completing the deferred, so no waiter
     * can wake and re-register ahead of its own publisher. The guard is here
     * anyway because that argument is global and this one is local — it holds by
     * reading four lines rather than by reasoning about every path that could
     * ever publish. Re-introduce an abandonment path and the invariant still
     * stands instead of silently going with it.
     */
    private fun releaseLocked(key: K, load: Load<V>) {
        if (inflight[key] === load) inflight.remove(key)
    }

    /**
     * Drops every entry past its TTL. It runs at each publication — the one
     * moment the cache does work proportional to a miss anyway — so a long-lived
     * client that has seen many buckets keeps a snapshot for at most a TTL past
     * its last use plus the interval to the next load on any key, rather than for
     * its lifetime. Caller holds [mutex].
     */
    private fun sweepLocked() {
        val cutoff = now()
        entries.entries.removeAll { cutoff - it.value.fetched >= ttlMillis }
    }

    /**
     * Evicts the oldest-fetched entries until the one about to be stored for
     * [key] fits under [maxItems] — oldest by fetch time, and by publication
     * order among equals, so the choice is total rather than whatever map
     * iteration happens to visit first. Oldest-first is also the right order: the
     * entry nearest its TTL is the one least worth keeping. Caller holds [mutex].
     */
    private fun makeRoomLocked(key: K) {
        if (maxItems <= 0) return
        if (entries.containsKey(key)) return // an overwrite takes no new room
        while (entries.size >= maxItems) {
            val oldest = entries.entries.minWithOrNull(
                compareBy({ it.value.fetched }, { it.value.seq }),
            ) ?: return
            entries.remove(oldest.key)
        }
    }
}

/** A dock cache key: an account's view of one bucket. */
internal data class CampfireBucketKey(val accountId: String, val bucketId: Long)

/**
 * One consultation of a discovery source: the candidate ids it holds for the
 * bucket, when that snapshot was fetched, and whether it predated the call.
 */
internal class SourceRead(val ids: List<Long>, val fetched: Long, val cached: Boolean)

/**
 * The load failure a Campfire listing past its cap raises;
 * `RecordingsService.summarize` turns it into the typed
 * `campfire_discovery_incomplete`.
 */
internal class CampfireListingOverflow :
    Exception("campfire listing exceeds MAX_CAMPFIRE_LISTING ($MAX_CAMPFIRE_LISTING)")

/**
 * Campfire discovery for chat lines: the two sources, each cached
 * [CAMPFIRE_INDEX_TTL_MILLIS].
 *
 * A `chat.line.created` row carries the line's id and bucket, not its Campfire,
 * and the line read is `/chats/{campfireId}/lines/{lineId}`. Candidates come
 * from two sources, tried in order:
 *
 *  1. The bucket's project dock, whose `chat` tool is the project's Campfire:
 *     one project read per bucket, and the answer for every line posted in a
 *     project. Cached per bucket.
 *  2. The account-wide Campfire listing (BC3 has no per-bucket one), filtered to
 *     the bucket, for buckets that are not projects or whose dock did not hold
 *     the line. Cached per account, so a burst of lines costs one listing.
 *
 * It lives on the client (shared by every [AccountClient] the client hands out);
 * a client is bound to one credential, so entries are never shared across
 * authorization contexts, and every key carries the account id. Expired
 * snapshots are swept at each load and each cache is bounded
 * ([CAMPFIRE_INDEX_MAX_ITEMS], oldest out first), so the index holds at most the
 * buckets and accounts consulted within the last TTL, and never more than the
 * bound, whatever the client has seen.
 */
internal class CampfireIndex(now: () -> Long = ::monotonicMillis) {
    private val docks = TtlCache<CampfireBucketKey, List<Long>>(
        now,
        CAMPFIRE_INDEX_TTL_MILLIS,
        CAMPFIRE_INDEX_MIN_REFRESH_MILLIS,
        CAMPFIRE_INDEX_MAX_ITEMS,
    )
    private val listings = TtlCache<String, Map<Long, List<Long>>>(
        now,
        CAMPFIRE_INDEX_TTL_MILLIS,
        CAMPFIRE_INDEX_MIN_REFRESH_MILLIS,
        CAMPFIRE_INDEX_MAX_ITEMS,
    )

    /**
     * The Campfire ids a bucket's project dock names. A bucket that is not a
     * project (a 404 on the project read) has none; any other failure of the read
     * is raised.
     */
    suspend fun dockCampfires(account: AccountClient, bucketId: Long, refresh: Boolean): SourceRead {
        val key = CampfireBucketKey(account.accountId, bucketId)
        val hit = docks.get(key, refresh) {
            val project = try {
                account.projects.get(bucketId)
            } catch (e: BasecampException.NotFound) {
                null
            }
            project?.dock.orEmpty().filter { it.name == "chat" && it.id != 0L }.map { it.id }
        }
        return SourceRead(hit.value, hit.fetched, hit.cached)
    }

    /**
     * The Campfire ids the cached account-wide listing shows in a bucket, without
     * fetching: null when the listing is not cached or has expired.
     */
    suspend fun cachedListedCampfires(accountId: String, bucketId: Long): SourceRead? {
        val hit = listings.peek(accountId) ?: return null
        return SourceRead(hit.value[bucketId].orEmpty(), hit.fetched, cached = true)
    }

    /**
     * The Campfire ids the account-wide listing shows in a bucket. A listing that
     * overflows [MAX_CAMPFIRE_LISTING] is not cached and raises
     * [CampfireListingOverflow], which the caller reports as incomplete
     * discovery.
     */
    suspend fun listedCampfires(account: AccountClient, bucketId: Long, refresh: Boolean): SourceRead {
        val hit = listings.get(account.accountId, refresh) {
            val listed = account.campfires.list(PaginationOptions(maxItems = MAX_CAMPFIRE_LISTING))
            if (listed.meta.truncated) throw CampfireListingOverflow()
            listed.filter { it.bucket.id != 0L }.groupBy({ it.bucket.id }, { it.id })
        }
        return SourceRead(hit.value[bucketId].orEmpty(), hit.fetched, hit.cached)
    }
}
