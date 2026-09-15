# frozen_string_literal: true

module Basecamp
  # Raised for a recording pointer +summarize+ cannot route, before any request
  # is made. Two kinds:
  #
  # * +no_recording_type+ — an event type that names no recording type, which is
  #   +boost.*+: the row points at the boost's target and does not carry that
  #   target's type. A consumer resolves those from its own record of what it
  #   posted, not through +summarize+.
  # * +unknown_recording_type+ — neither the event type nor the recording type
  #   names a type in the routing table. The table is a DELIBERATE set, not an
  #   exhaustive one (see +RecordingsExtensions::RECORDING_TYPES+), so a type
  #   outside it is this error by design, whether or not the SDK has an id-only
  #   read for it.
  #
  # +code+ is +usage+: nothing was asked of the API, and the caller's own
  # pointer is what cannot be routed.
  class RecordingRoutingError < RecordingSummaryError
    NO_RECORDING_TYPE = "no_recording_type"
    UNKNOWN_RECORDING_TYPE = "unknown_recording_type"

    # @return [String] the recording type or event type that could not be routed
    attr_reader :routing_key

    # @param kind [String] {NO_RECORDING_TYPE} or {UNKNOWN_RECORDING_TYPE}
    # @param routing_key [String, nil] the type string that could not be routed
    def initialize(kind:, routing_key:)
      reason = kind == NO_RECORDING_TYPE ? "event type names no recording type" : "no typed read for recording type"
      super(
        kind: kind,
        code: ErrorCode::USAGE,
        message: "#{reason}: #{routing_key.to_s.inspect}"
      )
      @routing_key = routing_key.to_s
    end

    # @param routing_key [String, nil]
    # @return [RecordingRoutingError]
    def self.no_recording_type(routing_key)
      new(kind: NO_RECORDING_TYPE, routing_key: routing_key)
    end

    # @param routing_key [String, nil]
    # @return [RecordingRoutingError]
    def self.unknown_recording_type(routing_key)
      new(kind: UNKNOWN_RECORDING_TYPE, routing_key: routing_key)
    end
  end
end
