# frozen_string_literal: true

require_relative "oauth/errors"
require_relative "oauth/types"
require_relative "oauth/discovery"
require_relative "oauth/exchange"

module Basecamp
  # OAuth 2.0 module for Basecamp SDK.
  #
  # Provides OAuth discovery, token exchange, and token refresh functionality.
  # Supports both standard OAuth 2.0 and Basecamp's Launchpad legacy format.
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
    # Re-export constants
    # @return [String] Default Launchpad base URL
    # LAUNCHPAD_BASE_URL is defined in discovery.rb
  end
end
