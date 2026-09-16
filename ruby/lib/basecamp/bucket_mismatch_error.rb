# frozen_string_literal: true

module Basecamp
  # Raised when the recording a typed read returned lives in a different bucket
  # from the one the pointer named, so a pointer from one project can never
  # resolve to a recording in another.
  #
  # +code+ is +usage+ with +retryable+ false, settled across every port on
  # card 41[https://app.basecamp.com/2914079/buckets/48699913/card_tables/cards/10308966794] after
  # Rust shipped +not_found+ where this port and three others shipped +usage+.
  # The read succeeded and the API answered honestly — it FOUND the recording,
  # in another bucket, and returned it, so nothing is absent and +not_found+
  # would be a false claim. It is the caller's pointer that disagreed with it.
  class BucketMismatchError < RecordingSummaryError
    KIND = "bucket_mismatch"

    # @return [Integer] the bucket the pointer named
    attr_reader :bucket_id

    # @return [Integer] the bucket the read returned
    attr_reader :actual_bucket_id

    # @return [Integer] the recording's id
    attr_reader :recording_id

    # @param bucket_id [Integer] the bucket the pointer named
    # @param actual_bucket_id [Integer] the bucket the read returned
    # @param recording_id [Integer]
    def initialize(bucket_id:, actual_bucket_id:, recording_id:)
      super(
        kind: KIND,
        code: ErrorCode::USAGE,
        message: "recording is not in the requested bucket: recording #{recording_id} is in bucket " \
                 "#{actual_bucket_id}, not #{bucket_id}"
      )
      @bucket_id = bucket_id
      @actual_bucket_id = actual_bucket_id
      @recording_id = recording_id
    end
  end
end
