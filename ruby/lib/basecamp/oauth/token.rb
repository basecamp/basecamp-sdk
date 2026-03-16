# frozen_string_literal: true

module Basecamp
  module Oauth
    # OAuth 2 access token response.
    #
    # @attr access_token [String] The access token string
    # @attr token_type [String] Token type (usually "Bearer")
    # @attr refresh_token [String, nil] The refresh token string
    # @attr expires_in [Integer, nil] Lifetime of the access token in seconds
    # @attr expires_at [Time, nil] Calculated expiration time
    # @attr scope [String, nil] OAuth scope granted
    Token = Data.define(
      :access_token,
      :token_type,
      :refresh_token,
      :expires_in,
      :expires_at,
      :scope
    ) do
      def initialize(
        access_token:,
        token_type: "Bearer",
        refresh_token: nil,
        expires_in: nil,
        expires_at: nil,
        scope: nil
      )
        # Calculate expires_at from expires_in if not provided
        calculated_expires_at = expires_at || (expires_in ? Time.now + expires_in : nil)
        super(
          access_token: access_token,
          token_type: token_type,
          refresh_token: refresh_token,
          expires_in: expires_in,
          expires_at: calculated_expires_at,
          scope: scope
        )
      end

      # Checks if the token is expired or about to expire.
      #
      # @param buffer_seconds [Integer] Buffer time before actual expiration (default: 60)
      # @return [Boolean] true if expired or will expire within buffer time
      def expired?(buffer_seconds = 60)
        return false unless expires_at

        Time.now + buffer_seconds >= expires_at
      end
    end
  end
end
