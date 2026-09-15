//! Campfire discovery for chat lines: the two sources
//! [`RecordingsService::summarize`](crate::services::recordings::RecordingsService::summarize)
//! consults to find the Campfire a line lives in, and the bounded, TTL'd cache they are
//! read through.
//!
//! A `chat.line.created` row carries the line's id and bucket, not its Campfire, and the
//! line read is `GET /chats/{campfireId}/lines/{lineId}`. Candidates come from two sources,
//! tried in order and each cached [`CAMPFIRE_INDEX_TTL`]:
//!
//! 1. The bucket's project dock, whose `chat` tool is the project's Campfire: one project
//!    read per bucket, and the answer for every line posted in a project. Cached per bucket.
//! 2. The account-wide Campfire listing (BC3 has no per-bucket one), filtered to the
//!    bucket, for buckets that are not projects or whose dock did not hold the line. Cached
//!    per account, so a burst of lines costs one listing.
//!
//! The index lives on the client every [`AccountClient`] is derived from, and a client is
//! bound to one credential, so entries are never shared across authorization contexts —
//! and every key carries the account id besides. Expired snapshots are swept at each load
//! and each cache is bounded (a fixed entry count, oldest out first), so the index
//! holds at most the buckets and accounts consulted within the last TTL, and never more
//! than the bound, whatever the client has seen.

use std::collections::HashMap;
use std::future::Future;
use std::hash::Hash;
use std::panic::AssertUnwindSafe;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use futures_util::FutureExt;
use futures_util::future::{BoxFuture, Shared};

use crate::client::AccountClient;
use crate::error::{Error, ErrorCode};
use crate::generated::services::campfires::ListCampfiresParams;
use crate::generated::types::DockItem;

/// How long a cached discovery source — a bucket's project dock, the account's Campfire
/// listing — is reused before it is read again.
pub const CAMPFIRE_INDEX_TTL: Duration = Duration::from_secs(10 * 60);

/// Bounds the refresh-on-miss: a line found under no candidate re-reads the cached sources,
/// but not more often than this per source, so a run of unresolvable lines cannot turn into
/// a listing per line.
/// [`UnresolvedRecording::refreshed`](crate::services::recordings::UnresolvedRecording::refreshed)
/// says whether the floor applied.
pub const CAMPFIRE_INDEX_MIN_REFRESH: Duration = Duration::from_secs(30);

/// How many Campfires one
/// [`summarize`](crate::services::recordings::RecordingsService::summarize) call tries,
/// across both sources and the refresh. A project has one Campfire and a handful of pings; a
/// bucket past this bound is not a shape BC3 produces, and the call reports
/// [`RecordingSummaryError::CampfireDiscoveryIncomplete`](crate::services::recordings::RecordingSummaryError::CampfireDiscoveryIncomplete)
/// rather than calling the rest absent.
pub const MAX_CAMPFIRE_CANDIDATES: usize = 50;

/// Caps the account-wide Campfire listing the fallback source reads. A listing that
/// overflows it is not cached and the call reports incomplete discovery: the dock covers
/// every project, so the listing only ever serves the leftover, and an account with more
/// Campfires than this should not pay a full walk per ten minutes for it.
pub const MAX_CAMPFIRE_LISTING: usize = 1000;

/// Bounds each discovery cache's entry count. The dock cache holds one snapshot per bucket
/// consulted within the TTL: a connector listening across every project an agent can see
/// touches hundreds of buckets, not thousands, and a snapshot is a handful of ids, so 1024
/// is generous headroom at a few hundred KB, and the bound exists so that a process alive
/// for weeks can never grow past it whatever it sees. When the bound is reached the
/// oldest-fetched entries go first, deterministically.
const CAMPFIRE_INDEX_MAX_ITEMS: usize = 1024;

/// The dock tool whose id is the project's Campfire.
const CHAT_DOCK_TOOL: &str = "chat";

/// A reader of the current instant, so a test can move time without sleeping.
pub(crate) type Clock = Arc<dyn Fn() -> Instant + Send + Sync>;

