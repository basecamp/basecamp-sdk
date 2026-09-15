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
  # +code+ is +api_error+ with +retryable+ false: the call reached no verdict,
  # and no argument the caller can change would produce one — the bounds are the
  # SDK's, and a bucket past them is not a shape BC3 produces.
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
        code: ErrorCode::API,
        message: "campfire discovery incomplete: line #{recording_id} in bucket #{bucket_id}: #{reason}"
      )
      @bucket_id = bucket_id
      @recording_id = recording_id
      @reason = reason
    end
  end
end
