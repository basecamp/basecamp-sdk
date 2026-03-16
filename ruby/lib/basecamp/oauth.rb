# frozen_string_literal: true

module Basecamp
  # OAuth 2 module for Basecamp SDK.
  #
  # Provides OAuth discovery, token exchange, and token refresh functionality.
  # Supports both standard OAuth 2 and Basecamp's Launchpad legacy format.
  #
  # @example Complete OAuth flow
  #   # 1. Discover OAuth configuration
  #   config = Basecamp::Oauth.discover_launchpad
  #
  #   # 2. Build authorization URL (redirect user here)
  #   auth_url = "#{config.authorization_endpoint}?" + URI.encode_www_form(
  #     type: "web_server",
  #     client_id: ENV["BASECAMP_CLIENT_ID"],
  #     redirect_uri: "https://myapp.com/callback"
  #   )
  #
  #   # 3. Exchange authorization code for tokens (in callback handler)
  #   token = Basecamp::Oauth.exchange_code(
  #     token_endpoint: config.token_endpoint,
  #     code: params[:code],
  #     redirect_uri: "https://myapp.com/callback",
  #     client_id: ENV["BASECAMP_CLIENT_ID"],
  #     client_secret: ENV["BASECAMP_CLIENT_SECRET"],
  #     use_legacy_format: true  # Required for Launchpad
  #   )
  #
  #   # 4. Use the token
  #   client = Basecamp.client(
  #     access_token: token.access_token,
  #     account_id: "12345"
  #   )
  #
  #   # 5. Refresh when needed
  #   if token.expired?
  #     token = Basecamp::Oauth.refresh_token(
  #       token_endpoint: config.token_endpoint,
  #       refresh_token: token.refresh_token,
  #       use_legacy_format: true
  #     )
  #   end
  #
  # @see https://github.com/basecamp/api/blob/master/sections/authentication.md
  module Oauth
    LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com"

    def self.discover(base_url, timeout: 10)
      Discovery.new(timeout: timeout).discover(base_url)
    end

    def self.discover_launchpad(timeout: 10)
      discover(LAUNCHPAD_BASE_URL, timeout: timeout)
    end

    def self.exchange_code(
      token_endpoint:, code:, redirect_uri:, client_id:,
      client_secret: nil, code_verifier: nil,
      use_legacy_format: false, timeout: 30
    )
      request = ExchangeRequest.new(
        token_endpoint: token_endpoint, code: code,
        redirect_uri: redirect_uri, client_id: client_id,
        client_secret: client_secret, code_verifier: code_verifier,
        use_legacy_format: use_legacy_format
      )
      Exchange.new(timeout: timeout).exchange(request)
    end

    def self.refresh_token(
      token_endpoint:, refresh_token:,
      client_id: nil, client_secret: nil,
      use_legacy_format: false, timeout: 30
    )
      request = RefreshRequest.new(
        token_endpoint: token_endpoint, refresh_token: refresh_token,
        client_id: client_id, client_secret: client_secret,
        use_legacy_format: use_legacy_format
      )
      Exchange.new(timeout: timeout).refresh(request)
    end

    def self.token_expired?(token, buffer_seconds = 60)
      token.expired?(buffer_seconds)
    end
  end
end