/// What a cache read hands back: the value, when it was fetched, and whether it predated
/// the call (as opposed to being loaded during it, by this caller or by one it waited on).
/// The fetch time is what lets a caller tell a snapshot it already consulted from a newer
/// one, whoever loaded it.
pub(crate) struct Hit<V> {
    pub(crate) value: Arc<V>,
    pub(crate) fetched: Instant,
    pub(crate) cached: bool,
}

struct Entry<V> {
    value: Arc<V>,
    fetched: Instant,
    /// Publication order: the tie-breaker when fetch times are equal, so eviction is total
    /// rather than whatever the map happens to visit first.
    seq: u64,
}

type Loaded<V> = Result<(Arc<V>, Instant), Arc<Error>>;
type Pending<V> = Shared<BoxFuture<'static, Loaded<V>>>;

struct CacheState<K, V> {
    seq: u64,
    entries: HashMap<K, Entry<V>>,
    inflight: HashMap<K, Pending<V>>,
}

/// A per-key cache with single-flight loading: concurrent callers for one key wait on the
/// one load in progress rather than loading again, a failed load leaves the previous value
/// in place, and a refresh is honoured only once the value is older than a floor.
///
/// The load runs as a shared future, so a caller dropped mid-load — an operation deadline
/// shorter than the listing's round trip — leaves the load for whoever is still waiting on
/// it rather than abandoning the key in flight.
pub(crate) struct TtlCache<K, V> {
    state: Mutex<CacheState<K, V>>,
    clock: Clock,
    ttl: Duration,
    floor: Duration,
    max_items: usize,
}

