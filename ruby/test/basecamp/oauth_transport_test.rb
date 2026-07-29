# frozen_string_literal: true

require "test_helper"
require "faraday"
require "net/http"
require "openssl"
require "socket"
require "zlib"

# Acceptance tests for the headers-first default transport
# ({Basecamp::Oauth::Fetcher.stream_http}) against REAL sockets — the response
# shapes these prove (a stalled or byte-dripped header phase, a body that never
# arrives after headers) cannot be produced by WebMock stubs or Faraday test
# adapters, and are exactly the shapes the primitive exists to bound.
class OAuthTransportTest < Minitest::Test
  TIMEOUT = 0.6

  def setup
    # Fully unpatch Net::HTTP (not merely allow_localhost): WebMock's patched
    # Net::HTTP buffers even allowed real requests, which destroys the
    # header-time semantics these tests exist to prove.
    WebMock.disable!
    @servers = []
    @conns = []
    @server_threads = []
  end

  def teardown
    # Stop the server threads FIRST (they append to @conns), then close every
    # accepted socket and listener — the stall handlers sleep for tens of
    # seconds, so without the kill+join each test would leak a live thread and
    # its socket well past the test's end.
    @server_threads.each(&:kill).each(&:join)
    @conns.each { |conn| conn.close rescue nil }
    @servers.each { |server| server.close rescue nil }
    WebMock.enable!
    WebMock.disable_net_connect!
  end

  # Starts a real TCP server; the handler receives each accepted socket after
  # the request headers have been consumed. Returns [endpoint, accepts] where
  # +accepts+ counts connections — the zero-retry assertions read it. Accepted
  # sockets and the accept thread are tracked for teardown.
  def start_server(&handler)
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    accepts = []
    @server_threads << Thread.new do
      loop do
        conn = server.accept
        accepts << conn
        @conns << conn
        while (line = conn.gets) && line != "\r\n"; end
        handler.call(conn)
      rescue IOError, SystemCallError
        break # server closed in teardown
      end
    end
    [ "http://127.0.0.1:#{server.addr[1]}", accepts ]
  end

  def elapsed
    start = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    yield
    Process.clock_gettime(Process::CLOCK_MONOTONIC) - start
  end

  # --- status-first: a skipped status classifies at HEADER time -------------

  def test_device_auth_non_2xx_with_stalled_body_is_immediate_api_error
    # 500 headers, then NOT ONE body byte: the old body-callback transport could
    # only time this out as :transport; headers-first classifies it instantly.
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 500 Internal Server Error\r\nContent-Length: 1000\r\n\r\n")
      sleep 30
    end

    error = nil
    seconds = elapsed do
      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth::DeviceFlow.request_device_authorization(
          device_authorization_endpoint: "#{endpoint}/device",
          client_id: "basecamp-cli", timeout: TIMEOUT
        )
      end
    end

    assert_equal "api_error", error.type
    assert_equal 500, error.http_status
    assert_operator seconds, :<, TIMEOUT, "status must classify at header time, not after a body timeout"
  end

  def test_token_poll_302_with_stalled_body_is_immediate_api_error_with_zero_retries
    # The SPEC §16 contract this transport closes: a token 3xx whose body stalls
    # must surface the redirect api_error immediately — one request, no
    # transport-backoff retries toward code expiry.
    endpoint, accepts = start_server do |conn|
      conn.write("HTTP/1.1 302 Found\r\nLocation: https://attacker.example/\r\nContent-Length: 1000\r\n\r\n")
      sleep 30
    end

    error = nil
    seconds = elapsed do
      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth::DeviceFlow.poll_device_token(
          token_endpoint: "#{endpoint}/token", client_id: "basecamp-cli",
          device_code: "d", interval: 5, expires_in: 900,
          timeout: TIMEOUT, sleeper: ->(_seconds) { }
        )
      end
    end

    assert_equal "api_error", error.type
    assert_equal 302, error.http_status
    assert_match(/redirect/i, error.message)
    assert_equal 1, accepts.length, "a header-classified redirect must never be retried"
    assert_operator seconds, :<, TIMEOUT
  end

  def test_skipped_response_closes_the_connection_undrained
    # Releasing the connection matters as much as classifying it: the server
    # must observe the socket close (EPIPE/RST) instead of feeding a body to a
    # client that will never read it.
    server_saw_close = Queue.new
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 302 Found\r\nLocation: https://x/\r\nContent-Length: 10000000\r\n\r\n")
      begin
        1_000.times { conn.write("x" * 10_000); sleep 0.005 }
        server_saw_close << false
      rescue IOError, SystemCallError
        server_saw_close << true
      end
    end

    status, body = Basecamp::Oauth::Fetcher.stream_http(
      :post, "#{endpoint}/token", form: { "a" => "b" },
      timeout: TIMEOUT, skip_status: ->(s) { (300..399).cover?(s) }
    )

    assert_equal 302, status
    assert_equal "", body
    assert_equal true, server_saw_close.pop, "the abandoned body's socket must be torn down"
  end

  # --- total wall-clock bound, including the header phase -------------------

  def test_header_phase_stall_is_bounded_transport_error
    endpoint, = start_server { |_conn| sleep 30 } # headers never arrive

    error = nil
    seconds = elapsed do
      error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
        Basecamp::Oauth::DeviceFlow.request_device_authorization(
          device_authorization_endpoint: "#{endpoint}/device",
          client_id: "basecamp-cli", timeout: TIMEOUT
        )
      end
    end

    assert_equal :transport, error.reason
    assert_operator seconds, :<, TIMEOUT * 3, "a header stall must be bounded by the watchdog"
  end

  def test_header_phase_drip_is_bounded_transport_error
    # One header byte per 0.1s: every read succeeds inside the per-read timeout,
    # so only the watchdog's monotonic deadline can bound this — the case that
    # is structurally impossible to bound through Faraday's on_data.
    endpoint, = start_server do |conn|
      "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n".each_char do |char|
        begin
          conn.write(char)
        rescue IOError, SystemCallError
          break
        end
        sleep 0.1
      end
      sleep 30
    end

    error = nil
    seconds = elapsed do
      error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
        Basecamp::Oauth::DeviceFlow.request_device_authorization(
          device_authorization_endpoint: "#{endpoint}/device",
          client_id: "basecamp-cli", timeout: TIMEOUT
        )
      end
    end

    assert_equal :transport, error.reason
    assert_operator seconds, :<, TIMEOUT * 3, "a dripped header phase must be bounded by the watchdog"
  end

  def test_discovery_non_2xx_with_stalled_body_is_immediate_api_error
    # SPEC.md: non-2xx on either discovery hop → api_error, never network —
    # status dominates even when the error body stalls forever.
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 500 Internal Server Error\r\nContent-Length: 1000\r\n\r\n")
      sleep 30
    end

    error = nil
    seconds = elapsed do
      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth::Fetcher.fetch_json(nil, "#{endpoint}/doc", timeout: TIMEOUT)
      end
    end

    assert_equal "api_error", error.type
    assert_equal 500, error.http_status
    assert_operator seconds, :<, TIMEOUT, "status must classify at header time"
  end

  def test_body_slow_drip_is_bounded_for_discovery
    # A wanted (2xx) body dripped forever: the read-loop deadline bounds it and
    # discovery surfaces its retryable network timeout.
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n")
      loop do
        conn.write("x")
        sleep 0.1
      rescue IOError, SystemCallError
        break
      end
    end

    error = nil
    seconds = elapsed do
      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth::Fetcher.fetch_json(nil, "#{endpoint}/.well-known/oauth-authorization-server", timeout: TIMEOUT)
      end
    end

    assert_equal "network", error.type
    assert error.retryable
    assert_operator seconds, :<, TIMEOUT * 3
  end

  def test_oversized_body_aborts_streaming_read
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Length: 300000\r\n\r\n")
      begin
        30.times { conn.write("x" * 10_000) }
      rescue IOError, SystemCallError
        nil
      end
    end

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.fetch_json(
        nil, "#{endpoint}/doc", timeout: TIMEOUT, max_body_bytes: 8 * 1024
      )
    end

    assert_equal "api_error", error.type
    assert_match(/size cap/i, error.message)
  end

  def test_self_signed_tls_certificate_is_rejected_and_mapped
    # The default transport moved from Faraday to direct Net::HTTP — prove peer
    # verification survived the move: a self-signed certificate must fail the
    # handshake and map to the same Faraday::SSLError → network classification
    # faraday-net_http produced, never a raw OpenSSL exception (and never a
    # completed request).
    key = OpenSSL::PKey::RSA.new(2048)
    name = OpenSSL::X509::Name.parse("/CN=127.0.0.1")
    cert = OpenSSL::X509::Certificate.new
    cert.version = 2
    cert.serial = 1
    cert.subject = name
    cert.issuer = name
    cert.public_key = key.public_key
    cert.not_before = Time.now - 60
    cert.not_after = Time.now + 3600
    cert.sign(key, OpenSSL::Digest.new("SHA256"))

    ssl_context = OpenSSL::SSL::SSLContext.new
    ssl_context.cert = cert
    ssl_context.key = key
    tcp = TCPServer.new("127.0.0.1", 0)
    @servers << tcp
    ssl_server = OpenSSL::SSL::SSLServer.new(tcp, ssl_context)
    handshakes_completed = 0
    @server_threads << Thread.new do
      loop do
        @conns << ssl_server.accept
        handshakes_completed += 1
      rescue OpenSSL::SSL::SSLError, IOError, SystemCallError
        # The client rejecting the cert aborts the handshake server-side —
        # keep accepting until teardown closes the listener.
        break if tcp.closed?
      end
    end

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.fetch_json(nil, "https://127.0.0.1:#{tcp.addr[1]}/doc", timeout: TIMEOUT)
    end

    assert_equal "network", error.type
    assert error.retryable
    assert_match(/certificate|SSL/i, error.message)
    assert_equal 0, handshakes_completed, "the client must abort the handshake, not complete it"
  end

  def test_non_http_scheme_fails_closed_as_validation_error
    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.stream_http(:get, "ftp://127.0.0.1/doc", timeout: TIMEOUT)
    end
    assert_equal "validation", error.type
  end

  def test_bracketed_ipv6_host_header_keeps_its_brackets
    # Net::HTTP derives Host from the bracket-stripped connect address,
    # emitting the invalid "Host: ::1:PORT" — the transport must send the
    # RFC 3986 authority form for bracketed IPv6 endpoints.
    begin
      server = TCPServer.new("::1", 0)
    rescue Errno::EADDRNOTAVAIL, Errno::EAFNOSUPPORT
      skip "IPv6 loopback unavailable"
    end
    @servers << server
    seen = Queue.new
    @server_threads << Thread.new do
      loop do
        conn = server.accept
        @conns << conn
        lines = []
        while (line = conn.gets) && line != "\r\n"
          lines << line
        end
        seen << lines.join
        conn.write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}")
      rescue IOError, SystemCallError
        break
      end
    end
    port = server.addr[1]

    status, = Basecamp::Oauth::Fetcher.stream_http(:get, "http://[::1]:#{port}/doc", timeout: TIMEOUT)
    assert_equal 200, status
    request_headers = seen.pop
    assert_match(/^Host: \[::1\]:#{port}\r?$/i, request_headers)
  end

  def test_caller_headers_cannot_override_identity_encoding
    # The identity Accept-Encoding is the compression-bomb bound — a caller
    # override must be dropped, matching the Python transport.
    seen = Queue.new
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    @server_threads << Thread.new do
      loop do
        conn = server.accept
        @conns << conn
        lines = []
        while (line = conn.gets) && line != "\r\n"
          lines << line
        end
        seen << lines.join
        conn.write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}")
      rescue IOError, SystemCallError
        break
      end
    end
    endpoint = "http://127.0.0.1:#{server.addr[1]}"

    Basecamp::Oauth::Fetcher.stream_http(
      :get, "#{endpoint}/doc",
      headers: { "Accept-Encoding" => "gzip", "X-Custom" => "kept" },
      timeout: TIMEOUT
    )

    request_headers = seen.pop
    assert_match(/^Accept-Encoding: identity\r?$/i, request_headers)
    assert_no_match(/gzip/i, request_headers)
    assert_match(/^X-Custom: kept\r?$/i, request_headers)
  end

  def test_form_with_get_fails_fast
    # A GET-with-body contradicts the method contract and masks call-site
    # mistakes — reject before any connection.
    error = assert_raises(ArgumentError) do
      Basecamp::Oauth::Fetcher.stream_http(:get, "http://127.0.0.1:9/doc", form: { "a" => "b" }, timeout: TIMEOUT)
    end
    assert_match(/form is only valid with :post/, error.message)
  end

  def test_fetch_json_buffered_injected_adapter_still_yields_the_body
    # Faraday's test adapter BUFFERS and never invokes on_data: the streamed
    # chunks are empty while the body sits on the response. The fallback must
    # hand the document to the parser instead of a bogus empty-body failure.
    stubs = Faraday::Adapter::Test::Stubs.new do |stub|
      stub.get("/doc") { [ 200, { "Content-Type" => "application/json" }, '{"ok":true}' ] }
    end
    connection = Faraday.new { |f| f.adapter :test, stubs }

    doc = Basecamp::Oauth::Fetcher.fetch_json(connection, "https://example.test/doc", timeout: 1)
    assert_equal({ "ok" => true }, doc)
  end

  def test_fetch_json_injected_streaming_non_2xx_is_status_first_api_error
    # A streaming injected adapter (net_http via WebMock) classifies a non-2xx
    # at header time through the SkipBody seam; the observable stays the
    # status-only api_error.
    WebMock.enable!
    WebMock.disable_net_connect!
    stub_request(:get, "https://example.test/doc").to_return(status: 500, body: "x" * 1000)
    connection = Faraday.new

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.fetch_json(connection, "https://example.test/doc", timeout: 1)
    end
    assert_equal "api_error", error.type
    assert_equal 500, error.http_status
  ensure
    WebMock.disable!
  end

  def test_unsupported_method_fails_fast
    # A typo'd verb must not silently become a GET.
    assert_raises(ArgumentError) do
      Basecamp::Oauth::Fetcher.stream_http(:put, "https://issuer.example/x", timeout: TIMEOUT)
    end
  end

  def test_hostless_url_fails_closed_as_validation_error
    # "https:foo" passes the scheme-only HTTPS guard but parses with a nil
    # hostname; without the explicit check it surfaced as a raw ArgumentError
    # from inside Net::HTTP — outside the transport's error contract.
    [ "https:foo", "https://", "http:" ].each do |url|
      error = assert_raises(Basecamp::Oauth::OauthError, url) do
        Basecamp::Oauth::Fetcher.stream_http(:post, url, form: { "a" => "b" }, timeout: TIMEOUT)
      end
      assert_equal "validation", error.type, url
      assert_match(/no host/i, error.message)
    end
  end

  def test_malformed_http_response_maps_to_transport_error
    # A non-HTTP peer (garbage status line) raises Net::HTTPBadResponse — a bare
    # StandardError subclass that must be mapped, or it leaks raw from the
    # public discovery/device APIs instead of the documented network error.
    endpoint, = start_server do |conn|
      conn.write("NOT-HTTP GARBAGE\r\n\r\n")
      conn.close
    end

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.fetch_json(nil, "#{endpoint}/doc", timeout: TIMEOUT)
    end

    assert_equal "network", error.type
    assert error.retryable
  end

  def test_compressed_bodies_are_never_inflated_by_the_transport
    # Transparent decompression would let a compression bomb balloon past the
    # byte cap BEFORE the per-chunk check ran (Net::HTTP inflates before
    # read_body yields). The transport requests identity and disables
    # decoding: a server compressing anyway hands over raw bytes bounded by
    # the cap, and classification (here: a JSON parse failure) happens on the
    # small compressed payload — memory never exceeds the advertised bound.
    require "stringio"
    gz = StringIO.new
    writer = Zlib::GzipWriter.new(gz)
    writer.write("x" * 10_000_000) # ~10 MB decoded
    writer.close
    compressed = gz.string
    assert_operator compressed.bytesize, :<, 20_000, "bomb premise: tiny on the wire"

    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\nContent-Length: #{compressed.bytesize}\r\n\r\n#{compressed}")
    end

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover(endpoint)
    end
    # The RAW bytes flowed through under the cap and failed JSON parsing —
    # never a BodyTooLarge/decoded blow-up (both would be api_error, but the
    # parse failure proves no inflation happened).
    assert_equal "api_error", error.type
    # scrub: the parse-failure message can embed raw compressed bytes.
    assert_match(/parse|JSON/i, error.message.scrub)
  end

  def test_classify_stream_error_maps_every_forced_close_shape_after_deadline
    # Deterministic exception-shape matrix: once the watchdog deadline fired,
    # the forced close can surface as IOError, SystemCallError, or SocketError
    # depending on platform/read phase — every shape must classify as the
    # timeout (the poll's transient-backoff signal), and as a connection
    # failure before the deadline. The live-socket test exercises only the
    # common IOError path.
    shapes = [
      IOError.new("closed stream"),
      Errno::ECONNRESET.new,
      Errno::EBADF.new,
      SocketError.new("socket closed"),
      Net::HTTPBadResponse.new("wrong status line"),
      Zlib::DataError.new("invalid stored block lengths")
    ]

    shapes.each do |shape|
      after = Basecamp::Oauth::Fetcher.classify_stream_error(shape, true)
      assert_kind_of Faraday::TimeoutError, after, "post-deadline #{shape.class} must be a timeout"

      before = Basecamp::Oauth::Fetcher.classify_stream_error(shape, false)
      assert_kind_of Faraday::ConnectionFailed, before, "pre-deadline #{shape.class} must be a connection failure"
    end
  end

  def test_proxy_connect_drip_is_bounded_by_the_deadline
    # With an ENV-configured proxy and an HTTPS endpoint, Net::HTTP parses the
    # proxy's CONNECT response BEFORE it marks the session started: the
    # watchdog cannot close the connecting socket (finish raises IOError), and
    # the per-read timeout resets on every dripped byte. A proxy dripping the
    # CONNECT response below read_timeout must be cut by the whole-phase
    # deadline bound, not left to per-read timeouts that never fire.
    proxy = TCPServer.new("127.0.0.1", 0)
    @servers << proxy
    @server_threads << Thread.new do
      loop do
        conn = proxy.accept
        @conns << conn
        while (line = conn.gets) && line != "\r\n"; end
        # Drip the CONNECT response one byte at a time, each gap well below
        # the 0.5s per-read timeout, for ~7.6s — far past the 0.5s deadline.
        "HTTP/1.1 200 Connection Established\r\n\r\n".each_char do |ch|
          conn.write(ch)
          sleep(0.2)
        end
      rescue IOError, SystemCallError
        break
      end
    end

    # Net::HTTP's :ENV proxy detection builds an http:// URI for the target
    # and calls find_proxy on it, so it reads http_proxy even for TLS requests.
    proxy_env = %w[http_proxy HTTP_PROXY https_proxy HTTPS_PROXY no_proxy NO_PROXY]
    saved = ENV.to_h.slice(*proxy_env)
    proxy_env.each { |k| ENV.delete(k) }
    ENV["http_proxy"] = "http://127.0.0.1:#{proxy.addr[1]}"
    begin
      took = elapsed do
        assert_raises(Faraday::TimeoutError) do
          Basecamp::Oauth::Fetcher.stream_http(:get, "https://proxy-drip.test/token", timeout: 0.5)
        end
      end
      assert_operator took, :<, 2.0, \
        "CONNECT parsing must be cut at the ~0.5s deadline; per-read timeouts alone let the drip run ~8s"
    ensure
      proxy_env.each { |k| ENV.delete(k) }
      saved.each { |k, v| ENV[k] = v }
    end
  end

  def test_watchdog_close_cannot_mint_a_second_connection
    # Once the watchdog closes the session post-deadline, Net::HTTP#request's
    # implicit re-start must be refused outright (the restart runs outside the
    # connect Timeout.timeout, where an ENV-proxied CONNECT could drip
    # unbounded). Same deterministic hold harness as the reopen test — but the
    # guarded start now raises, and CRITICALLY no second connection is minted.
    accepts = []
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    @server_threads << Thread.new do
      loop do
        conn = server.accept
        accepts << conn
        @conns << conn
        begin
          while (line = conn.gets) && line != "\r\n"; end
          conn.write("HTTP/1.1 302 Found\r\nLocation: https://x/\r\nConnection: close\r\n\r\n")
          conn.close
        rescue IOError, SystemCallError
          # Connection #1 dies under the watchdog's close; keep accepting.
        end
      rescue IOError, SystemCallError
        break # listener closed in teardown
      end
    end
    endpoint = "http://127.0.0.1:#{server.addr[1]}"

    instrumented = Class.new(Net::HTTP) do
      def hold_next_request!
        @hold_next_request = true
      end

      def request(req, body = nil, &block)
        if @hold_next_request
          @hold_next_request = false
          sleep(0.005) while started?
        end
        super
      end
    end

    uri = URI.parse(endpoint)
    http = instrumented.new(uri.hostname, uri.port)
    http.hold_next_request!

    original_new = Net::HTTP.method(:new)
    Net::HTTP.define_singleton_method(:new) { |*| http }
    begin
      assert_raises(Faraday::TimeoutError) do
        Basecamp::Oauth::Fetcher.stream_http(
          :get, "#{endpoint}/token", timeout: 0.3,
          skip_status: ->(s) { (300..399).cover?(s) }
        )
      end
    ensure
      Net::HTTP.define_singleton_method(:new, original_new)
    end
    assert_equal 1, accepts.length, "the post-deadline implicit re-start must never mint a second connection"
  end

  def test_watchdog_close_racing_the_request_cannot_reopen_past_the_deadline
    # Net::HTTP#request implicitly re-starts a finished session. If the
    # watchdog's close lands between stream_http's post-connect deadline
    # re-check and the request write, the request would ride a fresh
    # post-deadline connection — and a bodyless response would surface as
    # SUCCESS after the wall clock expired. Lose the race deterministically:
    # hold the request until the watchdog has finished the session, exactly
    # the window the race occupies. Not start_server: its accept loop dies on
    # the watchdog-closed first connection, and the reopened connection must
    # get a real, instant response for the un-fixed code to "succeed".
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    @server_threads << Thread.new do
      loop do
        conn = server.accept
        @conns << conn
        begin
          while (line = conn.gets) && line != "\r\n"; end
          conn.write("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n")
          conn.close
        rescue IOError, SystemCallError
          # Connection #1 dies under the watchdog's close; keep accepting.
        end
      rescue IOError, SystemCallError
        break # listener closed in teardown
      end
    end
    endpoint = "http://127.0.0.1:#{server.addr[1]}"

    instrumented = Class.new(Net::HTTP) do
      def hold_next_request!
        @hold_next_request = true
      end

      def request(req, body = nil, &block)
        if @hold_next_request
          @hold_next_request = false
          sleep(0.005) while started?
        end
        super
      end
    end

    uri = URI.parse(endpoint)
    http = instrumented.new(uri.hostname, uri.port)
    http.hold_next_request!

    # Hand the instrumented instance to stream_http (minitest 6 dropped
    # minitest/mock, so restore the singleton by hand).
    original_new = Net::HTTP.method(:new)
    Net::HTTP.define_singleton_method(:new) { |*| http }
    begin
      # Either deadline classification is correct: the response-block re-check
      # raises the bounded-read marker, and the persistent watchdog loop may
      # close the reopened connection mid-flight first (surfacing as timeout).
      assert_raises(Basecamp::Oauth::Fetcher::ReadDeadlineExceeded, Faraday::TimeoutError) do
        Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/token", timeout: 0.3)
      end
    ensure
      Net::HTTP.define_singleton_method(:new, original_new)
    end
  end

  def test_completed_response_is_never_accepted_past_the_deadline
    # The narrow race the watchdog cannot cover: the last body chunk lands
    # just before the deadline and the completion lands just after it, with
    # the request thread processing completion before the watchdog sets
    # deadline_fired. The instrumented request tail-sleep makes the ordering
    # deterministic: the response completes cleanly, then the deadline passes
    # before stream_http can return — the final monotonic re-check must
    # refuse the completed response.
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    @server_threads << Thread.new do
      loop do
        conn = server.accept
        @conns << conn
        while (line = conn.gets) && line != "\r\n"; end
        conn.write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}")
      rescue IOError, SystemCallError
        break
      end
    end
    endpoint = "http://127.0.0.1:#{server.addr[1]}"

    instrumented = Class.new(Net::HTTP) do
      def request(req, body = nil, &block)
        result = super
        sleep(0.4) # completion processed; deadline passes before returning
        result
      end
    end

    uri = URI.parse(endpoint)
    http = instrumented.new(uri.hostname, uri.port)
    original_new = Net::HTTP.method(:new)
    Net::HTTP.define_singleton_method(:new) { |*| http }
    begin
      assert_raises(Basecamp::Oauth::Fetcher::ReadDeadlineExceeded) do
        Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/token", timeout: 0.2)
      end
    ensure
      Net::HTTP.define_singleton_method(:new, original_new)
    end
  end

  def test_watchdog_kill_cannot_leak_a_half_closed_socket
    # Completion racing the deadline: the watchdog can be INSIDE http.finish —
    # after do_finish clears started? but before the socket close — when the
    # ensure's kill fires. Un-fixed, the thread dies mid-close and the request
    # side skips its own close (started? is already false): the socket leaks
    # until GC. The instrumented subclass widens that window so the kill lands
    # inside it deterministically; the deferred-interrupt fix must still
    # complete the close before the thread dies.
    # The shape needs a KEEP-ALIVE response that completes cleanly (any raise
    # inside the response block makes Net::HTTP's own transport rescue close
    # the socket) with the deadline firing between the response block and the
    # request-side ensure — the instrumented request tail-sleep pins the main
    # thread there while the deadline fires.
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    @server_threads << Thread.new do
      loop do
        conn = server.accept
        @conns << conn
        while (line = conn.gets) && line != "\r\n"; end
        conn.write("HTTP/1.1 204 No Content\r\n\r\n") # keep-alive: socket survives the request
      rescue IOError, SystemCallError
        break
      end
    end
    endpoint = "http://127.0.0.1:#{server.addr[1]}"

    sockets = []
    instrumented = Class.new(Net::HTTP) do
      define_method(:do_finish) do
        @started = false
        sockets << @socket if @socket
        sleep(0.5) # widened started?-cleared -> socket-close window
        @socket&.close
        @socket = nil
      end

      define_method(:request) do |req, body = nil, &block|
        result = super(req, body, &block)
        sleep(0.2) # let the deadline fire before the request-side ensure runs
        result
      end
    end

    uri = URI.parse(endpoint)
    http = instrumented.new(uri.hostname, uri.port)
    original_new = Net::HTTP.method(:new)
    Net::HTTP.define_singleton_method(:new) { |*| http }
    begin
      # The final post-request deadline re-check now refuses the completed
      # 204 (it finished past the wall clock); the leak property under test
      # is unchanged — the watchdog's close must still complete.
      assert_raises(Basecamp::Oauth::Fetcher::ReadDeadlineExceeded) do
        Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/token", timeout: 0.1)
      end
    ensure
      Net::HTTP.define_singleton_method(:new, original_new)
    end

    assert_not_empty sockets, "watchdog never entered the close window"
    assert sockets.all?(&:closed?), "the deadline-racing close must complete: no socket may leak"
  end

  def test_unnormalized_timeout_fails_closed_before_any_request
    # The operation entry points normalize; the primitive still fails closed
    # for direct callers — a non-finite, non-positive, or beyond-ceiling
    # timeout would leave the socket timeouts and watchdog sleep unbounded.
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    accepted = []
    @server_threads << Thread.new do
      loop do
        accepted << server.accept
        @conns << accepted.last
      rescue IOError, SystemCallError
        break
      end
    end
    endpoint = "http://127.0.0.1:#{server.addr[1]}"

    [ Float::INFINITY, Float::NAN, 0, -1, 3601, nil, "30" ].each do |timeout|
      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/token", timeout: timeout)
      end
      assert_equal "validation", error.type, "timeout=#{timeout.inspect}"
    end
    assert_empty accepted, "the guard must reject before any connection"
  end

  def test_watchdog_threads_do_not_leak
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}")
    end

    # Warm-up request BEFORE the baseline: the process's first Net::HTTP
    # connect can lazily spawn Ruby's persistent Timeout worker thread (one
    # per process, never reaped). Whether that already happened depends on
    # randomized suite order, so counting it after the baseline made this
    # assertion flake by exactly +1 on the orderings where this test ran
    # first. The warm-up folds any lazily-created runtime threads into the
    # baseline; the assertion then counts only threads OUR primitive leaks.
    Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/doc", timeout: TIMEOUT)

    baseline = Thread.list.length
    5.times do
      Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/doc", timeout: TIMEOUT)
    end
    # The watchdog is killed and JOINED in the primitive's ensure, so no request
    # leaves a thread behind.
    assert_operator Thread.list.length, :<=, baseline
  end
end
