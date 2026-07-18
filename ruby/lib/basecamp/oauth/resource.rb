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
        # Normalize before building the client and before the fetch computes its
        # wall-clock deadline: a non-finite/non-positive timeout must not disable
        # either bound (see Fetcher.normalize_timeout).
        @timeout = Fetcher.normalize_timeout(timeout)
        @http_client = http_client || Fetcher.build_client(@timeout)
        # Normalize the public cap to a finite non-negative Integer: a nil, float,
        # or Float::INFINITY would otherwise disable the streaming memory bound
        # (an infinite/undefined cap never trips), reintroducing an SSRF/OOM risk.
        # Shared with Discovery and the device flow.
        @max_body_bytes = Fetcher.normalize_body_cap(max_body_bytes)
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

        # Bind the resource identifier to the requested identifier (the raw caller
        # origin), code-point exact, NO normalization (RFC 9728 §3.3, SPEC.md §16):
        # the well-known URL is built from the normalized +origin+, but the metadata
        # +resource+ must be identical to what the caller supplied.
        unless resource == resource_origin
          raise OauthError.new(
            "api_error",
            "Resource identifier mismatch: metadata resource #{resource.inspect} does not equal #{resource_origin.inspect}"
          )
        end

        ProtectedResourceMetadata.new(
          resource: resource,
          authorization_servers: extract_authorization_servers(data)
        )
      end

      private

        # Preserve absent (+nil+) vs present-empty (+[]+). A present value that is
        # not an array of strings — including a JSON +null+ — is malformed metadata
        # and is rejected (→ soft resource_discovery_failed in the orchestrator),
        # never iterated (a bare string would otherwise be treated as a sequence of
        # single-character issuers during selection) and never normalized to +[]+.
        def extract_authorization_servers(data)
          return nil unless data.key?("authorization_servers")

          servers = data["authorization_servers"]
          # A present JSON null (or any non-array-of-strings) is MALFORMED metadata,
          # not "present but empty": it must fail hop-1 (→ soft resource_discovery_failed
          # in the orchestrator), never be normalized to [] and read as no_as_advertised.
          # An empty array is a valid "present but empty" value and is preserved.
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
