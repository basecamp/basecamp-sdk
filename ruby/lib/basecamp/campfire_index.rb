# frozen_string_literal: true

module Basecamp
  # The two Campfire discovery sources a chat line's read needs, each cached.
  #
  # A +chat.line.created+ row carries the line's id and bucket, not its
  # Campfire, and the line read is +/chats/{campfireId}/lines/{lineId}+.
  # Candidates come from two sources, tried in order and each cached {TTL}
  # seconds:
  #
  # 1. The bucket's project dock, whose "chat" tool is the project's Campfire:
  #    one project read per bucket, and the answer for every line posted in a
  #    project. Cached per bucket.
  # 2. The account-wide Campfire listing (BC3 has no per-bucket one), filtered
  #    to the bucket, for buckets that are not projects or whose dock did not
  #    hold the line. Cached per account, so a burst of lines costs one listing.
  #
  # This class holds the caching only; the reads themselves are passed in as
  # blocks by +RecordingsExtensions+, so every request still goes through a
  # generated service method (SPEC section 18 rule 1) and this file touches no
  # wire.
  #
  # It lives on {Client}, shared by every {AccountClient} that client hands out.
  # A Client is bound to one credential, so entries are never shared across
  # authorization contexts, and every key carries the account id anyway. Expired
  # snapshots are swept at each publication and each cache is bounded
  # ({MAX_ITEMS}, oldest fetched out first), so the index holds at most the
  # buckets and accounts consulted within the last TTL, and never more than the
  # bound, whatever the Client has seen.
  class CampfireIndex
    # How long a cached discovery source — a bucket's project dock, the
    # account's Campfire listing — is reused before it is read again.
    TTL = 600.0

    # Bounds the refresh-on-miss: a line found under no candidate re-reads the
    # cached sources, but not more often than this per source, so a run of
    # unresolvable lines cannot turn into a listing per line.
    # +UnresolvedRecordingError#refreshed?+ says whether the floor applied.
    MIN_REFRESH = 30.0

    # Caps the account-wide Campfire listing the fallback source reads. A
    # listing that overflows it is not cached and the call reports
    # {CampfireDiscoveryIncompleteError}: the dock covers every project, so the
    # listing only ever serves the leftover, and an account with more Campfires
    # than this should not pay a full walk per TTL for it.
    MAX_LISTING = 1000

    # Bounds each cache's entry count. The dock cache holds one snapshot per
    # bucket consulted within the TTL: a connector listening across every
    # project an agent can see touches hundreds of buckets, not thousands, and a
    # snapshot is a handful of ids, so this is generous headroom at a few
    # hundred KB. The bound exists so that a process alive for weeks can never
    # grow past it whatever it sees. When the bound is reached the
    # oldest-fetched entries go first, deterministically.
    MAX_ITEMS = 1024

    # Raised by the listing loader when the account-wide listing overflows
    # {MAX_LISTING}. +RecordingsExtensions+ turns it into the typed
    # {CampfireDiscoveryIncompleteError}; it is never cached, so the next call
    # re-reads rather than remembering a verdict it never reached.
    class ListingOverflow < StandardError; end

    # One consultation of a discovery source: the candidate ids it holds for the
    # bucket, when that snapshot was fetched, and whether it predated the call.
    SourceRead = Struct.new(:ids, :fetched, :cached, keyword_init: true) do
      # @return [Boolean] whether the snapshot predated this call
      def cached?
        cached
      end
    end

    # @param clock [#call, nil] monotonic seconds, injectable for tests
    def initialize(clock: nil)
      @docks = TTLCache.new(ttl: TTL, floor: MIN_REFRESH, max_items: MAX_ITEMS, clock: clock)
      @listings = TTLCache.new(ttl: TTL, floor: MIN_REFRESH, max_items: MAX_ITEMS, clock: clock)
    end

    # The Campfire ids a bucket's project dock names.
    #
    # @param account_id [String]
    # @param bucket_id [Integer]
    # @param refresh [Boolean] re-read a cached snapshot older than {MIN_REFRESH}
    # @yieldreturn [Array<Integer>] the dock's Campfire ids
    # @return [SourceRead]
    def dock_campfires(account_id:, bucket_id:, refresh: false, &load)
      hit = @docks.get([ account_id, bucket_id ], refresh: refresh, &load)
      SourceRead.new(ids: Array(hit.value), fetched: hit.fetched, cached: hit.cached)
    end

    # The Campfire ids the CACHED account-wide listing shows in a bucket,
    # without fetching. nil when the listing is not cached or has expired.
    #
    # It exists so a caller can consult what the source already holds before
    # deciding whether to pay for the listing fetch — the expensive, slow
    # request, which the dock (including its refresh) should get to pre-empt.
    #
    # @param account_id [String]
    # @param bucket_id [Integer]
    # @return [SourceRead, nil]
    def cached_listed_campfires(account_id:, bucket_id:)
      hit = @listings.peek(account_id)
      return nil if hit.nil?

      SourceRead.new(ids: Array(hit.value[bucket_id]).dup, fetched: hit.fetched, cached: true)
    end

    # The Campfire ids the account-wide listing shows in a bucket.
    #
    # @param account_id [String]
    # @param bucket_id [Integer]
    # @param refresh [Boolean] re-read a cached snapshot older than {MIN_REFRESH}
    # @yieldreturn [Hash{Integer => Array<Integer>}] campfire ids by bucket id
    # @return [SourceRead]
    # @raise [ListingOverflow] when the loader reports the listing was truncated
    def listed_campfires(account_id:, bucket_id:, refresh: false, &load)
      hit = @listings.get(account_id, refresh: refresh, &load)
      SourceRead.new(ids: Array(hit.value[bucket_id]).dup, fetched: hit.fetched, cached: hit.cached)
    end

    # A per-key cache with single-flight loading: concurrent callers for one key
    # wait on the one load in progress rather than loading again, a failed load
    # leaves the previous value in place (and its error is shared with the
    # waiters, so N callers never re-run one failed load N times), and a refresh
    # is honoured only once the value is older than a floor.
    class TTLCache
      # Handed to the waiters when a load left without publishing an outcome of
      # its own — see the +ensure+ in {#get}.
      class LoaderAbandoned < StandardError; end

      # What a cache read hands back: the value, when it was fetched, and
      # whether it predated the call (as opposed to being loaded during it, by
      # this caller or by one it waited on). The fetch time is what lets a
      # caller tell a snapshot it already consulted from a newer one, whoever
      # loaded it.
      Hit = Struct.new(:value, :fetched, :cached, keyword_init: true)

      Entry = Struct.new(:value, :fetched, :seq, keyword_init: true)

      # The clock is MONOTONIC, never wall time. A backwards step in wall time —
      # an NTP correction, a VM resume — would make an entry outlive its TTL,
      # decline a refresh that is genuinely due, and take the `refreshed` and
      # stale-candidate signals down with it, none of which surfaces as an
      # error. The injected clock in tests is monotonic in the same sense: it
      # only ever moves forward.
      #
      # @param ttl [Float] seconds a value is reused for
      # @param floor [Float] seconds a refresh must wait before it is honoured
      # @param max_items [Integer] entry bound; 0 or less disables eviction
      # @param clock [#call, nil] monotonic seconds, injectable for tests
      def initialize(ttl:, floor:, max_items:, clock: nil)
        @ttl = ttl
        @floor = floor
        @max_items = max_items
        @clock = clock || -> { Process.clock_gettime(Process::CLOCK_MONOTONIC) }
        @mutex = Mutex.new
        @condition = ConditionVariable.new
        @entries = {}
        @inflight = {}
        @seq = 0
      end

      # Returns the value for key, loading it when absent or older than the TTL
      # — or, with +refresh+ set, older than the floor.
      #
      # @param key [Object]
      # @param refresh [Boolean]
      # @yieldreturn [Object] the loaded value
      # @return [Hit]
      def get(key, refresh: false)
        owner = false
        pending = nil
        published = false

        # The ensure encloses the REGISTRATION, not just the loader. A loader
        # that leaves by anything the rescue below does not catch — Interrupt, a
        # signal, NoMemoryError, a Thread#kill that raises nothing at all —
        # would otherwise leave the key in flight forever, and since {#await}
        # waits with no timeout, every later caller for that key would park on
        # it permanently rather than erroring. The listing's key is the bare
        # account id, so that is one killed thread wedging chat-line discovery
        # for a whole account for the life of the process. Go publishes from a
        # deferred recover for exactly this reason — and an ensure that began
        # after the key was registered would leave the same hole a few
        # instructions wide.
        begin
          @mutex.synchronize do
            entry = @entries[key]
            if entry
              age = @clock.call - entry.fetched
              if age < @ttl && (!refresh || age < @floor)
                return Hit.new(value: entry.value, fetched: entry.fetched, cached: true)
              end
            end

            pending = @inflight[key]
            if pending.nil?
              pending = { done: false, error: nil, value: nil, fetched: nil }
              # Ownership is claimed BEFORE the key is registered, so an
              # interrupt between the two leaves the ensure publishing a record
              # nothing is registered against — harmless — rather than a
              # registration nothing will ever release.
              owner = true
              @inflight[key] = pending
            end
          end

          return await(pending) unless owner

          value = yield
          publish(key, pending, value, nil)
          published = true
          Hit.new(value: value, fetched: pending[:fetched], cached: false)
        rescue StandardError => e
          publish(key, pending, nil, e) if owner
          published = true
          raise
        ensure
          publish(key, pending, nil, LoaderAbandoned.new("cache loader did not complete")) if owner && !published
        end
      end

      # Returns the cached value for key when one is within the TTL, without
      # loading.
      #
      # @param key [Object]
      # @return [Hit, nil]
      def peek(key)
        @mutex.synchronize do
          entry = @entries[key]
          return nil if entry.nil? || (@clock.call - entry.fetched) >= @ttl

          Hit.new(value: entry.value, fetched: entry.fetched, cached: true)
        end
      end

      private

      # Waits on another caller's load and hands its outcome over.
      #
      # The value is read off the load record, not out of the cache, so a sweep
      # or the entry bound evicting the entry in the meantime cannot take it
      # from this waiter. It is reported as loaded during this call rather than
      # as something that predated it, because it was.
      def await(pending)
        @mutex.synchronize do
          @condition.wait(@mutex) until pending[:done]
        end
        raise pending[:error] if pending[:error]

        Hit.new(value: pending[:value], fetched: pending[:fetched], cached: false)
      end

      # Releases the key and wakes the waiters. A failed load publishes no entry,
      # so the previous value — if any — stays in place and keeps serving until
      # its own TTL runs out.
      def publish(key, pending, value, error)
        @mutex.synchronize do
          # Idempotent, so publication survives being interrupted. An async
          # exception landing between the load returning and this call
          # completing would otherwise let {#get}'s ensure publish a second time
          # and overwrite a good value with an abandonment error. Whichever
          # publication lands first is the outcome; the rest only wake the
          # waiters again.
          #
          # The repeat still broadcasts, and that is the point of doing it here
          # rather than returning bare: an interrupt landing between the flag
          # below and its broadcast would otherwise leave a record marked done
          # with everybody still parked on it.
          if pending[:done]
            @condition.broadcast
            return
          end

          @inflight.delete(key)
          pending[:error] = error
          if error.nil?
            pending[:value] = value
            pending[:fetched] = @clock.call
            sweep_locked
            make_room_locked(key)
            @seq += 1
            @entries[key] = Entry.new(value: value, fetched: pending[:fetched], seq: @seq)
          end
          pending[:done] = true
          @condition.broadcast
        end
      end

      # Drops every entry past its TTL. Runs at each publication — the one
      # moment the cache does work proportional to a miss anyway — so a
      # long-lived Client that has seen many buckets keeps a snapshot for at most
      # a TTL past its last use plus the interval to the next load on any key,
      # rather than for its lifetime.
      def sweep_locked
        now = @clock.call
        @entries.delete_if { |_key, entry| (now - entry.fetched) >= @ttl }
      end

      # Evicts the oldest-fetched entries until the one about to be stored for
      # +key+ fits under the bound — oldest by fetch time, and by publication
      # order among equals, so the choice is total rather than whatever hash
      # order happens to come first. Oldest-first is also the right order: the
      # entry nearest its TTL is the one least worth keeping.
      def make_room_locked(key)
        return if @max_items <= 0
        return if @entries.key?(key) # an overwrite takes no new room

        while @entries.size >= @max_items
          oldest = @entries.min_by { |_k, entry| [ entry.fetched, entry.seq ] }
          break if oldest.nil?

          @entries.delete(oldest.first)
        end
      end
    end
  end
end
