# frozen_string_literal: true

module Basecamp
  # Raised when a chat line was found under none of the Campfires the caller can
  # currently see in its bucket.
  #
  # It is distinct from a failed read — any non-404 answer from a candidate is
  # raised as itself and stops the loop — and from
  # {CampfireDiscoveryIncompleteError}, where candidates were left unsearched:
  # here EVERY candidate answered 404.
  #
  # It is NOT distinct from lost visibility. BC3 answers 404 for a Campfire the
  # caller may not see, too, so "unresolved" means "under no Campfire the caller
  # can currently see". {#stale_campfire_ids} is what lets a consumer tell the
  # two apart: candidates the cache held that the refreshed sources no longer
  # list are Campfires the caller could see when the cache filled and cannot
  # now. A consumer marks the record blocked and retries on its own schedule.
  #
  # +code+ is +not_found+: the recording could not be located. The CLASS is what
  # keeps it distinct from a read's {NotFoundError} — this error carries no
  # +http_status+, because no single HTTP answer produced it.
  class UnresolvedRecordingError < RecordingSummaryError
    KIND = "recording_unresolved"

    # @return [Integer] the bucket the pointer named
    attr_reader :bucket_id

    # @return [Integer] the chat line's id
    attr_reader :recording_id

    # @return [Array<Integer>] the candidates tried, in order; empty when the
    #   bucket has no visible Campfire at all
    attr_reader :campfire_ids

    # @return [Array<Integer>] candidates from the cache that the refreshed
    #   sources no longer list. Set only when {#refreshed?}.
    attr_reader :stale_campfire_ids

    # @param bucket_id [Integer]
    # @param recording_id [Integer]
    # @param campfire_ids [Array<Integer>]
    # @param refreshed [Boolean] whether the cached discovery sources were
    #   re-read before concluding. False when every source had been read within
    #   the last +CampfireIndex::MIN_REFRESH+, so a Campfire created in that
    #   window was not seen: the conclusion stands on data up to that old, and a
    #   retry after the floor sees the current sources.
    # @param stale_campfire_ids [Array<Integer>]
    def initialize(bucket_id:, recording_id:, campfire_ids:, refreshed:, stale_campfire_ids: [])
      super(
        kind: KIND,
        code: ErrorCode::NOT_FOUND,
        message: "chat line found under no visible campfire: line #{recording_id} in bucket #{bucket_id} " \
                 "(tried #{campfire_ids.length} campfires)"
      )
      @bucket_id = bucket_id
      @recording_id = recording_id
      @campfire_ids = campfire_ids
      @refreshed = refreshed
      @stale_campfire_ids = stale_campfire_ids
    end

    # @return [Boolean] whether the cached discovery sources were re-read before
    #   concluding
    def refreshed?
      @refreshed
    end
  end
end
