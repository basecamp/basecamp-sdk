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
    # 2. bounds the timeout — both a per-read socket timeout AND a monotonic
    #    wall-clock deadline over the whole streaming read (a per-read timeout
    #    alone resets on every chunk, so a slow-drip peer could hang the fetch);
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

      # Default request timeout in seconds when a caller supplies none or an
      # invalid one.
      DEFAULT_TIMEOUT = 10

      # Coerce the public timeout to a finite, positive numeric. A nil, non-numeric,
      # non-positive, or +Float::INFINITY+/+NaN+ value would otherwise disable BOTH
      # the socket timeout and the wall-clock deadline in {fetch_json} (+now + inf+
      # never trips), letting a slow-drip peer hold the fetch open indefinitely.
      # Mirrors the +max_body_bytes+ normalization in the discovery initializers.
      #
      # @param timeout [Object] caller-supplied timeout
      # @return [Numeric] a finite, positive timeout in seconds
      # +default+ is operation-specific: discovery falls back to +DEFAULT_TIMEOUT+
      # (10s), device flow passes its own 30s budget, so an invalid runtime value
      # falls back to that operation's own timeout rather than a foreign one.
      def self.normalize_timeout(timeout, default: DEFAULT_TIMEOUT)
        return timeout if valid_timeout?(timeout)
        # Validate the fallback too: a caller passing an invalid +default+ must not
        # be able to disable both timeout bounds. Fall back to the finite constant.
        return default if valid_timeout?(default)

        DEFAULT_TIMEOUT
      end

      # +real?+ gates out Complex before +finite?+/+positive?+ (which Complex does
      # not define — calling them would raise NoMethodError). Integer, Float, and
      # Rational are all real and answer both.
      def self.valid_timeout?(value)
        value.is_a?(Numeric) && value.real? && value.finite? && value.positive?
      end

      # Coerce the public body cap to a non-negative Integer. A nil, non-Integer
      # (+Float::INFINITY+ included), or negative value would disable the streaming
      # memory bound (+total > cap+ never trips), defeating the bounded-read
      # guarantee. This is the one shared policy for Discovery, Resource, and the
      # device flow; the +default+ is validated too, so an invalid fallback cannot
      # disable the bound either.
      def self.normalize_body_cap(cap, default: DEFAULT_MAX_BODY_BYTES)
        return cap if valid_body_cap?(cap)
        return default if valid_body_cap?(default)

        DEFAULT_MAX_BODY_BYTES
      end

      def self.valid_body_cap?(value)
        value.is_a?(Integer) && value >= 0
      end

      # Raised internally to abort a streaming read once the cap is exceeded.
      # Never escapes this module — it is mapped to an OauthError.
      class BodyTooLarge < StandardError; end

      # Raised internally when a streaming read exceeds its wall-clock deadline.
      # Never escapes this module — it is mapped to a retryable +network+ OauthError.
      class ReadDeadlineExceeded < StandardError; end

      # Raised from +on_data+ to STOP reading a response whose body the caller does
      # not use (a non-2xx device-auth, a 3xx token redirect). Draining a slow such
      # body would otherwise time out and be misclassified as a transport failure.
      # Carries the response status so the caller can classify by it. Never escapes.
      class SkipBody < StandardError
        attr_reader :status

        def initialize(status)
          @status = status
          super("device flow response body skipped for status #{status}")
        end
      end

      # Builds a +[chunks, on_data]+ pair for a genuine bounded/streaming read.
      # Assign +on_data+ to a request's +req.options.on_data+; after the request
      # returns, +chunks.join+ is the accumulated body. The proc raises
      # {BodyTooLarge} the moment the accumulated size exceeds +max_body_bytes+,
      # so an oversized body is never fully buffered. Callers rescue
      # {BodyTooLarge} and map it to their own error. Shared by both discovery
      # hops and the device flow so every OAuth response reads under the same cap.
      #
      # +req.options.timeout+ only bounds each individual socket read, and every
      # +on_data+ chunk resets it — so a peer dripping one byte before each read
      # timeout can hold the connection open arbitrarily long without ever tripping
      # the cap. When a monotonic +deadline+ is supplied, the proc raises
      # {ReadDeadlineExceeded} the moment the WHOLE read outlives it, matching the
      # wall-clock bound the other SDKs enforce (Python's monotonic deadline, Go's
      # context, TS's abort timer, Kotlin's requestTimeoutMillis).
      #
      # @param max_body_bytes [Integer] bounded read cap in bytes
      # @param deadline [Float, nil] monotonic clock deadline (CLOCK_MONOTONIC seconds)
      # @return [Array(Array<String>, Proc)] the chunk buffer and the +on_data+ proc
      def self.bounded_reader(max_body_bytes, deadline: nil, skip_status: nil)
        chunks = []
        total = 0
        reader = proc do |chunk, _received, env|
          # Fast-path status-first skip: Faraday >= 2.5 passes +env+ (with the
          # response status) to +on_data+ once headers are in, so a body the caller
          # will not use (a non-2xx device-auth / a 3xx token redirect) is abandoned
          # at the FIRST body chunk rather than drained. +env+ is nil on older
          # Faraday (2.0–2.4) or a 2-arg call; every response that COMPLETES is
          # still classified by status via the caller's post-request re-check (see
          # +DeviceFlow#post_form+). +on_data+ is a body callback, not a headers
          # callback, so the one unreachable case — headers arrive, then the body
          # stalls past the read timeout — surfaces as a bounded transport timeout
          # instead (never followed, never unbounded; see +post_form+).
          raise SkipBody.new(env.status) if skip_status && env && skip_status.call(env.status)

          if deadline && Process.clock_gettime(Process::CLOCK_MONOTONIC) > deadline
            raise ReadDeadlineExceeded
          end

          total += chunk.bytesize
          raise BodyTooLarge if total > max_body_bytes

          chunks << chunk
        end
        [ chunks, reader ]
      end

      # Rejects an INJECTED connection whose middleware stack we cannot verify to
      # be redirect-free. Redirect suppression is a load-bearing SSRF control (RFC
      # 9728 §7.7): a caller-supplied client that follows redirects would silently
      # chase an attacker-controlled +Location+. A class-NAME heuristic (matching
      # +/redirect/+) is bypassable by a follower whose class name does not contain
      # "redirect", so we enforce a POLICY instead of guessing by name: an injected
      # connection may carry ONLY adapter handlers (an adapter-only connection or
      # a test's mock adapter qualifies); ANY request/response middleware — which
      # could follow redirects under any name, or otherwise rewrite the request —
      # is refused rather than trusted.
      #
      # @param client [Faraday::Connection]
      # @raise [OauthError] +validation+ when non-adapter middleware is present
      def self.ensure_redirects_suppressed!(client)
        return unless client.respond_to?(:builder)

        builder = client.builder
        handlers = Array(builder.handlers)
        # Faraday keeps the TERMINAL adapter handler OUTSIDE builder.handlers, so
        # a redirect-follower smuggled into the adapter slot (+conn.adapter Follower+,
        # not validated to be a Faraday::Adapter subclass) would evade a
        # handlers-only scan and run as the terminal app. Fold the adapter into
        # the same policy check: a genuine adapter (<= Faraday::Adapter) passes;
        # any non-adapter class in that slot is refused.
        handlers += [ builder.adapter ] if builder.respond_to?(:adapter)
        offending = handlers.compact.find do |h|
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

      # Headers-first bounded HTTP over Net::HTTP — the default transport for
      # every SDK-built OAuth fetch (both discovery hops and the device flow).
      # Injected Faraday connections keep the Faraday path; this primitive exists
      # because Faraday cannot provide these two guarantees:
      #
      # 1. **Status at header time.** Faraday's +on_data+ is a body callback, so a
      #    response whose body never arrives can only be classified after a
      #    timeout. Net::HTTP's block form yields the response once HEADERS are
      #    in: +skip_status+ classifies by status BEFORE any body read, and
      #    raising out of the block makes +Net::HTTP.start+'s ensure close the
      #    socket with the body undrained — exact status-first classification
      #    (SPEC.md §16) for every response shape, including a stalled body.
      # 2. **A total wall-clock bound.** A per-read timeout resets on every byte,
      #    so a peer dripping header or body bytes defeats it. A WATCHDOG thread
      #    closes the connection at a monotonic deadline, which interrupts even a
      #    blocked or dripped HEADER read (closing the socket from another thread
      #    raises IOError in the blocked reader — verified on a live socket).
      #    +max_retries = 0+ is load-bearing: Net::HTTP's idempotent-retry would
      #    otherwise silently REOPEN the connection the watchdog just closed.
      #
      # The body streams under the same cap + deadline as the Faraday path, and
      # redirects are structurally never followed (+Net::HTTP#request+ has no
      # follow logic). Transport failures surface as Faraday errors
      # (+TimeoutError+ for timeouts, +ConnectionFailed+ for connection and
      # protocol-parse failures) so both transport paths classify through the
      # same caller rescues. Bounded-read violations keep raising the shared
      # {BodyTooLarge} / {ReadDeadlineExceeded} markers — deliberately NOT
      # Faraday errors, so each caller maps them to its own operation-specific
      # error message, exactly as on the Faraday path.
      #
      # @param method [Symbol] +:get+ or +:post+
      # @param url [String] fully-qualified URL (already origin-validated)
      # @param headers [Hash] request headers
      # @param form [Hash, nil] form params; www-form-encoded into the POST body
      # @param timeout [Numeric] total request bound in seconds (already
      #   normalized by the caller). The deadline is anchored BEFORE connect and
      #   open_timeout carries the same value, so the total wall time is
      #   ~timeout regardless of which phase stalls (the watchdog closes the
      #   session the moment it exists if the deadline fired mid-connect)
      # @param max_body_bytes [Integer] bounded read cap in bytes
      # @param skip_status [Proc, nil] statuses whose body is never read
      # @return [Array(Integer, String)] status and (possibly empty) body
      def self.stream_http(method, url, headers: {}, form: nil, timeout:, max_body_bytes: DEFAULT_MAX_BODY_BYTES, skip_status: nil)
        uri = URI.parse(url)
        # URI#hostname strips IPv6 brackets ("[::1]" -> "::1"), which is the form
        # Net::HTTP.new expects. ENV proxy handling matches faraday-net_http.
        http = Net::HTTP.new(uri.hostname, uri.port)
        http.use_ssl = uri.scheme == "https"
        http.open_timeout = timeout
        http.read_timeout = timeout
        http.max_retries = 0

        request = method == :post ? Net::HTTP::Post.new(uri) : Net::HTTP::Get.new(uri)
        headers.each { |name, value| request[name] = value }
        request.body = URI.encode_www_form(form) if form

        deadline = Process.clock_gettime(Process::CLOCK_MONOTONIC) + timeout
        deadline_fired = false
        watchdog = Thread.new do
          remaining = deadline - Process.clock_gettime(Process::CLOCK_MONOTONIC)
          sleep(remaining) if remaining.positive?
          deadline_fired = true
          # Retry until the session exists to close: the deadline can fire while
          # the session is still CONNECTING (finish then raises IOError), and a
          # one-shot close would leave the subsequent header read unbounded. The
          # ensure below kills this thread the moment the request completes, so
          # the loop cannot outlive the call.
          begin
            http.finish
          rescue IOError
            sleep(0.05)
            retry
          end
        end

        status = nil
        chunks = []
        total = 0
        http.start do |session|
          session.request(request) do |response|
            status = response.code.to_i
            # Status-first: a skipped status's body is NEVER read — the raise
            # unwinds through start, whose ensure closes the socket undrained.
            raise SkipBody.new(status) if skip_status&.call(status)

            response.read_body do |chunk|
              raise ReadDeadlineExceeded if Process.clock_gettime(Process::CLOCK_MONOTONIC) > deadline

              total += chunk.bytesize
              raise BodyTooLarge if total > max_body_bytes

              chunks << chunk
            end
          end
        end
        [ status, chunks.join.force_encoding(Encoding::UTF_8) ]
      rescue SkipBody => e
        [ e.status, "" ]
      rescue Net::OpenTimeout, Net::ReadTimeout => e
        raise Faraday::TimeoutError, "OAuth request timed out: #{e.message}"
      rescue IOError => e
        # The watchdog's close raises IOError in the blocked reader; only map it
        # to a timeout when the deadline actually fired — any other IOError (a
        # peer closing mid-headers, for example) is a connection failure.
        raise Faraday::TimeoutError, "OAuth request exceeded the timeout deadline" if deadline_fired

        raise Faraday::ConnectionFailed, e.message
      rescue OpenSSL::SSL::SSLError => e
        # TLS failures (an unverifiable peer certificate above all) map to
        # Faraday::SSLError exactly as faraday-net_http maps them, so the
        # default and injected paths classify certificate rejection alike.
        raise Faraday::SSLError, e.message
      rescue Net::HTTPBadResponse, Net::HTTPHeaderSyntaxError, Net::ProtocolError,
             SystemCallError, SocketError => e
        # The parse errors are direct StandardError subclasses (not IOError), so
        # a malformed status line / header must be mapped here explicitly or it
        # would leak raw from the public discovery/device APIs.
        raise Faraday::ConnectionFailed, e.message
      ensure
        watchdog&.kill
        watchdog&.join
      end

      # Fetches +url+ and returns the parsed JSON object (a Hash).
      #
      # With a nil +http_client+ the fetch runs on the headers-first
      # {stream_http} primitive (total wall-clock bound incl. the header phase).
      # An INJECTED connection keeps the Faraday path: the request timeout is
      # applied per-request (not only on the connection) so a bounded read is
      # enforced even under the injected adapter's defaults, and the wall-clock
      # deadline bounds the whole body read — but Faraday exposes no
      # headers-time callback, so a body that stalls past the read timeout on
      # the injected path surfaces as a bounded transport timeout.
      #
      # @param http_client [Faraday::Connection, nil] injected connection, or
      #   nil for the default headers-first transport
      # @param url [String] fully-qualified well-known URL to fetch
      # @param timeout [Integer] per-request timeout in seconds
      # @param max_body_bytes [Integer] bounded read cap in bytes
      # @return [Hash] the parsed JSON document
      # @raise [OauthError] +api_error+ on non-2xx, oversized body, non-object
      #   JSON, or parse failure; +network+ on transport failure
      def self.fetch_json(http_client, url, timeout:, max_body_bytes: DEFAULT_MAX_BODY_BYTES)
        status, body =
          if http_client.nil?
            stream_http(
              :get, url,
              headers: { "Accept" => "application/json" },
              timeout: timeout, max_body_bytes: max_body_bytes,
              # STATUS DOMINATES THE BODY (SPEC.md: non-2xx on either hop →
              # api_error, never network): skip draining a non-2xx body so a
              # stalled/dripped error body cannot convert the required api_error
              # into a network timeout. The body text was only optional
              # diagnostics — the other SDKs read it best-effort at most.
              skip_status: ->(response_status) { !(200..299).cover?(response_status) }
            )
          else
            faraday_fetch(http_client, url, timeout: timeout, max_body_bytes: max_body_bytes)
          end

        unless (200..299).cover?(status)
          # The default path skips the non-2xx body (empty here); the injected
          # Faraday path still carries it — append it only when present.
          detail = body.empty? ? "" : ": #{Basecamp::Security.truncate(body)}"
          raise OauthError.new(
            "api_error",
            "OAuth discovery failed with status #{status}#{detail}",
            http_status: status
          )
        end

        data = JSON.parse(body)
        raise OauthError.new("api_error", "OAuth discovery response is not a JSON object") unless data.is_a?(Hash)

        data
      rescue BodyTooLarge
        raise OauthError.new("api_error", "OAuth discovery response exceeds size cap")
      rescue ReadDeadlineExceeded
        raise OauthError.new("network", "OAuth discovery timed out", retryable: true)
      rescue Faraday::Error => e
        raise OauthError.new("network", "OAuth discovery failed: #{e.message}", retryable: true)
      rescue JSON::ParserError => e
        raise OauthError.new("api_error", "Failed to parse OAuth discovery response: #{e.message}")
      end

      # The Faraday transport for an INJECTED connection. The request timeout is
      # applied per-request (not only on the connection) so a bounded read is
      # enforced even under the injected adapter's defaults, and the wall-clock
      # deadline bounds the whole body read.
      #
      # @return [Array(Integer, String)] status and body
      def self.faraday_fetch(http_client, url, timeout:, max_body_bytes:)
        # Wall-clock deadline over the WHOLE read: req.options.timeout below bounds
        # only each socket read and resets on every chunk, so a slow-drip peer could
        # otherwise hang the fetch indefinitely while staying under max_body_bytes.
        deadline = Process.clock_gettime(Process::CLOCK_MONOTONIC) + timeout
        chunks, on_data = bounded_reader(max_body_bytes, deadline: deadline)

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

        [ response.status, chunks.join.force_encoding(Encoding::UTF_8) ]
      end
    end
  end
end