impl<K, V> TtlCache<K, V>
where
    K: Eq + Hash + Clone + Send + Sync + 'static,
    V: Send + Sync + 'static,
{
    fn new(clock: Clock, ttl: Duration, floor: Duration, max_items: usize) -> TtlCache<K, V> {
        TtlCache {
            state: Mutex::new(CacheState {
                seq: 0,
                entries: HashMap::new(),
                inflight: HashMap::new(),
            }),
            clock,
            ttl,
            floor,
            max_items,
        }
    }

    fn lock(&self) -> std::sync::MutexGuard<'_, CacheState<K, V>> {
        self.state
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner)
    }

    /// The value for `key`, loading it when absent or older than the TTL — or, with
    /// `refresh` set, older than the floor.
    pub(crate) async fn get<F, Fut>(
        self: &Arc<Self>,
        key: &K,
        refresh: bool,
        load: F,
    ) -> Result<Hit<V>, Error>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<V, Error>> + Send + 'static,
    {
        let pending = {
            let mut state = self.lock();
            if let Some(entry) = state.entries.get(key) {
                let age = (self.clock)().saturating_duration_since(entry.fetched);
                if age < self.ttl && (!refresh || age < self.floor) {
                    return Ok(Hit {
                        value: Arc::clone(&entry.value),
                        fetched: entry.fetched,
                        cached: true,
                    });
                }
            }
            if let Some(pending) = state.inflight.get(key) {
                pending.clone()
            } else {
                let cache = Arc::clone(self);
                let owned = key.clone();
                let loading = load();
                let pending = async move {
                    // A loader that panics — a hook, a token provider — still releases the
                    // key and wakes its waiters with an error; without that the key would
                    // stay in flight forever and every later caller would wait on it.
                    let outcome = match AssertUnwindSafe(loading).catch_unwind().await {
                        Ok(result) => result.map_err(Arc::new),
                        Err(_) => Err(Arc::new(Error::new(
                            ErrorCode::ApiError,
                            "campfire discovery source panicked while loading",
                        ))),
                    };
                    cache.publish(&owned, outcome)
                }
                .boxed()
                .shared();
                state.inflight.insert(key.clone(), pending.clone());
                pending
            }
        };
        match pending.await {
            // The load this call waited on is this call's load: its value is fresh, not
            // something that predated the call — and it is read off the load's own result,
            // so a sweep or the bound evicting the entry in the meantime cannot take it
            // from this waiter.
            Ok((value, fetched)) => Ok(Hit {
                value,
                fetched,
                cached: false,
            }),
            // One failed load is one failure, shared: N waiters never re-run it N times.
            // The projection carries everything a caller classifies on — code, hint,
            // status, retryability — with the shared original chained as the cause.
            Err(error) => Err(project(&error)),
        }
    }

    /// The cached value for `key` when one is within the TTL, without loading. It lets a
    /// caller consult what a source already holds before deciding whether to pay for a
    /// fetch of it.
    pub(crate) fn peek(&self, key: &K) -> Option<Hit<V>> {
        let state = self.lock();
        let entry = state.entries.get(key)?;
        ((self.clock)().saturating_duration_since(entry.fetched) < self.ttl).then(|| Hit {
            value: Arc::clone(&entry.value),
            fetched: entry.fetched,
            cached: true,
        })
    }

    fn publish(&self, key: &K, outcome: Result<V, Arc<Error>>) -> Loaded<V> {
        let mut state = self.lock();
        state.inflight.remove(key);
        // A failed load leaves the previous value in place: the next caller past the TTL
        // tries again rather than finding the key empty.
        let value = Arc::new(outcome?);
        let fetched = (self.clock)();
        self.sweep_locked(&mut state, fetched);
        self.make_room_locked(&mut state, key);
        state.seq += 1;
        let seq = state.seq;
        state.entries.insert(
            key.clone(),
            Entry {
                value: Arc::clone(&value),
                fetched,
                seq,
            },
        );
        Ok((value, fetched))
    }

    /// Drops every entry past its TTL. It runs at each publication — the one moment the
    /// cache does work proportional to a miss anyway — so a long-lived client that has seen
    /// many buckets keeps a snapshot for at most a TTL past its last use plus the interval
    /// to the next load on any key, rather than for its lifetime.
    fn sweep_locked(&self, state: &mut CacheState<K, V>, now: Instant) {
        state
            .entries
            .retain(|_, entry| now.saturating_duration_since(entry.fetched) < self.ttl);
    }

    /// Evicts the oldest-fetched entries until the one about to be stored for `key` fits
    /// under `max_items` — oldest by fetch time, and by publication order among equals, so
    /// the choice is total rather than whatever map iteration happens to visit first.
    /// Oldest-first is also the right order: the entry nearest its TTL is the one least
    /// worth keeping.
    fn make_room_locked(&self, state: &mut CacheState<K, V>, key: &K) {
        if self.max_items == 0 || state.entries.contains_key(key) {
            return; // an overwrite takes no new room
        }
        while state.entries.len() >= self.max_items {
            let oldest = state
                .entries
                .iter()
                .min_by_key(|(_, entry)| (entry.fetched, entry.seq))
                .map(|(key, _)| key.clone());
            match oldest {
                Some(oldest) => {
                    state.entries.remove(&oldest);
                }
                None => break,
            }
        }
    }
}

/// The key a dock snapshot is cached under: one bucket, in one account.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub(crate) struct BucketKey {
    pub(crate) account_id: String,
    pub(crate) bucket_id: i64,
}

/// One consultation of a discovery source: the candidate ids it holds for the bucket, when
/// that snapshot was fetched, and whether it predated the call.
pub(crate) struct SourceRead {
    pub(crate) ids: Vec<i64>,
    pub(crate) fetched: Instant,
    pub(crate) cached: bool,
}

/// The two discovery sources.
pub(crate) struct CampfireIndex {
    docks: Arc<TtlCache<BucketKey, Vec<i64>>>,
    listings: Arc<TtlCache<String, HashMap<i64, Vec<i64>>>>,
}

impl std::fmt::Debug for CampfireIndex {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("CampfireIndex").finish_non_exhaustive()
    }
}

impl CampfireIndex {
    pub(crate) fn new() -> CampfireIndex {
        CampfireIndex::with_clock(Arc::new(Instant::now))
    }

    pub(crate) fn with_clock(clock: Clock) -> CampfireIndex {
        CampfireIndex {
            docks: Arc::new(TtlCache::new(
                Arc::clone(&clock),
                CAMPFIRE_INDEX_TTL,
                CAMPFIRE_INDEX_MIN_REFRESH,
                CAMPFIRE_INDEX_MAX_ITEMS,
            )),
            listings: Arc::new(TtlCache::new(
                clock,
                CAMPFIRE_INDEX_TTL,
                CAMPFIRE_INDEX_MIN_REFRESH,
                CAMPFIRE_INDEX_MAX_ITEMS,
            )),
        }
    }

