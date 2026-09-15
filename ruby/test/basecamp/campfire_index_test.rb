# frozen_string_literal: true

require "test_helper"

# Tests for the discovery caches `recordings.summarize` consults for a chat line.
#
# These are about the cache's own contract — TTL, the refresh floor, the entry
# bound, single-flight loading, what a failed load leaves behind — which the
# conformance fixture cannot reach: a fixture case is one call on a fresh client.
class CampfireIndexTest < Minitest::Test
  include TestHelper

  def setup
    @now = 0.0
    @index = Basecamp::CampfireIndex.new(clock: -> { @now })
    @loads = 0
  end

  def dock(refresh: false, bucket_id: 1, account_id: "12345", ids: [ 500 ])
    @index.dock_campfires(account_id: account_id, bucket_id: bucket_id, refresh: refresh) do
      @loads += 1
      ids
    end
  end

  def test_a_value_is_reused_until_its_ttl_runs_out
    first = dock

    assert_equal [ 500 ], first.ids
    assert_not first.cached? # the first read loaded it

    @now += Basecamp::CampfireIndex::TTL - 1
    assert dock.cached?
    assert_equal 1, @loads

    @now += 2
    assert_not dock.cached?
    assert_equal 2, @loads
  end

  def test_a_refresh_is_declined_until_the_floor_has_passed
    dock
    @now += Basecamp::CampfireIndex::MIN_REFRESH - 1

    assert dock(refresh: true).cached?
    assert_equal 1, @loads

    @now += 2

    assert_not dock(refresh: true).cached?
    assert_equal 2, @loads
  end

  def test_entries_are_keyed_by_account_and_bucket
    dock(bucket_id: 1)
    dock(bucket_id: 2)
    dock(bucket_id: 1, account_id: "999")

    assert_equal 3, @loads
    assert dock(bucket_id: 1).cached?
  end

  def test_a_failed_load_leaves_the_previous_value_in_place
    dock
    @now += Basecamp::CampfireIndex::TTL + 1

    assert_raises(RuntimeError) do
      @index.dock_campfires(account_id: "12345", bucket_id: 1) { raise "boom" }
    end

    # The failure published nothing, so the entry is still there — expired, and
    # the next call loads for itself rather than serving it.
    assert_equal [ 501 ], @index.dock_campfires(account_id: "12345", bucket_id: 1) { [ 501 ] }.ids
  end

  def test_peek_never_loads_and_expires_with_the_ttl
    assert_nil @index.cached_listed_campfires(account_id: "12345", bucket_id: 1)

    @index.listed_campfires(account_id: "12345", bucket_id: 1) { { 1 => [ 500 ], 2 => [ 600 ] } }

    assert_equal [ 500 ], @index.cached_listed_campfires(account_id: "12345", bucket_id: 1).ids
    # The listing is account-wide and filtered per bucket.
    assert_equal [ 600 ], @index.cached_listed_campfires(account_id: "12345", bucket_id: 2).ids
    assert_empty @index.cached_listed_campfires(account_id: "12345", bucket_id: 3).ids

    @now += Basecamp::CampfireIndex::TTL
    assert_nil @index.cached_listed_campfires(account_id: "12345", bucket_id: 1)
  end

  def test_a_cached_listing_is_not_shared_between_accounts
    @index.listed_campfires(account_id: "12345", bucket_id: 1) { { 1 => [ 500 ] } }

    assert_nil @index.cached_listed_campfires(account_id: "999", bucket_id: 1)
  end

  def test_the_entry_bound_evicts_the_oldest_fetched_first
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 2, clock: -> { @now })
    cache.get(:a) { "a" }
    @now += 1
    cache.get(:b) { "b" }
    @now += 1
    cache.get(:c) { "c" }

    assert_nil cache.peek(:a)
    assert_equal "b", cache.peek(:b).value
    assert_equal "c", cache.peek(:c).value
  end

  def test_expired_entries_are_swept_at_each_publication
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 10.0, floor: 1.0, max_items: 100, clock: -> { @now })
    cache.get(:a) { "a" }
    @now += 11
    cache.get(:b) { "b" }

    # :a is gone rather than held for the process's lifetime.
    assert_nil cache.peek(:a)
  end

  def test_concurrent_callers_share_one_load
    cache, arrivals = cache_with_waiter_barrier
    started = Queue.new
    release = Queue.new
    loads = 0

    owner = Thread.new do
      cache.get(:k) do
        loads += 1
        started << :loading
        release.pop
        "value"
      end
    end
    started.pop

    waiters = Array.new(3) { Thread.new { cache.get(:k) } }
    # Released only once every waiter has announced itself from inside the wait.
    3.times { arrivals.pop }
    release << :go
    hits = ([ owner ] + waiters).map { |thread| thread.join.value }

    assert_equal 1, loads
    assert_equal [ "value" ] * 4, hits.map(&:value)
    # Every one of them loaded during this call rather than finding a snapshot
    # that predated it.
    assert_equal [ false ] * 4, hits.map(&:cached)
  end

  def test_the_default_clock_is_monotonic
    # Wall time can step backwards — an NTP correction, a VM resume — and every
    # consequence is silent: an entry outliving its TTL, a genuinely due refresh
    # declined, the `refreshed` and stale-candidate signals lost with it.
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 1.0, floor: 1.0, max_items: 10)
    clock = cache.instance_variable_get(:@clock)

    before = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    reading = clock.call
    after = Process.clock_gettime(Process::CLOCK_MONOTONIC)

    assert_operator reading, :>=, before
    assert_operator reading, :<=, after
    # A wall clock would be far away from the monotonic one; the same clock is
    # within the window above, which Time.now.to_f never would be.
    assert_operator (reading - Time.now.to_f).abs, :>, 1.0
  end

  def test_publication_is_idempotent
    # An async exception landing between the load returning and the key being
    # released lets `get`'s ensure publish a second time; the first outcome has
    # to stand rather than being overwritten with an abandonment error.
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })
    pending = { done: false, error: nil, value: nil, fetched: nil }
    cache.send(:publish, :k, pending, "first", nil)
    cache.send(:publish, :k, pending, nil, RuntimeError.new("late"))

    assert_nil pending[:error]
    assert_equal "first", pending[:value]
    assert_equal "first", cache.peek(:k).value
  end

  def test_a_loader_that_leaves_without_an_outcome_releases_the_key
    # Not a StandardError, so neither rescue arm runs: an Interrupt, a signal, a
    # Thread#kill. Without the ensure the key stays in flight and every later
    # caller parks on a wait that has no timeout — one killed thread wedging
    # discovery for a whole account for the life of the process.
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })

    Thread.new do
      cache.get(:k) { raise Interrupt }
    rescue Exception # rubocop:disable Lint/SuppressedException, Lint/RescueException
    end.join

    later = Thread.new { cache.get(:k) { "recovered" } }

    assert later.join(5), "the key was never released; a later caller parked forever"
    assert_equal "recovered", later.value.value
  end

  def test_a_killed_loader_releases_the_key_too
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })
    started = Queue.new
    owner = Thread.new { cache.get(:k) { started << :loading; sleep } }
    started.pop
    await_parked([ owner ])
    owner.kill
    owner.join

    later = Thread.new { cache.get(:k) { "recovered" } }

    assert later.join(5), "the key was never released; a later caller parked forever"
    assert_equal "recovered", later.value.value
  end

  def test_a_waiter_loads_for_itself_when_the_owner_was_abandoned
    # An abandoned load is not a failed one: the loading thread was killed, or
    # unwound by something that is not a StandardError. That says nothing about
    # whether this caller's own call can succeed, so it goes round once. Without
    # it, one caller's Timeout.timeout fails every concurrent waiter on a key
    # the whole account shares.
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })
    started = Queue.new
    owner = Thread.new { cache.get(:k) { started << :loading; sleep } }
    started.pop
    await_parked([ owner ])
    waiter = Thread.new { cache.get(:k) { "loaded by the waiter" } }
    await_parked([ waiter ])
    owner.kill
    owner.join

    assert waiter.join(5), "the waiter was left parked on an abandoned load"
    assert_equal "loaded by the waiter", waiter.value.value
  end

  def test_a_waiter_re_runs_an_abandoned_load_only_once
    # The bound, reached rather than implied: the waiter's OWN re-run is
    # abandoned too, and it must raise rather than load a third time. Deleting
    # the bound turns this into an unbounded per-waiter reload, which is the
    # request amplification the whole rule exists to prevent — so this test has
    # to fail when the bound is removed, and nothing else in this file does.
    cache, arrivals = cache_with_waiter_barrier
    loads = Queue.new
    loaders = Queue.new

    owner = Thread.new { cache.get(:k) { loads << Thread.current; loaders.pop } }
    loads.pop

    waiter = Thread.new do
      cache.get(:k) { loads << Thread.current; loaders.pop }
    rescue Basecamp::CampfireIndex::TTLCache::LoaderAbandoned => e
      e
    end
    arrivals.pop
    owner.kill
    owner.join

    # The waiter has used its one re-run and is now the owner, loading for
    # itself. Abandon that load too.
    second_loader = loads.pop

    assert_equal waiter, second_loader
    await_parked([ waiter ])
    waiter.kill
    waiter.join

    # A third caller finds the key free and loads normally, so the kill above
    # released it — the bound is about the WAITER, not about wedging the key.
    third = Thread.new { cache.get(:k) { "third" } }

    assert third.join(5), "the key was never released"
    assert_equal "third", third.value.value
  end

  def test_a_waiter_raises_rather_than_re_running_twice
    # The same bound, observed from the waiter rather than from the key: two
    # abandonments in a row and the waiter gets the error instead of a third
    # load.
    #
    # Which of the two waiters wins the re-run is arbitrary, so the test does
    # not assume it — the loader announces itself, and the OTHER one is the
    # thread whose re-run is spent.
    cache, arrivals = cache_with_waiter_barrier
    loaders = Queue.new
    held = Queue.new
    abandoned = Basecamp::CampfireIndex::TTLCache::LoaderAbandoned

    load_once = lambda do
      Thread.new do
        cache.get(:k) { loaders << Thread.current; held.pop }
      rescue abandoned => e
        e
      end
    end

    owner = load_once.call
    loaders.pop
    waiters = [ load_once.call, load_once.call ]
    2.times { arrivals.pop }

    owner.kill
    owner.join

    # One waiter spent its re-run becoming the loader; the other spent its own
    # queueing behind that load.
    second_loader = loaders.pop
    arrivals.pop
    spent = (waiters - [ second_loader ]).first

    await_parked([ second_loader ])
    second_loader.kill
    second_loader.join

    assert spent.join(5), "the waiter with its re-run spent was left parked"
    assert_kind_of abandoned, spent.value
    # The abandonment it woke on, propagated — not a substitute minted after the
    # attempts ran out. Those are different lines, and only this distinguishes
    # them.
    assert_equal "cache loader did not complete", spent.value.message
    assert_empty loaders, "the waiter loaded a third time instead of raising"
  end

  def test_a_loader_that_raises_stop_iteration_still_raises
    # Kernel#loop rescues StopIteration and returns its result, so wrapping get
    # in one would hand the caller an arbitrary value from the loader's
    # enumerator in place of a Hit — a raise turned into a silently wrong
    # answer, which the caller then dies on somewhere unrelated.
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })

    assert_raises(StopIteration) { cache.get(:k) { raise StopIteration } }
    assert_raises(StopIteration) do
      enumerator = [].each
      cache.get(:k2) { enumerator.next }
    end
  end

  def test_an_abandoned_load_is_a_basecamp_error
    # A consumer's `rescue Basecamp::Error` around a composite has to catch it
    # like everything else, rather than meeting an SDK-internal class with no
    # code.
    error = Basecamp::CampfireIndex::TTLCache::LoaderAbandoned.new

    assert_kind_of Basecamp::Error, error
    assert_equal Basecamp::ErrorCode::API, error.code
    assert_predicate error, :retryable?
  end

  def test_an_abandonment_publish_never_evicts_another_threads_registration
    # The interrupt window the ensure covers can publish a record that was never
    # registered. Deleting the key blindly would drop the single-flight
    # guarantee of whatever thread has since registered under it — and the
    # listing's key is the bare account id, so that is process-wide.
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })
    started = Queue.new
    release = Queue.new
    loads = 0
    live = Thread.new do
      cache.get(:k) do
        loads += 1
        started << :loading
        release.pop
        "the live load"
      end
    end
    started.pop

    ghost = { done: false, error: nil, value: nil, fetched: nil }
    cache.send(:publish, :k, ghost, nil, Basecamp::CampfireIndex::TTLCache::LoaderAbandoned.new)

    waiter = Thread.new { cache.get(:k) { flunk "the live registration was evicted" } }
    await_parked([ waiter ])
    release << :go

    assert_equal "the live load", live.join.value.value
    assert_equal "the live load", waiter.join.value.value
    assert_equal 1, loads
  end

  # A cache whose waiters announce themselves as they reach the wait, and the
  # queue they announce on.
  #
  # Thread#status is NOT a sound barrier here: Ruby reports a thread merely
  # blocked on the cache's lock as "sleep" too, so a waiter could still be at
  # the entry check when the test released the owner, read the published entry,
  # and report a cache hit — timing-dependent again. The seam fires under the
  # lock, so a waiter the test has heard from provably reaches the wait before
  # anything else can take the lock.
  def cache_with_waiter_barrier(ttl: 100.0, floor: 1.0, max_items: 10)
    arrivals = Queue.new
    cache = Basecamp::CampfireIndex::TTLCache.new(
      ttl: ttl, floor: floor, max_items: max_items, clock: -> { @now },
      on_wait: -> { arrivals << :waiting }
    )
    [ cache, arrivals ]
  end

  # Blocks until every thread has stopped running, or fails rather than hanging.
  # Only for threads parked in a loader the test controls — never as a barrier
  # against a publication, which is what the seam above is for.
  def await_parked(threads, timeout: 5)
    deadline = Process.clock_gettime(Process::CLOCK_MONOTONIC) + timeout
    until threads.all? { |thread| thread.status == "sleep" }
      flunk "threads never parked" if Process.clock_gettime(Process::CLOCK_MONOTONIC) > deadline

      Thread.pass
    end
  end

  def test_a_failed_load_is_shared_with_its_waiters_rather_than_re_run
    cache, arrivals = cache_with_waiter_barrier
    started = Queue.new
    release = Queue.new
    loads = 0

    owner = Thread.new do
      cache.get(:k) do
        loads += 1
        started << :loading
        release.pop
        raise "boom"
      end
    rescue RuntimeError => e
      e
    end
    started.pop

    waiter = Thread.new do
      cache.get(:k) { flunk "the waiter must not re-run a failed load" }
    rescue RuntimeError => e
      e
    end
    arrivals.pop
    release << :go

    assert_equal "boom", owner.join.value.message
    assert_equal "boom", waiter.join.value.message
    assert_equal 1, loads
  end
end
