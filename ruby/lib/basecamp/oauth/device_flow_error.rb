# frozen_string_literal: true

module Basecamp
  module Oauth
    # A terminal RFC 8628 device-flow outcome. Carries a +reason+; the parent
    # {OauthError} +type+ is DERIVED from that reason (SPEC.md §16) so callers can
    # branch on either the precise +reason+ or the coarse +type+.
    #
    # | reason           | parent type          |
    # |------------------|----------------------|
    # | +:access_denied+ | +auth+               |
    # | +:expired+       | +auth+               |
    # | +:transport+     | +network+ (retryable)|
    # | +:unavailable+   | +validation+         |
    # | +:cancelled+     | +usage+              |
    #
    # @attr reason [Symbol] the device-flow termination reason
    class DeviceFlowError < OauthError
      # Maps a device-flow reason to its parent {OauthError} type.
      REASON_TYPES = {
        access_denied: "auth",
        expired: "auth",
        transport: "network",
        unavailable: "validation",
        cancelled: "usage"
      }.freeze

      attr_reader :reason

      # @param reason [Symbol] the device-flow termination reason
      # @param message [String] human-readable description
      # @param http_status [Integer, nil] HTTP status code, if applicable
      # @param hint [String, nil] helpful hint for resolving the error
      def initialize(reason, message, http_status: nil, hint: nil)
        super(
          REASON_TYPES.fetch(reason, "api_error"),
          message,
          http_status: http_status,
          hint: hint,
          retryable: reason == :transport
        )
        @reason = reason
      end
    end
  end
end
