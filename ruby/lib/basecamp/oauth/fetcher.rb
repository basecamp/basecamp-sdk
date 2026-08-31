# frozen_string_literal: true

require "faraday"
require "json"
# Full cgi, not cgi/escape: on Ruby 3.2-3.4 (the gem floor is 3.2) the
# escape-only require leaves CGI.unescapeURIComponent broken with an
# uninitialized @@accept_charset NameError.
require "cgi"
require "net/http"
require "openssl"
require "timeout"
require "uri"
require "zlib"

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

      # Upper bound (seconds) for any single OAuth request timeout. A huge but
      # FINITE caller value (1e100) would pass a bare finite/positive check and
      # hold both Faraday's socket timeout and the monotonic deadline open
      # effectively forever — the same cap discipline as Python
      # (_MAX_DEVICE_REQUEST_TIMEOUT = 3600) and TS (resolveDeviceTimeoutMs
      # rejects oversized values back to the default).
      MAX_REQUEST_TIMEOUT = 3600

      # +real?+ gates out Complex before +finite?+/+positive?+ (which Complex does
      # not define — calling them would raise NoMethodError). Integer, Float, and
      # Rational are all real and answer both.
      # Monotonic clock read (seconds). A module seam — rather than inline
      # Process.clock_gettime — so tests can stub Fetcher's own notion of
      # "now" without touching the process-wide clock that Net::HTTP, the
      # watchdog sleeps, and the test server all rely on.
      # net-http >= 0.5 (Ruby 3.4+) added Net::HTTP.new's eighth p_use_ssl
      # parameter; the gem floor is Ruby 3.2, whose bundled net-http lacks it.
      def self.proxy_tls_capable?
        Net::HTTP.method(:new).parameters.length >= 8
      end

      def self.monotonic_now
        Process.clock_gettime(Process::CLOCK_MONOTONIC)
      end

      def self.valid_timeout?(value)
        value.is_a?(Numeric) && value.real? && value.finite? && value.positive? \
          && value <= MAX_REQUEST_TIMEOUT
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

      # Classifies a wire fault from {stream_http}'s read into the Faraday error
      # the caller rescues. Once the watchdog DEADLINE has fired, the forced
      # close can surface in the blocked reader as IOError, a SystemCallError
      # (ECONNRESET/EBADF and friends), or SocketError depending on platform
      # and read phase — ALL of them are the timeout then, or the device poll
      # would terminate as transport instead of applying its transient backoff.
      # Before the deadline they are genuine wire faults: a peer closing
      # mid-headers, a malformed status line/header (the Net parse errors are
      # direct StandardError subclasses, not IOError, so they must be mapped
      # explicitly or they leak raw), or malformed compressed bytes from
      # Net::HTTP's automatic Content-Encoding decode (Zlib::Error).
      def self.classify_stream_error(error, deadline_fired)
        if deadline_fired
          Faraday::TimeoutError.new("OAuth request exceeded the timeout deadline")
        else
          # Wrap the exception itself (the conventional Faraday pattern), not
          # just its message: the original class and backtrace survive for
          # debugging while callers' rescues see the same Faraday class.
          Faraday::ConnectionFailed.new(error)
        end
      end

      # Raised to abort a streaming read once the cap is exceeded. Crosses the
      # module boundary by design: {stream_http} lets it propagate raw, and each
      # caller (fetch_json here, the device flow's post paths) maps it to its
      # own typed cap fault.
      class BodyTooLarge < StandardError; end

      # Raised when a streaming read exceeds its wall-clock deadline. Crosses
      # the module boundary like {BodyTooLarge}: callers map it to a retryable
      # +network+ fault (fetch_json) or a Faraday timeout (the device flow).
      class ReadDeadlineExceeded < StandardError; end

      # Raised from +on_data+ to STOP reading a response whose body the caller does
      # not use (a non-2xx device-auth, a 3xx token redirect). Draining such a slow
      # body would otherwise time out and be misclassified as a transport failure.
      # Carries the response status so the caller can classify by it; like the
      # markers above it crosses the module boundary raw and is mapped there.
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
          # Deadline FIRST: a first chunk that becomes runnable past the total
          # bound means the status was not known in time — the timeout wins,
          # matching the default transport's header-time gate. (The resetting
          # per-read timeout alone would admit it.)
          if deadline && monotonic_now > deadline
            raise ReadDeadlineExceeded
          end

          raise SkipBody.new(env.status) if skip_status && env && skip_status.call(env.status)

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
          "Injected OAuth client must carry only an adapter (no middleware); " \
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
      #    Scope: the watchdog governs from session-start on. The CONNECTION
      #    phases (TCP connect, proxy CONNECT, TLS handshake) are not
      #    watchdog-interruptible — Net::HTTP marks the session started only
      #    after they complete — so connection setup runs under its own
      #    whole-phase Timeout.timeout bound (a proxy's CONNECT response is
      #    parsed under the per-read timeout, which a byte-dripping proxy
      #    resets forever), and a post-connect deadline re-check refuses the
      #    request when setup consumed the budget. Total wall clock is
      #    ~timeout, never unbounded.
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
      # @param on_headers [Proc, nil] called with the +Net::HTTPResponse+ once
      #   headers arrive, before any body read or skip decision — the device
      #   poll loop reads +Retry-After+ here without widening the return shape
      # @return [Array(Integer, String)] status and (possibly empty) body
      def self.stream_http(method, url, headers: {}, form: nil, timeout:, max_body_bytes: DEFAULT_MAX_BODY_BYTES, skip_status: nil, on_headers: nil)
        # Response state first, before ANY call the def-level rescues could
        # catch, so every rescue path sees it assigned (the completed_at guard
        # means it is only READ once genuinely populated).
        status = nil
        chunks = []
        total = 0
        uri = begin
          URI.parse(url)
        rescue URI::InvalidURIError
          nil
        end
        # Fail closed on an unparsable or hostless URL ("https:foo" parses with a
        # nil hostname): require_https checks only the scheme, and a nil host
        # would otherwise surface as a raw ArgumentError from inside Net::HTTP —
        # outside the transport's Faraday-error contract.
        if uri.nil? || uri.hostname.nil? || uri.hostname.empty?
          raise OauthError.new("validation", "OAuth endpoint URL has no host: #{url.inspect}")
        end
        # Fail closed on a non-HTTP(S) scheme: callers TLS-guard upstream, but
        # the primitive must not run an HTTP conversation against ftp:// etc.
        unless %w[http https].include?(uri.scheme)
          raise OauthError.new("validation", "OAuth endpoint URL must be http(s): #{url.inspect}")
        end

        # Fail closed on an un-normalized timeout (the operation entry points
        # normalize; this guards direct callers): a non-finite, non-positive,
        # or beyond-ceiling value would leave the socket timeouts and the
        # watchdog's sleep unbounded, defeating the total-request bound this
        # primitive exists to guarantee — mirroring the Python transport's
        # fail-fast guard.
        unless valid_timeout?(timeout)
          raise OauthError.new("validation", \
            "stream_http timeout must be a positive number of seconds no greater than #{MAX_REQUEST_TIMEOUT}")
        end
        # The body cap gets the same fail-closed discipline: nil, a Float
        # (Infinity included), or a negative value would disable or crash the
        # streaming bound. Same predicate as {normalize_body_cap} so the
        # transport can never reject a cap the public entry points accept —
        # zero is a legitimate strict cap (any non-empty body trips it).
        unless valid_body_cap?(max_body_bytes)
          raise OauthError.new("validation", "stream_http max_body_bytes must be a non-negative Integer")
        end

        # URI#hostname strips IPv6 brackets ("[::1]" -> "::1"), which is the form
        # Net::HTTP.new expects.
        #
        # Net::HTTP's built-in :ENV proxy detection hardcodes an http:// URI
        # before calling find_proxy, so an HTTPS_PROXY-only environment would
        # silently bypass its proxy for HTTPS endpoints (and http_proxy would
        # wrongly govern TLS requests). Resolve the proxy against the REAL
        # scheme — matching faraday-net_http's per-scheme resolution — and
        # pass it explicitly; a nil p_addr disables the broken built-in.
        # The total deadline starts BEFORE proxy resolution: find_proxy
        # resolves the target hostname (IPSocket.getaddress) to evaluate its
        # loopback rule — a blocking DNS call that must sit inside the
        # advertised bound like every other network step. Net::OpenTimeout
        # maps through the Timeout::Error rescue to the transport timeout.
        deadline = monotonic_now + timeout
        deadline_fired = false
        completed_at = nil
        proxy_uri = Timeout.timeout(timeout, Net::OpenTimeout) { uri.find_proxy }
        http = if proxy_uri
                 proxy_tls = proxy_uri.scheme == "https"
                 # An https:// proxy needs TLS on its own connection via the
                 # p_use_ssl argument, which exists only in net-http >= 0.5
                 # (Ruby 3.4+). On the older bundled net-http (Ruby 3.2/3.3)
                 # the 8-arg call would raise ArgumentError — refuse with an
                 # actionable error instead of plaintext-to-a-TLS-proxy.
                 if proxy_tls && !proxy_tls_capable?
                   raise OauthError.new(
                     "validation",
                     "https:// proxies need net-http >= 0.5 (Ruby 3.4+, or add the net-http gem)"
                   )
                 end

                 # Percent-decode the credentials: URI#user/#password return the
                 # encoded forms, and the explicit-proxy Net::HTTP.new does NOT
                 # unescape them the way its :ENV mode does — p%40ss would be
                 # sent verbatim in Proxy-Authorization and fail authentication.
                 # unescapeURIComponent, NOT form decoding: a literal + in a
                 # userinfo component is a plus sign, never a space.
                 # p_no_proxy nil (find_proxy already honored no_proxy).
                 args = [
                   uri.hostname, uri.port,
                   proxy_uri.hostname, proxy_uri.port,
                   proxy_uri.user && CGI.unescapeURIComponent(proxy_uri.user),
                   proxy_uri.password && CGI.unescapeURIComponent(proxy_uri.password)
                 ]
                 args += [ nil, true ] if proxy_tls
                 Net::HTTP.new(*args)
        else
                 Net::HTTP.new(uri.hostname, uri.port, nil)
        end
        http.use_ssl = uri.scheme == "https"
        http.open_timeout = timeout
        http.read_timeout = timeout
        http.write_timeout = timeout
        http.max_retries = 0

        # A form body is only meaningful on POST — a GET-with-body masks a
        # call-site mistake and contradicts this method's own contract.
        raise ArgumentError, "stream_http: form is only valid with :post" if form && method != :post

        # Identity encoding IN THE INITHEADER: Net::HTTP inflates gzip/deflate
        # BEFORE read_body yields, so the per-chunk cap would measure DECODED
        # bytes — a small compressed body could balloon far past max_body_bytes
        # in memory (compression bomb). A caller-supplied Accept-Encoding at
        # construction time is the supported way to switch decode_content off
        # (assigning the header later does not); a server compressing anyway
        # hands us raw bytes bounded by the cap, which then fail
        # classification upstream instead of exhausting memory.
        identity = { "Accept-Encoding" => "identity" }
        request =
          case method
          when :post then Net::HTTP::Post.new(uri, identity)
          when :get then Net::HTTP::Get.new(uri, identity)
          else raise ArgumentError, "stream_http supports :get and :post, got #{method.inspect}"
          end
        headers.each do |name, value|
          # The identity Accept-Encoding above is load-bearing (compression
          # bomb bound): a caller-supplied override would reintroduce
          # transparent inflation, so it is dropped — matching the Python
          # transport, which forcibly overwrites the header.
          next if name.to_s.casecmp("accept-encoding").zero?

          request[name] = value
        end
        if form
          request.body = URI.encode_www_form(form)
          # A form body implies the form content type; explicit headers win.
          request["Content-Type"] ||= "application/x-www-form-urlencoded"
        end

        # Net::HTTP#request implicitly re-starts a finished session, and that
        # restart runs OUTSIDE the connect Timeout.timeout below — an
        # ENV-proxied CONNECT could drip unbounded there (started? stays
        # false, so the watchdog cannot close it). The only path to a
        # finished session is the watchdog, which fires only after the
        # deadline: guarding EVERY start on deadline_fired closes every
        # implicit-reconnect path deterministically.
        http.define_singleton_method(:start) do |&blk|
          raise Net::OpenTimeout, "total deadline exceeded before (re)connect" if deadline_fired

          super(&blk)
        end
        watchdog = Thread.new do
          remaining = deadline - monotonic_now
          sleep(remaining) if remaining.positive?
          deadline_fired = true
          # Close the session and KEEP closing until the ensure below kills this
          # thread. A one-shot close is wrong twice over: finish raises IOError
          # while the session is still CONNECTING (a single failed attempt would
          # leave the subsequent header read unbounded), and Net::HTTP#request
          # implicitly RE-STARTS a finished session — a close that landed
          # between the post-connect deadline re-check and the request write
          # would hand the reopened connection a fresh, unwatched header wait.
          # Looping bounds any such reopen to one tick. The ensure kills this
          # thread the moment the request completes, so the loop cannot outlive
          # the call.
          loop do
            # The ensure's kill must never land INSIDE finish: interrupted
            # after do_finish clears started? but before the socket close,
            # BOTH closers skip — the request-side ensure sees started? false
            # and this thread is dead — leaking the socket until GC. Defer the
            # kill across each close attempt; it lands at the sleep below.
            Thread.handle_interrupt(Object => :never) do
              http.finish
            rescue IOError
              # Not started: still connecting, or between implicit reopens.
            end
            sleep(0.05)
          end
        end

        # The connection phases (TCP connect, proxy CONNECT, TLS handshake) run
        # before Net::HTTP marks the session started, so the watchdog cannot
        # interrupt them (finish raises IOError until then) — and only TCP
        # connect and the TLS handshake carry native open_timeout bounds. A
        # proxy's CONNECT response is parsed under the PER-READ timeout, which
        # resets on every dripped byte: without a whole-phase bound, an
        # ENV-proxied HTTPS request could sit in connection setup indefinitely.
        # Timeout.timeout's async raise is safe here precisely because the
        # guarded region is ONLY connection setup: no response state exists
        # yet, and Net::HTTP's connect rescue closes both sockets on any raise.
        connect_remaining = deadline - monotonic_now
        raise ReadDeadlineExceeded unless connect_remaining.positive?

        begin
          # start INSIDE the cleanup region: Timeout.timeout's asynchronous
          # Net::OpenTimeout can land after do_start marked the session
          # started but before the next statement — outside the ensure, that
          # window leaked a live connection until GC (the outer ensure can
          # kill the watchdog before it ever closes).
          Timeout.timeout(connect_remaining, Net::OpenTimeout) { http.start }
          # Re-check the deadline the moment the session is up: if setup
          # consumed the whole budget, fail now rather than granting the
          # request a fresh header wait.
          raise ReadDeadlineExceeded if monotonic_now > deadline

          http.request(request) do |response|
            status = response.code.to_i
            # Headers are available before any body read; the device poll loop
            # uses this to read Retry-After without widening the return shape.
            on_headers&.call(response)
            if skip_status&.call(status)
              # Deadline first for a status NOT known in time: headers that
              # become runnable past the monotonic deadline (but before the
              # watchdog flips deadline_fired) are a timeout, not a
              # classification — matching the Python transport. A status
              # known IN time is still raised before any body read: the
              # SkipBody unwinds to the ensure below, which closes the
              # socket undrained, so a skipped status's body is NEVER
              # drained and a discovery 500 or token redirect known before
              # the deadline never softens into a retryable timeout.
              raise ReadDeadlineExceeded if monotonic_now > deadline

              raise SkipBody.new(status)
            end

            # Net::HTTP#request implicitly re-starts a finished session: if the
            # watchdog's close landed between the deadline re-check above and
            # the request write, this round trip rode a fresh post-deadline
            # connection. The watchdog loop closes such a connection within a
            # tick; this classifies a non-skipped round trip that beat the
            # next tick, BEFORE its body is read.
            raise ReadDeadlineExceeded if deadline_fired

            response.read_body do |chunk|
              raise ReadDeadlineExceeded if monotonic_now > deadline

              total += chunk.bytesize
              raise BodyTooLarge if total > max_body_bytes

              chunks << chunk
            end
            # Stamp completion INSIDE the request block: Net::HTTP's own
            # end_transport runs before http.request returns, and the
            # watchdog racing that cleanup must not erase a completed body.
            completed_at = monotonic_now
          end
          # Final monotonic re-check AFTER the response completes: a peer can
          # deliver the last body chunk just before the deadline and the
          # terminating EOF just after it — read_body runs no further per-chunk
          # check, and the request thread can process that completion before
          # the watchdog sets deadline_fired. A completed response is never
          # accepted past the advertised total-request bound. (Skipped statuses
          # keep status-first classification — SkipBody unwinds before this.)
          raise ReadDeadlineExceeded if monotonic_now > deadline
        ensure
          # Block-form start would close the session itself; with the explicit
          # start (needed so ONLY connection setup sits under Timeout.timeout)
          # close it here. When the connect timeout fires between Net::HTTP's
          # connect assigning a live @socket and do_start marking the session
          # started, started? is still false and Net::HTTP's own connect
          # rescue is out of scope — close the orphaned socket directly or it
          # leaks until GC.
          begin
            if http.started?
              http.finish
            else
              orphan = http.instance_variable_get(:@socket)
              orphan&.close
            end
          rescue IOError
            # Already closed by the watchdog.
          end
        end
        [ status, chunks.join.force_encoding(Encoding::UTF_8) ]
      rescue SkipBody => e
        [ e.status, "" ]
      rescue Timeout::Error, Errno::ETIMEDOUT => e
        # Timeout::Error covers Net::OpenTimeout/ReadTimeout/WriteTimeout alike
        # (a peer that accepts the connection but stops READING trips the write
        # timeout). Errno::ETIMEDOUT is a SystemCallError, but it is a TIMEOUT:
        # both must map with the timeouts — the exact pair faraday-net_http
        # rescues — or the device poll would terminate instead of applying its
        # transient backoff.
        raise Faraday::TimeoutError, "OAuth request timed out: #{e.message}"
      rescue IOError, Net::HTTPBadResponse, Net::HTTPHeaderSyntaxError, Net::ProtocolError,
             SystemCallError, SocketError, Zlib::Error => e
        # The clock outranks the watchdog FLAG: a wire fault observed past the
        # monotonic deadline is the timeout it raced, even when the watchdog
        # thread has not yet been scheduled to flip deadline_fired — otherwise
        # a post-deadline peer reset classifies as ConnectionFailed and the
        # device poll terminates instead of applying its transient backoff.
        raise classify_stream_error(e, deadline_fired || monotonic_now > deadline)
      rescue NoMethodError => e
        # The watchdog's cross-thread finish can nil @socket between the body
        # completing and Net::HTTP's own end_transport, whose @socket.closed?
        # then raises NoMethodError on the request thread. An in-deadline
        # COMPLETED response dominates that cleanup race (mirroring the
        # Python transport's outcome preservation); otherwise, past the
        # deadline, it is the timeout the cleanup raced. A genuine
        # NoMethodError (a bug) re-raises untouched.
        if completed_at && completed_at <= deadline
          return [ status, chunks.join.force_encoding(Encoding::UTF_8) ]
        end
        raise classify_stream_error(e, true) if deadline_fired || monotonic_now > deadline

        raise
      rescue OpenSSL::SSL::SSLError => e
        # The watchdog's forced close surfaces mid-handshake/mid-read as an
        # SSLError: past the deadline it is the timeout it raced, same clock
        # rule as the wire-error branch above. A genuine pre-deadline TLS
        # failure (an unverifiable peer certificate above all) still maps to
        # Faraday::SSLError exactly as faraday-net_http maps it, so the
        # default and injected paths classify certificate rejection alike.
        raise Faraday::TimeoutError, "OAuth request timed out: TLS interrupted past the deadline" \
          if deadline_fired || monotonic_now > deadline

        raise Faraday::SSLError, e.message
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
      # @param timeout [Numeric] per-request timeout in seconds (fractional accepted)
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
          # Status-only, on BOTH paths: the error category, retryability, and
          # http_status are the observable contract; embedding response body
          # text would put attacker-influenced content in exception messages
          # and diverge from the (body-less) default transport and Python.
          raise OauthError.new(
            "api_error",
            "OAuth discovery failed with status #{status}",
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
      # Known residual (injected connections only): unlike the default
      # {stream_http} path — which skips unusable bodies by status at header
      # time — Faraday exposes no headers-time seam, so a non-2xx body is
      # drained here under the same cap + deadline before the status error
      # surfaces. Bounded, never followed; callers who inject a connection
      # keep their transport's semantics by design.
      #
      # @return [Array(Integer, String)] status and body
      def self.faraday_fetch(http_client, url, timeout:, max_body_bytes:)
        # Wall-clock deadline over the WHOLE read: req.options.timeout below bounds
        # only each socket read and resets on every chunk, so a slow-drip peer could
        # otherwise hang the fetch indefinitely while staying under max_body_bytes.
        deadline = monotonic_now + timeout
        # Streaming adapters (Faraday >= 2.5 pass +env+ to on_data) classify a
        # non-2xx at HEADER time like the default path — fetch_json discards
        # non-2xx bodies, so draining one is pure waste. Buffered adapters
        # remain the documented drain-then-classify residual.
        chunks, on_data = bounded_reader(
          max_body_bytes, deadline: deadline,
          skip_status: ->(s) { !(200..299).cover?(s) }
        )

        # Timeout.timeout wraps the WHOLE call: the per-read timeout resets on
        # every socket read, so a peer dripping HEADER bytes under it would
        # otherwise hold the request open indefinitely — on_data (a body
        # callback) never runs during the header phase, leaving nothing else
        # to enforce the wall clock on an injected client.
        # The window is the REMAINING budget, not a fresh +timeout+: time
        # spent before dispatch (descheduling included) already counts
        # against the deadline, so the request can never run past it.
        remaining = deadline - monotonic_now
        raise Faraday::TimeoutError, "request budget exhausted before dispatch" if remaining <= 0

        response = Timeout.timeout(remaining, Faraday::TimeoutError) do
          http_client.get(url) do |req|
            req.headers["Accept"] = "application/json"
            # Bounded streaming read: abort the moment the cap is exceeded so an
            # oversized body is never fully buffered.
            req.options.on_data = on_data
            # Apply the request timeout on every request — even an injected client —
            # so a stalled socket can't hang discovery under the adapter default.
            req.options.timeout = timeout
            req.options.open_timeout = timeout
          end
        end

        # Timeout.timeout's interrupt can be delivered late: a 2xx whose block
        # returns just after the deadline would otherwise be accepted past the
        # wall clock. A completed non-2xx stays status-classified (status
        # outranks the deadline race, matching the default path); only a late
        # 2xx is refused as the same transport-shaped timeout.
        raise Faraday::TimeoutError, "response completed after the deadline" \
          if (200..299).cover?(response.status) && monotonic_now > deadline

        body =
          if chunks.empty?
            if (200..299).cover?(response.status)
              # A buffered adapter (Faraday's test adapter above all) never
              # invokes on_data: the streamed chunks are empty while the body
              # sits on the response. Fall back to it under the same cap so an
              # injected buffered client gets the document instead of a bogus
              # empty-body parse failure — mirroring post_form's fallback.
              # dup ONLY a frozen body (test adapters return literals, which
              # cannot take force_encoding below): copying every large
              # buffered 2xx unconditionally would double peak memory.
              raw = response.body.to_s
              raw = raw.dup if raw.frozen?
              raise BodyTooLarge if raw.bytesize > max_body_bytes

              raw
            else
              # fetch_json discards non-2xx bodies (status dominates, SPEC.md):
              # return status-only rather than dup-and-size-checking a body
              # nobody reads, so an oversized buffered error body surfaces as
              # the status fault — not a size-cap one — and is never copied.
              # +"" — the frozen literal cannot take force_encoding below.
              +""
            end
          else
            chunks.join
          end
        [ response.status, body.force_encoding(Encoding::UTF_8) ]
      rescue SkipBody => e
        [ e.status, "" ]
      end
    end
  end
end
