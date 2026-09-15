# frozen_string_literal: true

module Basecamp
  # Raised when the recording a typed read returned lives in a different bucket
  # from the one the pointer named, so a pointer from one project can never
  # resolve to a recording in another.
  #
  # +code+ is +usage+: the read succeeded and the API answered honestly; it is
  # the caller's pointer that disagreed with it.
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
