# frozen_string_literal: true

require "faraday"
require "json"

module Basecamp
  module Oauth
    # SSRF-hardened fetch of a small OAuth discovery JSON document, shared by both
    # discovery hops (RFC 9728 resource metadata and RFC 8414 AS metadata).
    #
    # RFC 9728 §7.7 flags SSRF via attacker-influenced metadata: advertised AS
    # URLs are untrusted input. Every fetch therefore:
    #
    # 1. requires HTTPS (localhost exempt) — enforced by the origin-root profile
    #    ({Basecamp::Security.require_origin_root!}) before this is called;
    # 2. bounds the timeout (set on the injected connection);
    # 3. suppresses redirects — the default Faraday connection carries no redirect
    #    middleware, so an attacker-controlled 3xx +Location+ is surfaced as a
    #    non-2xx +api_error+ rather than chased;
    # 4. reads the body under a genuine bounded/streaming cap that aborts the read
    #    the moment the accumulated size exceeds the limit (via Faraday's +on_data+
    #    streaming callback) — NOT a post-hoc size check on an already-buffered
    #    body.
    #
    # Non-2xx on either hop maps to +api_error+ (not +network+).
    module Fetcher
      # Discovery documents are tiny; cap the read at 1 MiB by default.
      DEFAULT_MAX_BODY_BYTES = 1 * 1024 * 1024

      # Raised internally to abort a streaming read once the cap is exceeded.
      # Never escapes this module — it is mapped to an OauthError.
      class BodyTooLarge < StandardError; end

      # Builds a +[chunks, on_data]+ pair for a genuine bounded/streaming read.
      # Assign +on_data+ to a request's +req.options.on_data+; after the request
      # returns, +chunks.join+ is the accumulated body. The proc raises
      # {BodyTooLarge} the moment the accumulated size exceeds +max_body_bytes+,
      # so an oversized body is never fully buffered. Callers rescue
      # {BodyTooLarge} and map it to their own error. Shared by both discovery
      # hops and the device flow so every OAuth response reads under the same cap.
      #
      # @param max_body_bytes [Integer] bounded read cap in bytes
      # @return [Array(Array<String>, Proc)] the chunk buffer and the +on_data+ proc
      def self.bounded_reader(max_body_bytes)
        chunks = []
        total = 0
        reader = proc do |chunk, _received|
          total += chunk.bytesize
          raise BodyTooLarge if total > max_body_bytes

          chunks << chunk
        end
        [ chunks, reader ]
      end

      # Builds the default SSRF-hardened Faraday connection. No redirect
      # middleware is registered, so redirects are not followed.
      #
      # @param timeout [Integer] request + connect timeout in seconds
      # @return [Faraday::Connection]
      def self.build_client(timeout)
        Faraday.new do |conn|
          conn.options.timeout = timeout
          conn.options.open_timeout = timeout
          conn.adapter Faraday.default_adapter
        end
      end

      # Rejects an INJECTED connection whose middleware stack we cannot verify to
      # be redirect-free. Redirect suppression is a load-bearing SSRF control (RFC
      # 9728 §7.7): a caller-supplied client that follows redirects would silently
      # chase an attacker-controlled +Location+. A class-NAME heuristic (matching
      # +/redirect/+) is bypassable by a follower whose class name does not contain
      # "redirect", so we enforce a POLICY instead of guessing by name: an injected
      # connection may carry ONLY adapter handlers. The default {build_client}
      # connection (adapter only) and a test's mock adapter qualify; ANY request/
      # response middleware — which could follow redirects under any name, or
      # otherwise rewrite the request — is refused rather than trusted.
      #
      # @param client [Faraday::Connection]
      # @raise [OauthError] +validation+ when non-adapter middleware is present
      def self.ensure_redirects_suppressed!(client)
        handlers = client.respond_to?(:builder) ? Array(client.builder.handlers) : []
        offending = handlers.find do |h|
          h.respond_to?(:klass) && h.klass.is_a?(Class) && !(h.klass <= Faraday::Adapter)
        end
        return unless offending

        raise OauthError.new(
          "validation",
          "Injected OAuth discovery client must carry only an adapter (no middleware); " \
          "found #{offending.klass.name}. Redirects are suppressed for SSRF safety, so a " \
          "connection whose middleware stack cannot be verified redirect-free is refused"
        )
      end

      # Fetches +url+ and returns the parsed JSON object (a Hash).
      #
      # The request timeout is applied per-request (not only on the connection)
      # so a bounded read is enforced even when the caller INJECTS its own
      # connection: an injected client's adapter default would otherwise leave the
      # requested +timeout+ unenforced. This mirrors the device flow's +post_form+.
      #
      # @param http_client [Faraday::Connection] the SSRF-hardened connection
      # @param url [String] fully-qualified well-known URL to fetch
      # @param timeout [Integer] per-request timeout in seconds
      # @param max_body_bytes [Integer] bounded read cap in bytes
      # @return [Hash] the parsed JSON document
      # @raise [OauthError] +api_error+ on non-2xx, oversized body, non-object
      #   JSON, or parse failure; +network+ on transport failure
      def self.fetch_json(http_client, url, timeout:, max_body_bytes: DEFAULT_MAX_BODY_BYTES)
        chunks, on_data = bounded_reader(max_body_bytes)

        response = http_client.get(url) do |req|
          req.headers["Accept"] = "application/json"
          # Bounded streaming read: abort the moment the cap is exceeded so an
          # oversized body is never fully buffered.
          req.options.on_data = on_data
          # Apply the request timeout on every request — even an injected client —
          # so a stalled socket can't hang discovery under the adapter default.
          req.options.timeout = timeout
          req.options.open_timeout = timeout
        end

        body = chunks.join.force_encoding(Encoding::UTF_8)

        unless (200..299).cover?(response.status)
          raise OauthError.new(
            "api_error",
            "OAuth discovery failed with status #{response.status}: #{Basecamp::Security.truncate(body)}",
            http_status: response.status
          )
        end

        data = JSON.parse(body)
        raise OauthError.new("api_error", "OAuth discovery response is not a JSON object") unless data.is_a?(Hash)

        data
      rescue BodyTooLarge
        raise OauthError.new("api_error", "OAuth discovery response exceeds size cap")
      rescue Faraday::Error => e
        raise OauthError.new("network", "OAuth discovery failed: #{e.message}", retryable: true)
      rescue JSON::ParserError => e
        raise OauthError.new("api_error", "Failed to parse OAuth discovery response: #{e.message}")
      end
    end
  end
end
