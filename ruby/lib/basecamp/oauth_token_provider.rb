# frozen_string_literal: true

module Basecamp
  # A token provider that supports OAuth token refresh.
  #
  # @example
  #   provider = Basecamp::OauthTokenProvider.new(
  #     access_token: "current-token",
  #     refresh_token: "refresh-token",
  #     client_id: "your-client-id",
  #     client_secret: "your-client-secret"
  #   )
  class OauthTokenProvider
    include TokenProvider

    # Token endpoint for Basecamp OAuth
    TOKEN_URL = "https://launchpad.37signals.com/authorization/token"

    # @return [String, nil] the current refresh token
    attr_reader :refresh_token

    # @return [Time, nil] when the access token expires
    attr_reader :expires_at

    # Callback invoked when tokens are refreshed.
    # @return [Proc, nil]
    attr_accessor :on_refresh

    # @param access_token [String] current access token
    # @param refresh_token [String, nil] refresh token for renewal
    # @param client_id [String] OAuth client ID
    # @param client_secret [String] OAuth client secret
    # @param expires_at [Time, nil] token expiration time
    # @param on_refresh [Proc, nil] callback when tokens refresh
    def initialize(access_token:, client_id:, client_secret:, refresh_token: nil, expires_at: nil, on_refresh: nil)
      @access_token = access_token
      @refresh_token = refresh_token
      @client_id = client_id
      @client_secret = client_secret
      @expires_at = expires_at
      @on_refresh = on_refresh
      @mutex = Mutex.new
    end

    # Returns the current access token, refreshing if expired.
    # @return [String]
    def access_token
      @mutex.synchronize do
        refresh_if_needed
        @access_token
      end
    end

    # Refreshes the access token using the refresh token.
    # @return [Boolean] true if refresh succeeded
    def refresh
      @mutex.synchronize do
        return false unless refreshable?

        perform_refresh
      end
    end

    # @return [Boolean] true if refresh token is available
    def refreshable?
      @refresh_token && !@refresh_token.empty?
    end

    # @return [Boolean] true if the access token is expired
    def expired?
      @expires_at && Time.now >= @expires_at
    end

    private

      def refresh_if_needed
        perform_refresh if expired? && refreshable?
      end

      def perform_refresh
        require "faraday"
        require "json"
        require "uri"

        response = Faraday.post(TOKEN_URL) do |req|
          req.headers["Content-Type"] = "application/x-www-form-urlencoded"
          req.body = URI.encode_www_form(
            type: "refresh",
            refresh_token: @refresh_token,
            client_id: @client_id,
            client_secret: @client_secret
          )
        end

        raise AuthError.new("Token refresh failed: #{response.status}") unless response.success?

        data = JSON.parse(response.body)
        @access_token = data["access_token"]
        @expires_at = Time.now + data["expires_in"].to_i if data["expires_in"]

        @on_refresh&.call(@access_token, @refresh_token, @expires_at)

        true
      rescue Faraday::Error => e
        raise NetworkError.new("Token refresh network error", cause: e)
      end
  end
end
