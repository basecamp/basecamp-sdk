# frozen_string_literal: true

require "faraday"
require "json"

module Basecamp
  module Oauth
    # Default Basecamp/Launchpad OAuth server URL.
    LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com"

    # Discoverer fetches OAuth 2.0 server configuration from discovery endpoints.
    class Discoverer
      # @param http_client [Faraday::Connection, nil] HTTP client (uses default if nil)
      # @param timeout [Integer] Request timeout in seconds (default: 10)
      def initialize(http_client: nil, timeout: 10)
        @http_client = http_client || build_default_client(timeout)
      end

      # Discovers OAuth configuration from the well-known endpoint.
      #
      # Fetches the OAuth 2.0 Authorization Server Metadata from:
      # `{base_url}/.well-known/oauth-authorization-server`
      #
      # @param base_url [String] The OAuth server's base URL (e.g., "https://launchpad.37signals.com")
      # @return [Config] The OAuth server configuration
      # @raise [OAuthError] on network or parsing errors
      #
      # @example
      #   discoverer = Basecamp::Oauth::Discoverer.new
      #   config = discoverer.discover("https://launchpad.37signals.com")
      #   puts config.token_endpoint
      #   # => "https://launchpad.37signals.com/authorization/token"
      def discover(base_url)
        normalized_base = base_url.chomp("/")
        discovery_url = "#{normalized_base}/.well-known/oauth-authorization-server"

        response = @http_client.get(discovery_url) do |req|
          req.headers["Accept"] = "application/json"
        end

        unless response.success?
          raise OAuthError.new(
            "network",
            "OAuth discovery failed with status #{response.status}: #{response.body}",
            http_status: response.status
          )
        end

        data = JSON.parse(response.body)
        validate_discovery_response!(data)

        Config.new(
          issuer: data["issuer"],
          authorization_endpoint: data["authorization_endpoint"],
          token_endpoint: data["token_endpoint"],
          registration_endpoint: data["registration_endpoint"],
          scopes_supported: data["scopes_supported"]
        )
      rescue Faraday::Error => e
        raise OAuthError.new("network", "OAuth discovery failed: #{e.message}", retryable: true)
      rescue JSON::ParserError => e
        raise OAuthError.new("api_error", "Failed to parse discovery response: #{e.message}")
      end

      private

      def build_default_client(timeout)
        Faraday.new do |conn|
          conn.options.timeout = timeout
          conn.options.open_timeout = timeout
          conn.adapter Faraday.default_adapter
        end
      end

      def validate_discovery_response!(data)
        missing = []
        missing << "issuer" unless data["issuer"]
        missing << "authorization_endpoint" unless data["authorization_endpoint"]
        missing << "token_endpoint" unless data["token_endpoint"]

        return if missing.empty?

        raise OAuthError.new(
          "api_error",
          "Invalid OAuth discovery response: missing required fields: #{missing.join(', ')}"
        )
      end
    end

    module_function

    # Discovers OAuth configuration from the well-known endpoint.
    #
    # @param base_url [String] The OAuth server's base URL
    # @param timeout [Integer] Request timeout in seconds (default: 10)
    # @return [Config] The OAuth server configuration
    #
    # @example
    #   config = Basecamp::Oauth.discover("https://launchpad.37signals.com")
    def discover(base_url, timeout: 10)
      Discoverer.new(timeout: timeout).discover(base_url)
    end

    # Discovers OAuth configuration from Basecamp's Launchpad server.
    #
    # Convenience function that calls discover() with the Launchpad base URL.
    #
    # @param timeout [Integer] Request timeout in seconds (default: 10)
    # @return [Config] The OAuth server configuration
    #
    # @example
    #   config = Basecamp::Oauth.discover_launchpad
    #   # Use config.authorization_endpoint to start OAuth flow
    def discover_launchpad(timeout: 10)
      discover(LAUNCHPAD_BASE_URL, timeout: timeout)
    end
  end
end
