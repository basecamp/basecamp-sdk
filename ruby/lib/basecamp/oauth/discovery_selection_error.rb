# frozen_string_literal: true

module Basecamp
  module Oauth
    # A hard resource-first selection/validation failure. Raised — never returned
    # as a fallback — so no consumer can convert it into a Launchpad request.
    #
    # The +reason+ is one of:
    # +ambiguous_issuers+, +expected_issuer_unavailable+, +invalid_issuer_origin+,
    # +as_fetch_failed+, +issuer_mismatch+, +capability_unavailable+.
    #
    # @attr reason [String] the hard-failure classification
    class DiscoverySelectionError < OauthError
      attr_reader :reason

      # @param reason [String] the hard-failure classification
      # @param message [String] human-readable description
      # @param http_status [Integer, nil] HTTP status code, if applicable
      def initialize(reason, message, http_status: nil)
        # Only capability_unavailable is consumer/usage-shaped (validation). Every
        # other reason — including expected_issuer_unavailable — is an AS metadata
        # fault surfaced as api_error, matching the other four SDKs (an issuer the
        # resource does not advertise is a metadata fault, not a caller-usage one).
        type = reason == "capability_unavailable" ? "validation" : "api_error"
        super(type, message, http_status: http_status)
        @reason = reason
      end
    end
  end
end