    /// The Campfire ids a bucket's project dock names. A bucket that is not a project (a
    /// 404 on the project read) has none; any other failure of the read is returned.
    pub(crate) async fn dock_campfires(
        &self,
        account: &AccountClient,
        bucket_id: i64,
        refresh: bool,
    ) -> Result<SourceRead, Error> {
        let key = BucketKey {
            account_id: account.account_id().to_string(),
            bucket_id,
        };
        let reader = account.clone();
        let hit = self
            .docks
            .get(&key, refresh, move || async move {
                match reader.projects().get(bucket_id).await {
                    Ok(project) => Ok(project
                        .dock
                        .unwrap_or_default()
                        .iter()
                        .filter(|item: &&DockItem| item.name == CHAT_DOCK_TOOL && item.id != 0)
                        .map(|item| item.id)
                        .collect()),
                    Err(error) if error.code() == ErrorCode::NotFound => Ok(Vec::new()),
                    Err(error) => Err(error),
                }
            })
            .await?;
        Ok(SourceRead {
            ids: hit.value.as_ref().clone(),
            fetched: hit.fetched,
            cached: hit.cached,
        })
    }

    /// The Campfire ids the cached account-wide listing shows in a bucket, without
    /// fetching: `None` when the listing is not cached or has expired.
    pub(crate) fn cached_listed_campfires(
        &self,
        account_id: &str,
        bucket_id: i64,
    ) -> Option<SourceRead> {
        let hit = self.listings.peek(&account_id.to_string())?;
        Some(SourceRead {
            ids: hit.value.get(&bucket_id).cloned().unwrap_or_default(),
            fetched: hit.fetched,
            cached: true,
        })
    }

    /// The Campfire ids the account-wide listing shows in a bucket. A listing that
    /// overflows [`MAX_CAMPFIRE_LISTING`] is not cached, and is reported by the caller as
    /// incomplete discovery rather than as an answer.
    pub(crate) async fn listed_campfires(
        &self,
        account: &AccountClient,
        bucket_id: i64,
        refresh: bool,
    ) -> Result<SourceRead, Error> {
        let key = account.account_id().to_string();
        let reader = account.clone();
        let hit = self
            .listings
            .get(&key, refresh, move || async move {
                let first = reader
                    .campfires()
                    .list(&ListCampfiresParams::default())
                    .await?;
                let listed = reader
                    .collect_all(first, Some(MAX_CAMPFIRE_LISTING))
                    .await?;
                if listed.meta.truncated {
                    return Err(listing_overflow());
                }
                let mut by_bucket: HashMap<i64, Vec<i64>> = HashMap::new();
                for campfire in listed.items {
                    if campfire.bucket.id == 0 {
                        continue;
                    }
                    by_bucket
                        .entry(campfire.bucket.id)
                        .or_default()
                        .push(campfire.id);
                }
                Ok(by_bucket)
            })
            .await?;
        Ok(SourceRead {
            ids: hit.value.get(&bucket_id).cloned().unwrap_or_default(),
            fetched: hit.fetched,
            cached: hit.cached,
        })
    }
}

/// The load error for a listing past its cap. The caller turns it into an incomplete
/// discovery verdict; it is recognized by its hint rather than by a code, since the
/// taxonomy (SPEC §6) has no member for "the SDK's own bound was reached".
const LISTING_OVERFLOW_HINT: &str = "campfire listing exceeds MAX_CAMPFIRE_LISTING";

/// One failed load, projected for each caller waiting on it: `Error` is not `Clone`, so the
/// shared original is chained as the cause and everything a caller classifies on is copied
/// onto the projection.
fn project(error: &Arc<Error>) -> Error {
    let mut projected = Error::new(error.code(), error.message()).retryable(error.is_retryable());
    if let Some(hint) = error.hint() {
        projected = projected.with_hint(hint);
    }
    if let Some(status) = error.http_status() {
        projected = projected.with_status(status);
    }
    if let Some(request_id) = error.request_id() {
        projected = projected.with_request_id(request_id);
    }
    if let Some(seconds) = error.retry_after() {
        projected = projected.with_retry_after(seconds);
    }
    projected.with_source(Arc::clone(error))
}

