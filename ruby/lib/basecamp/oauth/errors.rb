# frozen_string_literal: true

module Basecamp
  module Oauth
    # OAuth-specific error class.
    #
    # @attr type [String] Error type ("validation", "auth", "network", "api_error")
    # @attr http_status [Integer, nil] HTTP status code if applicable
    # @attr hint [String, nil] Helpful hint for resolving the error
    # @attr retryable [Boolean] Whether the request can be retried
    class OAuthError < StandardError
      attr_reader :type, :http_status, :hint, :retryable

      # @param type [String] Error type
      # @param message [String] Error message
      # @param http_status [Integer, nil] HTTP status code
      # @param hint [String, nil] Helpful hint
      # @param retryable [Boolean] Whether retryable
      def initialize(type, message, http_status: nil, hint: nil, retryable: false)
        super(message)
        @type = type
        @http_status = http_status
        @hint = hint
        @retryable = retryable
      end

      def to_s
        parts = [ "[#{type}] #{super}" ]
        parts << "(HTTP #{http_status})" if http_status
        parts << "Hint: #{hint}" if hint
        parts.join(" ")
      end
    end
  end
end
