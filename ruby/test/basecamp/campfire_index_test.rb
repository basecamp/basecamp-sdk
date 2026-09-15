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
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })
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
    # Released only once every waiter is provably parked inside `get` — a thread
    # that reached `get` can only be blocked on the cache's mutex or on its
    # condition variable, so "asleep" here means "waiting on the load". Without
    # this barrier a waiter that arrives after publication is served from the
    # cache instead, and the assertions below would pass or fail on timing.
    await_parked(waiters)
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

  def test_waiters_on_an_abandoned_load_are_woken_with_an_error_not_left_parked
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })
    started = Queue.new
    owner = Thread.new { cache.get(:k) { started << :loading; sleep } }
    started.pop
    await_parked([ owner ])
    waiter = Thread.new do
      cache.get(:k) { flunk "the waiter must not load while one is in flight" }
    rescue Basecamp::CampfireIndex::TTLCache::LoaderAbandoned => e
      e
    end
    await_parked([ waiter ])
    owner.kill
    owner.join

    assert waiter.join(5), "the waiter was left parked on an abandoned load"
    assert_kind_of Basecamp::CampfireIndex::TTLCache::LoaderAbandoned, waiter.value
  end

  # Blocks until every thread is parked, or fails the test rather than hanging.
  def await_parked(threads, timeout: 5)
    deadline = Process.clock_gettime(Process::CLOCK_MONOTONIC) + timeout
    until threads.all? { |thread| thread.status == "sleep" }
      flunk "threads never parked" if Process.clock_gettime(Process::CLOCK_MONOTONIC) > deadline

      Thread.pass
    end
  end

  def test_a_failed_load_is_shared_with_its_waiters_rather_than_re_run
    cache = Basecamp::CampfireIndex::TTLCache.new(ttl: 100.0, floor: 1.0, max_items: 10, clock: -> { @now })
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
    await_parked([ waiter ])
    release << :go

    assert_equal "boom", owner.join.value.message
    assert_equal "boom", waiter.join.value.message
    assert_equal 1, loads
  end
end
