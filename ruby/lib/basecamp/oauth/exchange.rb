# frozen_string_literal: true

require "faraday"
require "json"
require "uri"

module Basecamp
  # OAuth 2 helpers for Basecamp authentication.
  module Oauth
    # Exchanger handles OAuth 2 token exchange and refresh operations.
    class Exchanger
      # @param http_client [Faraday::Connection, nil] HTTP client (uses default if nil)
      # @param timeout [Integer] Request timeout in seconds (default: 30)
      def initialize(http_client: nil, timeout: 30)
        @http_client = http_client || build_default_client(timeout)
      end

      # Exchanges an authorization code for access and refresh tokens.
      #
      # Supports both standard OAuth 2 and Basecamp's Launchpad legacy format.
      # Use `use_legacy_format: true` for Launchpad compatibility.
      #
      # @param request [ExchangeRequest] Exchange request parameters
      # @return [Token] The token response
      # @raise [OAuthError] on validation, network, or authentication errors
      #
      # @example Standard OAuth 2
      #   token = exchanger.exchange(ExchangeRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     code: "auth_code_from_callback",
      #     redirect_uri: "https://myapp.com/callback",
      #     client_id: "my_client_id",
      #     client_secret: "my_client_secret"
      #   ))
      #
      # @example Launchpad legacy format
      #   token = exchanger.exchange(ExchangeRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     code: "auth_code",
      #     redirect_uri: "https://myapp.com/callback",
      #     client_id: "my_client_id",
      #     client_secret: "my_client_secret",
      #     use_legacy_format: true
      #   ))
      def exchange(request)
        validate_exchange_request!(request)

        params = build_exchange_params(request)
        do_token_request(request.token_endpoint, params)
      end

      # Refreshes an access token using a refresh token.
      #
      # Supports both standard OAuth 2 and Basecamp's Launchpad legacy format.
      # Use `use_legacy_format: true` for Launchpad compatibility.
      #
      # @param request [RefreshRequest] Refresh request parameters
      # @return [Token] The new token response
      # @raise [OAuthError] on validation, network, or authentication errors
      #
      # @example Standard OAuth 2
      #   new_token = exchanger.refresh(RefreshRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     refresh_token: old_token.refresh_token,
      #     client_id: "my_client_id",
      #     client_secret: "my_client_secret"
      #   ))
      #
      # @example Launchpad legacy format
      #   new_token = exchanger.refresh(RefreshRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     refresh_token: old_token.refresh_token,
      #     use_legacy_format: true
      #   ))
      def refresh(request)
        validate_refresh_request!(request)

        params = build_refresh_params(request)
        do_token_request(request.token_endpoint, params)
      end

      private

      def build_default_client(timeout)
        Faraday.new do |conn|
          conn.options.timeout = timeout
          conn.options.open_timeout = timeout
          conn.adapter Faraday.default_adapter
        end
      end

      def validate_exchange_request!(request)
        raise OAuthError.new("validation", "Token endpoint is required") if request.token_endpoint.to_s.empty?
        raise OAuthError.new("validation", "Authorization code is required") if request.code.to_s.empty?
        raise OAuthError.new("validation", "Redirect URI is required") if request.redirect_uri.to_s.empty?
        raise OAuthError.new("validation", "Client ID is required") if request.client_id.to_s.empty?
      end

      def validate_refresh_request!(request)
        raise OAuthError.new("validation", "Token endpoint is required") if request.token_endpoint.to_s.empty?
        raise OAuthError.new("validation", "Refresh token is required") if request.refresh_token.to_s.empty?
      end

      def build_exchange_params(request)
        params = {}

        if request.use_legacy_format
          # Launchpad uses non-standard "type" parameter
          params["type"] = "web_server"
        else
          # Standard OAuth 2
          params["grant_type"] = "authorization_code"
        end

        params["code"] = request.code
        params["redirect_uri"] = request.redirect_uri
        params["client_id"] = request.client_id
        params["client_secret"] = request.client_secret if request.client_secret
        params["code_verifier"] = request.code_verifier if request.code_verifier

        params
      end

      def build_refresh_params(request)
        params = {}

        if request.use_legacy_format
          # Launchpad uses non-standard "type" parameter
          params["type"] = "refresh"
        else
          # Standard OAuth 2
          params["grant_type"] = "refresh_token"
        end

        params["refresh_token"] = request.refresh_token
        params["client_id"] = request.client_id if request.client_id
        params["client_secret"] = request.client_secret if request.client_secret

        params
      end

      def do_token_request(token_endpoint, params)
        response = @http_client.post(token_endpoint) do |req|
          req.headers["Content-Type"] = "application/x-www-form-urlencoded"
          req.headers["Accept"] = "application/json"
          req.body = URI.encode_www_form(params)
        end

        parse_token_response(response)
      rescue Faraday::TimeoutError
        raise OAuthError.new("network", "Token request timed out", retryable: true)
      rescue Faraday::Error => e
        raise OAuthError.new("network", "Token request failed: #{e.message}", retryable: true)
      end

      def parse_token_response(response)
        data = JSON.parse(response.body)

        handle_error_response(response.status, data) unless response.success?

        raise OAuthError.new("api_error", "Token response missing access_token") unless data["access_token"]

        Token.new(
          access_token: data["access_token"],
          refresh_token: data["refresh_token"],
          token_type: data["token_type"] || "Bearer",
          expires_in: data["expires_in"],
          scope: data["scope"]
        )
      rescue JSON::ParserError
        raise OAuthError.new(
          "api_error",
          "Failed to parse token response: #{response.body}",
          http_status: response.status
        )
      end

      def handle_error_response(status, data)
        error_msg = data["error_description"] || data["error"] || "Token request failed"

        if status == 401 || data["error"] == "invalid_grant"
          raise OAuthError.new(
            "auth",
            error_msg,
            http_status: status,
            hint: "The authorization code or refresh token may be invalid or expired"
          )
        end

        raise OAuthError.new("api_error", error_msg, http_status: status)
      end
    end

    module_function

    # Exchanges an authorization code for access and refresh tokens.
    #
    # @param token_endpoint [String] URL of the token endpoint
    # @param code [String] The authorization code
    # @param redirect_uri [String] The redirect URI used in the authorization request
    # @param client_id [String] The client identifier
    # @param client_secret [String, nil] The client secret
    # @param code_verifier [String, nil] PKCE code verifier
    # @param use_legacy_format [Boolean] Use Launchpad's non-standard format
    # @param timeout [Integer] Request timeout in seconds (default: 30)
    # @return [Token] The token response
    #
    # @example
    #   token = Basecamp::Oauth.exchange_code(
    #     token_endpoint: config.token_endpoint,
    #     code: "auth_code",
    #     redirect_uri: "https://myapp.com/callback",
    #     client_id: ENV["BASECAMP_CLIENT_ID"],
    #     client_secret: ENV["BASECAMP_CLIENT_SECRET"],
    #     use_legacy_format: true
    #   )
    def exchange_code(
      token_endpoint:,
      code:,
      redirect_uri:,
      client_id:,
      client_secret: nil,
      code_verifier: nil,
      use_legacy_format: false,
      timeout: 30
    )
      request = ExchangeRequest.new(
        token_endpoint: token_endpoint,
        code: code,
        redirect_uri: redirect_uri,
        client_id: client_id,
        client_secret: client_secret,
        code_verifier: code_verifier,
        use_legacy_format: use_legacy_format
      )
      Exchanger.new(timeout: timeout).exchange(request)
    end

    # Refreshes an access token using a refresh token.
    #
    # @param token_endpoint [String] URL of the token endpoint
    # @param refresh_token [String] The refresh token
    # @param client_id [String, nil] The client identifier
    # @param client_secret [String, nil] The client secret
    # @param use_legacy_format [Boolean] Use Launchpad's non-standard format
    # @param timeout [Integer] Request timeout in seconds (default: 30)
    # @return [Token] The new token response
    #
    # @example
    #   new_token = Basecamp::Oauth.refresh_token(
    #     token_endpoint: config.token_endpoint,
    #     refresh_token: old_token.refresh_token,
    #     use_legacy_format: true
    #   )
    def refresh_token(
      token_endpoint:,
      refresh_token:,
      client_id: nil,
      client_secret: nil,
      use_legacy_format: false,
      timeout: 30
    )
      request = RefreshRequest.new(
        token_endpoint: token_endpoint,
        refresh_token: refresh_token,
        client_id: client_id,
        client_secret: client_secret,
        use_legacy_format: use_legacy_format
      )
      Exchanger.new(timeout: timeout).refresh(request)
    end

    # Checks if a token is expired or about to expire.
    #
    # @param token [Token] The token to check
    # @param buffer_seconds [Integer] Buffer time before actual expiration (default: 60)
    # @return [Boolean] true if expired or will expire within buffer time
    #
    # @example
    #   if Basecamp::Oauth.token_expired?(token)
    #     token = Basecamp::Oauth.refresh_token(...)
    #   end
    def token_expired?(token, buffer_seconds = 60)
      token.expired?(buffer_seconds)
    end
  end
end
