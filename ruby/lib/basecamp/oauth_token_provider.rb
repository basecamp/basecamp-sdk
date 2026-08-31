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

    # The redirect statuses a token endpoint response is refused for
    # (SPEC §16 "Token-Endpoint Transport Policy") — the refresh POST carries
    # the refresh token and client secret, and a redirect must surface as a
    # typed fault rather than re-issue those credentials toward Location.
    # 304 stays on the generic non-success path (a cache validator, not a
    # redirect-with-Location).
    REDIRECT_STATUSES = [ 301, 302, 303, 307, 308 ].freeze

    # Whole-request bound in seconds for the refresh POST — the shared
    # credential-POST default (SPEC §16). Enforced as socket timeouts AND a
    # monotonic wall-clock deadline by the transport below.
    REFRESH_TIMEOUT = 30

    # Cap on a refresh response body (1 MiB), matching the exchange path.
    MAX_RESPONSE_BYTES = 1 * 1024 * 1024

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

      # The refresh POST runs on the headers-first {Oauth::Fetcher.stream_http}
      # primitive — the same transport as the exchange and device paths — so it
      # gets the full SPEC §16 discipline rather than a bare Faraday.post:
      # redirects structurally never followed and classified at header time,
      # socket timeouts plus a monotonic whole-request watchdog (a slow-drip
      # peer cannot hold the refresh open past REFRESH_TIMEOUT), and a bounded
      # streaming body read.
      def perform_refresh
        require "faraday"
        require "json"

        status, body = Oauth::Fetcher.stream_http(
          :post, TOKEN_URL,
          headers: { "Content-Type" => "application/x-www-form-urlencoded" },
          form: {
            "type" => "refresh",
            "refresh_token" => @refresh_token,
            "client_id" => @client_id,
            "client_secret" => @client_secret
          },
          timeout: REFRESH_TIMEOUT,
          max_body_bytes: MAX_RESPONSE_BYTES,
          skip_status: ->(s) { REDIRECT_STATUSES.include?(s) }
        )

        # A refused redirect is a typed api fault carrying the real status —
        # not the generic AuthError below, which would imply the credentials
        # were judged and rejected when no such judgement happened.
        if REDIRECT_STATUSES.include?(status)
          raise ApiError.new("redirect #{status} on the token endpoint is not followed", http_status: status)
        end
        raise AuthError.new("Token refresh failed: #{status}") unless (200..299).cover?(status)

        data = JSON.parse(body)
        @access_token = data["access_token"]
        @expires_at = Time.now + data["expires_in"].to_i if data["expires_in"]

        @on_refresh&.call(@access_token, @refresh_token, @expires_at)

        true
      rescue Oauth::Fetcher::BodyTooLarge
        raise ApiError.new("Token refresh response exceeds size cap")
      rescue Oauth::Fetcher::ReadDeadlineExceeded => e
        # A slow-drip read past the deadline is a transport timeout.
        raise NetworkError.new("Token refresh network error", cause: e)
      rescue Faraday::Error => e
        raise NetworkError.new("Token refresh network error", cause: e)
      end
  end
end
