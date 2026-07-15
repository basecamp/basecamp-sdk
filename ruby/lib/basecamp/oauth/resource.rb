# frozen_string_literal: true

module Basecamp
  module Oauth
    # Fetches RFC 9728 protected-resource metadata (hop 1 of resource-first
    # discovery) and binds the returned +resource+ to the requested origin.
    class Resource
      # @param http_client [Faraday::Connection, nil] HTTP client (SSRF-hardened default if nil)
      # @param timeout [Integer] request timeout in seconds (default: 10)
      # @param max_body_bytes [Integer] bounded read cap in bytes
      def initialize(http_client: nil, timeout: 10, max_body_bytes: Fetcher::DEFAULT_MAX_BODY_BYTES)
        Fetcher.ensure_redirects_suppressed!(http_client) if http_client
        @http_client = http_client || Fetcher.build_client(timeout)
        @timeout = timeout
        @max_body_bytes = max_body_bytes
      end

      # Discovers protected-resource metadata from
      # <tt>{resource_origin}/.well-known/oauth-protected-resource</tt>.
      # +resource+ is required and must equal the requested origin by code-point;
      # +authorization_servers+ is preserved distinctly as absent (+nil+) vs
      # present-empty (+[]+).
      #
      # @param resource_origin [String] the API/resource host origin
      # @return [ProtectedResourceMetadata]
      # @raise [Basecamp::UsageError] on a malformed caller origin
      # @raise [OauthError] +api_error+ on invalid metadata / resource mismatch
      def discover(resource_origin)
        origin = Basecamp::Security.require_origin_root!(resource_origin, "resource origin")
        url = "#{origin}/.well-known/oauth-protected-resource"
        data = Fetcher.fetch_json(@http_client, url, timeout: @timeout, max_body_bytes: @max_body_bytes)

        resource = data["resource"]
        # Type-check, don't just probe truthiness: a wrong-typed resource (number,
        # object, array) is malformed metadata, and calling +.empty?+ on it would
        # raise a NoMethodError rather than a clean api_error.
        if !resource.is_a?(String) || resource.empty?
          raise OauthError.new("api_error", "Invalid resource metadata: missing required field: resource")
        end

        # Bind the resource identifier to the requested origin, code-point exact.
        unless resource == origin
          raise OauthError.new(
            "api_error",
            "Resource identifier mismatch: metadata resource #{resource.inspect} does not equal #{origin.inspect}"
          )
        end

        ProtectedResourceMetadata.new(
          resource: resource,
          authorization_servers: extract_authorization_servers(data)
        )
      end

      private

        # Preserve absent (+nil+) vs present-empty (+[]+); a present +null+ is
        # normalized to +[]+ to match the "present but empty" posture. A present
        # value that is not an array of strings is malformed metadata and must be
        # rejected — never iterated (a bare string would otherwise be treated as a
        # sequence of single-character issuers during selection).
        def extract_authorization_servers(data)
          return nil unless data.key?("authorization_servers")

          servers = data["authorization_servers"]
          return [] if servers.nil?

          unless servers.is_a?(Array) && servers.all?(String)
            raise OauthError.new(
              "api_error",
              "Invalid resource metadata: authorization_servers must be an array of strings"
            )
          end

          servers
        end
    end
  end
end