fn listing_overflow() -> Error {
    Error::new(
        ErrorCode::ApiError,
        "campfire listing is too large to cache",
    )
    .with_hint(LISTING_OVERFLOW_HINT)
}

/// Whether an error is the listing-overflow marker, through however many wrappers the cache
/// projected it into.
pub(crate) fn is_listing_overflow(error: &Error) -> bool {
    error.hint() == Some(LISTING_OVERFLOW_HINT)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicU64, Ordering};

    struct TestClock {
        base: Instant,
        offset: AtomicU64,
    }

    impl TestClock {
        fn new() -> Arc<TestClock> {
            Arc::new(TestClock {
                base: Instant::now(),
                offset: AtomicU64::new(0),
            })
        }

        fn clock(self: &Arc<Self>) -> Clock {
            let clock = Arc::clone(self);
            Arc::new(move || {
                clock.base + Duration::from_millis(clock.offset.load(Ordering::SeqCst))
            })
        }

        fn advance(&self, by: Duration) {
            let millis = u64::try_from(by.as_millis()).unwrap_or(u64::MAX);
            self.offset.fetch_add(millis, Ordering::SeqCst);
        }
    }

    fn cache<V: Send + Sync + 'static>(
        clock: &Arc<TestClock>,
        max_items: usize,
    ) -> Arc<TtlCache<i64, V>> {
        Arc::new(TtlCache::new(
            clock.clock(),
            CAMPFIRE_INDEX_TTL,
            CAMPFIRE_INDEX_MIN_REFRESH,
            max_items,
        ))
    }

    #[tokio::test]
    async fn a_value_inside_the_ttl_is_served_without_loading() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 16);
        let loads = Arc::new(AtomicU64::new(0));
        for _ in 0..3 {
            let counter = Arc::clone(&loads);
            let hit = cache
                .get(&1, false, move || async move {
                    counter.fetch_add(1, Ordering::SeqCst);
                    Ok(7)
                })
                .await
                .unwrap();
            assert_eq!(*hit.value, 7);
        }
        assert_eq!(loads.load(Ordering::SeqCst), 1);
    }

    #[tokio::test]
    async fn a_refresh_is_declined_under_the_floor_and_honoured_past_it() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 16);
        let loads = Arc::new(AtomicU64::new(0));
        let load = || {
            let counter = Arc::clone(&loads);
            move || async move {
                counter.fetch_add(1, Ordering::SeqCst);
                Ok(1)
            }
        };
        cache.get(&1, false, load()).await.unwrap();
        clock.advance(Duration::from_secs(5));
        let under = cache.get(&1, true, load()).await.unwrap();
        assert!(under.cached, "the floor declines the refresh");
        clock.advance(CAMPFIRE_INDEX_MIN_REFRESH);
        let past = cache.get(&1, true, load()).await.unwrap();
        assert!(!past.cached, "past the floor the source is re-read");
        assert_eq!(loads.load(Ordering::SeqCst), 2);
    }

    #[tokio::test]
    async fn a_value_past_the_ttl_is_loaded_again() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 16);
        cache.get(&1, false, || async { Ok(1) }).await.unwrap();
        clock.advance(CAMPFIRE_INDEX_TTL);
        assert!(cache.peek(&1).is_none());
        let hit = cache.get(&1, false, || async { Ok(2) }).await.unwrap();
        assert_eq!(*hit.value, 2);
    }

    #[tokio::test]
    async fn a_failed_load_leaves_the_previous_value_in_place() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 16);
        cache.get(&1, false, || async { Ok(1) }).await.unwrap();
        clock.advance(CAMPFIRE_INDEX_MIN_REFRESH);
        let failed = cache
            .get(&1, true, || async { Err(Error::usage("nope")) })
            .await;
        assert!(failed.is_err());
        let still_there = cache.peek(&1).expect("the old snapshot survives");
        assert_eq!(*still_there.value, 1);
    }

    #[tokio::test]
    async fn the_bound_evicts_the_oldest_fetched_entry_first() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 2);
        cache.get(&1, false, || async { Ok(1) }).await.unwrap();
        clock.advance(Duration::from_secs(1));
        cache.get(&2, false, || async { Ok(2) }).await.unwrap();
        clock.advance(Duration::from_secs(1));
        cache.get(&3, false, || async { Ok(3) }).await.unwrap();
        assert!(cache.peek(&1).is_none(), "the oldest went first");
        assert!(cache.peek(&2).is_some());
        assert!(cache.peek(&3).is_some());
    }

    #[tokio::test]
    async fn expired_entries_are_swept_at_the_next_publication() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 16);
        cache.get(&1, false, || async { Ok(1) }).await.unwrap();
        clock.advance(CAMPFIRE_INDEX_TTL);
        cache.get(&2, false, || async { Ok(2) }).await.unwrap();
        let state = cache.lock();
        assert_eq!(state.entries.len(), 1, "the expired key was swept");
        assert!(state.entries.contains_key(&2));
    }

    #[tokio::test]
    async fn concurrent_callers_share_one_load() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 16);
        let loads = Arc::new(AtomicU64::new(0));
        let (release, held) = tokio::sync::oneshot::channel::<()>();
        let first = {
            let counter = Arc::clone(&loads);
            cache.get(&1, false, move || async move {
                counter.fetch_add(1, Ordering::SeqCst);
                let _ = held.await;
                Ok(9)
            })
        };
        let second = {
            let counter = Arc::clone(&loads);
            cache.get(&1, false, move || async move {
                counter.fetch_add(1, Ordering::SeqCst);
                Ok(0)
            })
        };
        // `join!` polls in order: the first call takes the key and parks on the gate, the
        // second finds the load in flight and parks on it, and only then is it released.
        let releaser = async {
            tokio::task::yield_now().await;
            let _ = release.send(());
        };
        let (first, second, ()) = futures_util::join!(first, second, releaser);
        assert_eq!(*first.unwrap().value, 9);
        let second = second.unwrap();
        assert_eq!(*second.value, 9, "the waiter shares the loaded value");
        assert!(
            !second.cached,
            "and reads it as fresh, not as a prior snapshot"
        );
        assert_eq!(loads.load(Ordering::SeqCst), 1);
    }

    #[tokio::test]
    async fn one_failed_load_is_one_failure_shared_by_every_waiter() {
        let clock = TestClock::new();
        let cache = cache::<i64>(&clock, 16);
        let loads = Arc::new(AtomicU64::new(0));
        let (release, held) = tokio::sync::oneshot::channel::<()>();
        let first = {
            let counter = Arc::clone(&loads);
            cache.get(&1, false, move || async move {
                counter.fetch_add(1, Ordering::SeqCst);
                let _ = held.await;
                Err(Error::new(ErrorCode::Forbidden, "no").with_status(403))
            })
        };
        let second = {
            let counter = Arc::clone(&loads);
            cache.get(&1, false, move || async move {
                counter.fetch_add(1, Ordering::SeqCst);
                Ok(0)
            })
        };
        let releaser = async {
            tokio::task::yield_now().await;
            let _ = release.send(());
        };
        let (first, second, ()) = futures_util::join!(first, second, releaser);
        for outcome in [first, second] {
            let error = outcome.err().expect("the shared load failed");
            assert_eq!(error.code(), ErrorCode::Forbidden);
            assert_eq!(error.http_status(), Some(403));
        }
        assert_eq!(loads.load(Ordering::SeqCst), 1);
    }

    #[test]
    fn the_listing_overflow_marker_survives_the_cache_projection() {
        let overflow = listing_overflow();
        assert!(is_listing_overflow(&overflow));
        let projected = Error::new(overflow.code(), overflow.message())
            .with_hint(overflow.hint().unwrap_or_default());
        assert!(is_listing_overflow(&projected));
        assert!(!is_listing_overflow(&Error::usage("something else")));
    }
}
