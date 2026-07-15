# frozen_string_literal: true

module Basecamp
  module Oauth
    # Fetches RFC 8414 OAuth 2 Authorization Server Metadata and binds the
    # returned +issuer+ to the requested issuer origin (hop 2 of resource-first
    # discovery).
    class Discovery
      # Structured marker: AS metadata failed the RFC 8414 issuer code-point
      # bind. Raised (never message-matched) so {Oauth.discover_from_resource}
      # classifies an issuer mismatch by CLASS via {Oauth.as_failure_error} —
      # brittle, locale-sensitive substring matching is gone. Kept in the
      # discovery layer, deliberately NOT in the device error files that
      # {DeviceFlowError} shares.
      class IssuerBindingError < OauthError
        def initialize(message, http_status: nil)
          super("api_error", message, http_status: http_status)
        end
      end

      # @param http_client [Faraday::Connection, nil] HTTP client (SSRF-hardened default if nil)
      # @param timeout [Integer] request timeout in seconds (default: 10)
      # @param max_body_bytes [Integer] bounded read cap in bytes
      def initialize(http_client: nil, timeout: 10, max_body_bytes: Fetcher::DEFAULT_MAX_BODY_BYTES)
        Fetcher.ensure_redirects_suppressed!(http_client) if http_client
        @http_client = http_client || Fetcher.build_client(timeout)
        @timeout = timeout
        @max_body_bytes = max_body_bytes
      end

      # Discovers OAuth configuration from
      # <tt>{base_url}/.well-known/oauth-authorization-server</tt>, binding the
      # returned +issuer+ to +base_url+ by code-point (RFC 8414 §3.3/§4, no
      # normalization beyond origin-root parsing). +token_endpoint+ is required;
      # +authorization_endpoint+ is optional (device-only servers omit it).
      #
      # @param base_url [String] the OAuth server's issuer origin
      # @return [Config] the OAuth server configuration
      # @raise [Basecamp::UsageError] on a malformed origin
      # @raise [OauthError] +api_error+ on invalid metadata / issuer mismatch
      #
      # @example
      #   config = Basecamp::Oauth::Discovery.new.discover("https://launchpad.37signals.com")
      #   config.token_endpoint # => "https://launchpad.37signals.com/authorization/token"
      def discover(base_url)
        issuer_origin = Basecamp::Security.require_origin_root!(base_url, "OAuth discovery base URL")
        discovery_url = "#{issuer_origin}/.well-known/oauth-authorization-server"
        data = Fetcher.fetch_json(@http_client, discovery_url, timeout: @timeout, max_body_bytes: @max_body_bytes)
        parse_and_bind(data, issuer_origin)
      end

      private

        # Universal validation only: +issuer+ + +token_endpoint+ present and
        # non-empty, issuer identical by code-point, and any present +*_endpoint+
        # field non-empty. Per-grant endpoint checks are the consumer's job.
        def parse_and_bind(data, expected_issuer_origin)
          issuer = data["issuer"]
          if !issuer.is_a?(String) || issuer.empty?
            raise OauthError.new("api_error", "Invalid OAuth discovery response: missing required fields: issuer")
          end

          # RFC 8414 §3.3/§4: issuer identical by code-point. No normalization.
          unless issuer == expected_issuer_origin
            raise IssuerBindingError.new(
              "OAuth issuer mismatch: metadata issuer #{issuer.inspect} does not equal #{expected_issuer_origin.inspect}"
            )
          end

          token_endpoint = data["token_endpoint"]
          if !token_endpoint.is_a?(String) || token_endpoint.empty?
            raise OauthError.new("api_error", "Invalid OAuth discovery response: missing required fields: token_endpoint")
          end

          reject_empty_endpoints!(data)
          validate_string_array!(data, "grant_types_supported")
          validate_string_array!(data, "scopes_supported")

          Config.new(
            issuer: issuer,
            authorization_endpoint: data["authorization_endpoint"],
            token_endpoint: token_endpoint,
            device_authorization_endpoint: data["device_authorization_endpoint"],
            registration_endpoint: data["registration_endpoint"],
            scopes_supported: data["scopes_supported"],
            grant_types_supported: data["grant_types_supported"]
          )
        end

        # Any endpoint field that IS present must be a non-empty String: "" is a
        # truthy value in Ruby and must be rejected, and a non-string endpoint
        # (array/number/object) is malformed metadata.
        def reject_empty_endpoints!(data)
          data.each do |key, value|
            next unless key.end_with?("_endpoint") && !value.nil?

            unless value.is_a?(String) && !value.empty?
              raise OauthError.new("api_error", "Invalid OAuth discovery response: invalid #{key}")
            end
          end
        end

        # A metadata list field (e.g. +grant_types_supported+, +scopes_supported+),
        # when present, must be an array of strings. A bare string must never be
        # accepted: substring-matching +grant_types_supported+ could falsely enable
        # a grant such as device_code, and a non-array is malformed metadata.
        def validate_string_array!(data, key)
          value = data[key]
          return if value.nil?

          unless value.is_a?(Array) && value.all?(String)
            raise OauthError.new(
              "api_error",
              "Invalid OAuth discovery response: #{key} must be an array of strings"
            )
          end
        end
    end
  end
end
