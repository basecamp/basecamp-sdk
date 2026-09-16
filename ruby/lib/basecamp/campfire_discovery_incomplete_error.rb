# frozen_string_literal: true

module Basecamp
  # Raised when a chat line's Campfire discovery could not be carried to a
  # conclusion — the account-wide Campfire listing overflowed
  # +CampfireIndex::MAX_LISTING+, or the bucket has more visible Campfires than
  # +RecordingsExtensions::MAX_CAMPFIRE_CANDIDATES+.
  #
  # Deliberately distinct from {UnresolvedRecordingError}: candidates were left
  # unsearched, so nothing can be reported absent. Nothing left unsearched is
  # ever called missing.
  #
  # +code+ is +usage+ with +retryable+ false, settled across every port on
  # card 40[https://app.basecamp.com/2914079/buckets/48699913/card_tables/cards/10308122086]
  # after the merged ports shipped two different answers.
  #
  # +usage+ is the one coarse code no HTTP response can produce — the status
  # mapping yields +auth_required+, +forbidden+, +not_found+, +rate_limit+,
  # +validation+, +limit_exceeded+ and +api_error+, never this one — so a
  # verdict the composite reached on its own can never be read back as a
  # constituent read's own answer. This port previously said +api_error+, which
  # a caller could not tell from a 500 one of those reads returned.
  #
  # Retryability is a separate field and is unchanged: false, because the call
  # reached no verdict and no argument the caller can change would produce one
  # — the bounds are the SDK's, both reasons are deterministic for the same
  # account state, and a retry loop would re-run the identical search forever.
  class CampfireDiscoveryIncompleteError < RecordingSummaryError
    KIND = "campfire_discovery_incomplete"

    # @return [Integer] the bucket the pointer named
    attr_reader :bucket_id

    # @return [Integer] the chat line's id
    attr_reader :recording_id

    # @return [String] why discovery stopped short
    attr_reader :reason

    # @param bucket_id [Integer]
    # @param recording_id [Integer]
    # @param reason [String]
    def initialize(bucket_id:, recording_id:, reason:)
      super(
        kind: KIND,
        code: ErrorCode::USAGE,
        message: "campfire discovery incomplete: line #{recording_id} in bucket #{bucket_id}: #{reason}"
      )
      @bucket_id = bucket_id
      @recording_id = recording_id
      @reason = reason
    end
  end
end
