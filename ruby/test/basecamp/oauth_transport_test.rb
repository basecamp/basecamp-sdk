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
    # its socket well past the test's end. Close the LISTENERS first: a
    # thread blocked in accept is unblocked deterministically by the close,
    # where Thread#kill alone can hang the join mid-syscall on some
    # platforms/builds.
    @servers.each { |server| server.close rescue nil }
    @server_threads.each(&:kill).each(&:join)
    @conns.each { |conn| conn.close rescue nil }
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

  PROXY_ENV_KEYS = %w[http_proxy HTTP_PROXY https_proxy HTTPS_PROXY no_proxy NO_PROXY].freeze

  # TEST-NET-3 (RFC 5737). Documentation-only and never routable — but what
  # matters to find_proxy is only that it is NOT loopback, so the proxy is
  # kept rather than skipped.
  STUB_TARGET_ADDRESS = "203.0.113.1"

  # Runs the block with ONLY +vars+ among the proxy environment variables, and
  # — unless +resolves_to+ is nil — with target-name resolution stubbed.
  #
  # The stub is the point (#720). {Fetcher.stream_http} resolves the proxy with
  # URI#find_proxy INSIDE the advertised deadline, and find_proxy calls
  # IPSocket.getaddress(hostname) to evaluate its loopback rule. Every proxy
  # test here targets a `.test` name, which RFC 6761 reserves as never
  # resolvable — so that call is a REAL round trip to a resolver that has to
  # answer NXDOMAIN, and on a loaded runner (or one with search-domain
  # expansion across several nameservers) it can consume the whole 0.5-1s
  # budget before the behavior under test is even reachable.
  #
  # Both failures recorded on #720 are that one cause wearing two masks: the
  # validation error arrives as Faraday::TimeoutError, or the proxy never sees
  # a connection at all and the mechanism queue stays empty. Neither is a
  # timing assertion that needs widening; the resolver simply does not belong
  # inside the deadline these tests are measuring. Stubbing getaddress removes
  # it while preserving the only thing find_proxy asks of it — a non-loopback
  # answer.
  #
  # Tests that are ABOUT resolution pass resolves_to: nil and install their
  # own stub, so this one never sits underneath theirs.
  def with_proxy_env(vars, resolves_to: STUB_TARGET_ADDRESS)
    saved = ENV.to_h.slice(*PROXY_ENV_KEYS)
    PROXY_ENV_KEYS.each { |key| ENV.delete(key) }
    vars.each { |key, value| ENV[key] = value }

    real_getaddress = IPSocket.method(:getaddress)
    IPSocket.define_singleton_method(:getaddress) { |_host| resolves_to } if resolves_to
    begin
      yield
    ensure
      # Restore by re-delegating, not by remove_method: getaddress is defined
      # directly on IPSocket's singleton class, so removing the override would
      # delete the original rather than uncover it.
      IPSocket.define_singleton_method(:getaddress) { |host| real_getaddress.call(host) } if resolves_to
      PROXY_ENV_KEYS.each { |key| ENV.delete(key) }
      saved.each { |key, value| ENV[key] = value }
    end
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

  def test_invalid_max_body_bytes_fails_closed_as_validation_error
    # The cap IS the streaming bound this transport exists to provide — nil,
    # a float, a bool, or a negative value would disable or crash it, so
    # misuse rejects before any connection.
    [ nil, -1, 1.5, true, Float::INFINITY ].each do |cap|
      error = assert_raises(Basecamp::Oauth::OauthError, "cap=#{cap.inspect}") do
        Basecamp::Oauth::Fetcher.stream_http(:get, "http://127.0.0.1:9/doc", timeout: TIMEOUT, max_body_bytes: cap)
      end
      assert_equal "validation", error.type, "cap=#{cap.inspect}"
      assert_match(/max_body_bytes must be a non-negative Integer/, error.message)
    end
  end

  def test_zero_max_body_bytes_is_a_strict_cap_not_a_validation_error
    # normalize_body_cap accepts zero as a legitimate strict cap, so the
    # transport must too: the request goes out and any non-empty body trips
    # the bound, surfacing as the documented cap fault — never "validation".
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nhi")
    end

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.fetch_json(nil, "#{endpoint}/doc", timeout: TIMEOUT, max_body_bytes: 0)
    end

    assert_equal "api_error", error.type
    assert_match(/size cap/i, error.message)
  end

  def test_bracketed_ipv6_host_header_keeps_its_brackets
    # Net::HTTP derives Host from the bracket-stripped connect address,
    # emitting the invalid "Host: ::1:PORT" — the transport must send the
    # RFC 3986 authority form for bracketed IPv6 endpoints.
    server = nil
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

  def test_skip_status_headers_past_the_deadline_classify_as_timeout
    # Headers that become runnable past the monotonic deadline — but before
    # the watchdog flips deadline_fired — mean the status was NOT known in
    # time: the skip fast-path must surface the timeout, never race into a
    # status classification (matching the Python transport). The skip
    # callable itself flips Fetcher's stubbed clock, so exactly the reads
    # AFTER header arrival see a past-deadline "now".
    served = false
    real = Basecamp::Oauth::Fetcher.method(:monotonic_now)
    skip = lambda do |status|
      served = true
      !(200..299).cover?(status)
    end

    # The suite does not load minitest/mock — swap the singleton by hand and
    # restore it in ensure (the established idiom in this file).
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) do
      served ? 1e12 : real.call
    end

    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 500 Nope\r\nContent-Length: 2\r\nConnection: close\r\n\r\nno")
    end

    assert_raises(Basecamp::Oauth::Fetcher::ReadDeadlineExceeded) do
      Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/doc", timeout: 5, skip_status: skip)
    end
  ensure
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) { real.call }
  end

  def test_completed_response_survives_the_watchdog_finish_race
    # The watchdog's cross-thread finish can nil @socket while the request
    # thread is inside Net::HTTP's own end_transport, whose @socket.closed?
    # then raises NoMethodError. An in-deadline COMPLETED response must
    # dominate that cleanup race (mirroring the Python transport), never
    # leak the raw NoMethodError. end_transport is delayed past the 0.5s
    # watchdog to land in the race window deterministically.
    real = Net::HTTP.instance_method(:end_transport)
    Net::HTTP.send(:define_method, :end_transport) do |*args|
      sleep(0.9)
      real.bind_call(self, *args)
    end

    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nhi")
    end

    status, body = Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/doc", timeout: 0.5)
    assert_equal 200, status
    assert_equal "hi", body
  ensure
    Net::HTTP.send(:define_method, :end_transport, real)
  end

  def test_tls_failure_past_the_deadline_classifies_as_timeout
    # The watchdog's forced close during a TLS operation surfaces as an
    # SSLError, not a timeout — past the monotonic deadline it must classify
    # as the timeout it raced, or the device poll terminates instead of
    # backing off. The handler flips Fetcher's stubbed clock before feeding
    # the handshake garbage, so the SSLError is observed strictly after it.
    state = { past_deadline: false }
    real = Basecamp::Oauth::Fetcher.method(:monotonic_now)
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) do
      state[:past_deadline] ? 1e12 : real.call
    end

    tcp = TCPServer.new("127.0.0.1", 0)
    port = tcp.addr[1]
    thread = Thread.new do
      conn = tcp.accept
      state[:past_deadline] = true
      # Non-TLS bytes during the handshake surface client-side as an
      # OpenSSL::SSL::SSLError ("wrong version number"), never a timeout.
      conn.write("HTTP/1.1 200 OK\r\n\r\n")
      conn.close
    rescue IOError
      nil
    end

    assert_raises(Faraday::TimeoutError) do
      Basecamp::Oauth::Fetcher.stream_http(:get, "https://127.0.0.1:#{port}/doc", timeout: 5)
    end
  ensure
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) { real.call }
    thread&.kill&.join
    tcp&.close
  end

  def test_wire_error_past_the_deadline_classifies_as_timeout
    # A peer reset observed past the monotonic deadline — before the watchdog
    # thread is scheduled to flip deadline_fired — is the timeout it raced:
    # the device poll retries only Faraday::TimeoutError, so classifying it
    # as ConnectionFailed would terminate polling. The handler flips
    # Fetcher's stubbed clock before resetting, so the classification read
    # sees a past-deadline now while the watchdog flag is still false.
    state = { past_deadline: false }
    real = Basecamp::Oauth::Fetcher.method(:monotonic_now)
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) do
      state[:past_deadline] ? 1e12 : real.call
    end

    endpoint, = start_server do |conn|
      state[:past_deadline] = true
      # An abrupt RST mid-request surfaces as a wire error, never a timeout.
      conn.setsockopt(Socket::SOL_SOCKET, Socket::SO_LINGER, [ 1, 0 ].pack("ii"))
      conn.close
    end

    assert_raises(Faraday::TimeoutError) do
      Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/doc", timeout: 5)
    end
  ensure
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) { real.call }
  end

  def test_fetch_json_buffered_oversized_error_body_is_the_status_fault
    # fetch_json discards non-2xx bodies (status dominates): a buffered adapter
    # whose oversized ERROR body is already in memory must surface the status
    # api_error, not a size-cap fault — the body is never read or copied.
    stubs = Faraday::Adapter::Test::Stubs.new do |stub|
      stub.get("/doc") { [ 500, { "Content-Type" => "text/html" }, "x" * 2048 ] }
    end
    connection = Faraday.new { |f| f.adapter :test, stubs }

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.fetch_json(connection, "https://example.test/doc", timeout: 1, max_body_bytes: 1024)
    end

    assert_equal "api_error", error.type
    assert_equal 500, error.http_status
    assert_match(/status 500/, error.message)
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

  def test_injected_skip_status_past_the_deadline_classifies_as_timeout
    # The injected-Faraday bounded_reader gates the SkipBody fast-path on the
    # monotonic deadline: a first chunk that becomes runnable past the total
    # bound means the status was not known in time — the timeout must win,
    # matching the default transport's header-time gate. The stubbed clock
    # jumps once WebMock is serving, so the on_data callback observes a
    # past-deadline now while the per-read timeout would have admitted it.
    state = { past_deadline: false }
    real = Basecamp::Oauth::Fetcher.method(:monotonic_now)
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) do
      state[:past_deadline] ? 1e12 : real.call
    end

    WebMock.enable!
    WebMock.disable_net_connect!
    stub_request(:get, "https://example.test/doc").to_return do
      state[:past_deadline] = true
      { status: 500, body: "x" * 1000 }
    end
    connection = Faraday.new

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Fetcher.fetch_json(connection, "https://example.test/doc", timeout: 1)
    end
    assert_equal "network", error.type
    assert error.retryable
  ensure
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) { real.call }
    WebMock.disable!
  end

  def test_injected_dispatch_refused_when_budget_already_spent
    # The wall-clock wrap must grant the REMAINING budget, not a fresh full
    # timeout: when the deadline has already passed by dispatch time (thread
    # descheduled after the deadline was captured), the GET must be refused
    # rather than handed a whole new request window. The stubbed clock jumps
    # after the deadline capture, so the remaining budget is negative at
    # dispatch.
    calls = 0
    real = Basecamp::Oauth::Fetcher.method(:monotonic_now)
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) do
      calls += 1
      calls == 1 ? real.call : real.call + 10
    end
    dispatched = false
    client = Object.new
    client.define_singleton_method(:get) { |_url| dispatched = true }

    assert_raises(Faraday::TimeoutError) do
      Basecamp::Oauth::Fetcher.faraday_fetch(
        client, "https://example.test/doc", timeout: 1, max_body_bytes: 1024
      )
    end
    assert_equal false, dispatched, "GET must not dispatch on an exhausted budget"
  ensure
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) { real.call }
  end

  def test_injected_2xx_completing_past_the_deadline_is_refused
    # Timeout.timeout's interrupt can be delivered late: simulate that race
    # with a client that completes a 200 while the stubbed clock has moved
    # past the deadline. The post-return monotonic re-check must refuse the
    # late 2xx as a transport-shaped timeout — a completed non-2xx stays
    # status-classified (status outranks the deadline race).
    state = { late: false }
    real = Basecamp::Oauth::Fetcher.method(:monotonic_now)
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) do
      state[:late] ? real.call + 10 : real.call
    end
    response = Struct.new(:status, :body).new(200, +"{}")
    client = Object.new
    client.define_singleton_method(:get) do |_url|
      state[:late] = true
      response
    end

    assert_raises(Faraday::TimeoutError) do
      Basecamp::Oauth::Fetcher.faraday_fetch(
        client, "https://example.test/doc", timeout: 1, max_body_bytes: 1024
      )
    end
  ensure
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :monotonic_now) { real.call }
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

  def test_proxy_credentials_are_percent_decoded
    # URI#user/#password return the percent-encoded forms, and the
    # explicit-proxy Net::HTTP.new does not unescape them the way its :ENV
    # mode does — the CONNECT's Proxy-Authorization must carry the DECODED
    # credentials or authenticated proxies reject the request.
    proxy = TCPServer.new("127.0.0.1", 0)
    @servers << proxy
    # The preamble is HANDED ACROSS a Queue rather than shared as a mutable
    # String (#739). The old form appended to a `captured = +""` that the main
    # thread read the instant assert_raises returned, with no join, queue or
    # condvar between them — it was ordered only in practice, by the socket
    # close, and the client's own `timeout: 1` can fire while the proxy is
    # still mid-gets on a loaded runner. That failure reads as absent
    # credentials, which points at percent-decoding, which is not the bug.
    #
    # Queue rather than Thread#join: join also establishes the happens-before
    # edge, but a join(timeout) that returns nil leaves you reading the shared
    # String with no edge at all — the race re-enters through the hang guard.
    # Here the VALUE's existence is the evidence: it can only be popped if the
    # producer finished reading the preamble, so a wedged producer is a
    # distinguishable, explicitly-reported failure instead of an empty match.
    # Same idiom as this file's other header-capture tests.
    captured = Queue.new
    @server_threads << Thread.new do
      conn = proxy.accept
      @conns << conn
      lines = []
      while (line = conn.gets) && line != "\r\n"
        lines << line
      end
      captured << lines.join
      conn.close
    rescue IOError, SystemCallError
      nil
    end

    # p%40s+s: the %40 percent-decodes to @, while the literal + must stay a
    # plus (userinfo is percent-decoded, never form-decoded).
    with_proxy_env({ "https_proxy" => "http://user:p%40s+s@127.0.0.1:#{proxy.addr[1]}" }) do
      assert_raises(Faraday::Error) do
        Basecamp::Oauth::Fetcher.stream_http(:get, "https://proxy-auth.test/token", timeout: 1)
      end
      # A WAIT bound, not a timing assertion: this test is about content, so a
      # generous wait costs nothing (unlike the sub-second bounds #734 kept on
      # the siblings, where the margin between correct and broken IS the
      # deadline).
      preamble = captured.pop(timeout: 15)
      assert preamble, \
        "the proxy never finished reading the CONNECT preamble — nothing was handed over to assert against"
      expected = [ "user:p@s+s" ].pack("m0")
      assert_includes preamble, "Proxy-Authorization: Basic #{expected}"
    end
  end

  def test_proxy_resolution_dns_is_inside_the_deadline
    # find_proxy resolves the target hostname (IPSocket.getaddress) to
    # evaluate its loopback rule — a stalled resolver must be cut by the
    # advertised bound, not hold the request open before any deadline exists.
    resolver_finished = false
    real = IPSocket.method(:getaddress)
    IPSocket.define_singleton_method(:getaddress) do |_host|
      sleep(5)
      resolver_finished = true
      # A fixed non-loopback answer, never the real resolver: the stall IS the
      # subject here, and the un-cut path must not go on to make a live lookup.
      STUB_TARGET_ADDRESS
    end

    # resolves_to: nil — this test is ABOUT resolution, so it keeps its own
    # (deterministic, already-stubbed) resolver rather than sitting on top of
    # with_proxy_env's.
    begin
      with_proxy_env({ "https_proxy" => "http://127.0.0.1:9" }, resolves_to: nil) do
        took = elapsed do
          assert_raises(Faraday::TimeoutError) do
            Basecamp::Oauth::Fetcher.stream_http(:get, "https://dns-stall.test/token", timeout: 0.5)
          end
        end
        # Assert the MECHANISM, not wall clock (#708): the find_proxy bound's
        # asynchronous raise interrupts the resolver stub MID-SLEEP, so its
        # completion flag never flips. Without the bound the call sits out the
        # full 5s stall and the flag flips — which is the observation that does
        # not depend on where the resulting error is raised or how it is
        # classified. (Removing the bound happens to surface
        # ReadDeadlineExceeded from the post-resolution budget check rather
        # than a mapped timeout, so the assert_raises above fires too; the flag
        # is the assertion that stays true to the mechanism either way.)
        assert_not resolver_finished, \
          "stream_http returned only after the stalled resolver ran to completion — the find_proxy bound did not cut it"
        # Hang guard ONLY — generous on purpose: it protects the suite from a
        # wedged resolution phase and asserts nothing about timing tightness.
        assert_operator took, :<, 15, \
          "hang guard, not a timing bound: the stalled resolver should be cut in ~0.5s; #{took.round(2)}s means nothing cut it"
      end
    ensure
      IPSocket.define_singleton_method(:getaddress) { |host| real.call(host) }
    end
  end

  def test_https_proxy_refused_without_p_use_ssl_support
    # On Ruby 3.2/3.3 the bundled net-http lacks Net::HTTP.new's eighth
    # p_use_ssl parameter — an https:// proxy must refuse with an actionable
    # validation error, never plaintext-to-a-TLS-proxy or an ArgumentError.
    real = Basecamp::Oauth::Fetcher.method(:proxy_tls_capable?)
    Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :proxy_tls_capable?) { false }

    begin
      with_proxy_env({ "https_proxy" => "https://127.0.0.1:9" }) do
        error = assert_raises(Basecamp::Oauth::OauthError) do
          Basecamp::Oauth::Fetcher.stream_http(:get, "https://old-net-http.test/token", timeout: 1)
        end
        assert_equal "validation", error.type
        assert_match(/net-http >= 0\.5/, error.message)
      end
    ensure
      Basecamp::Oauth::Fetcher.singleton_class.send(:define_method, :proxy_tls_capable?) { real.call }
    end
  end

  def test_https_proxy_scheme_gets_tls_on_the_proxy_connection
    # An https:// proxy URL means TLS on the PROXY connection itself:
    # Net::HTTP.new's p_use_ssl must be passed through or the transport
    # sends plaintext CONNECT to a TLS-only proxy. The plain TCP listener
    # distinguishes the two by the first bytes: a TLS ClientHello record
    # (0x16 0x03...) with the fix, a plaintext "CONNECT ..." without.
    first_bytes = nil
    server = TCPServer.new("127.0.0.1", 0)
    port = server.addr[1]
    thread = Thread.new do
      conn = server.accept
      first_bytes = conn.readpartial(5)
      conn.close
    rescue IOError, SystemCallError
      nil
    end

    begin
      with_proxy_env({ "https_proxy" => "https://127.0.0.1:#{port}" }) do
        assert_raises(Faraday::Error) do
          Basecamp::Oauth::Fetcher.stream_http(:get, "https://tls-proxy.test/token", timeout: 1)
        end
        thread.join(2)
        assert_equal 0x16, first_bytes&.bytes&.first,
          "expected a TLS ClientHello to the https:// proxy, got #{first_bytes.inspect}"
      end
    ensure
      thread&.kill&.join
      server&.close
    end
  end

  def test_proxy_connect_drip_is_bounded_by_the_deadline
    # With an ENV-configured proxy and an HTTPS endpoint, Net::HTTP parses the
    # proxy's CONNECT response BEFORE it marks the session started: the
    # watchdog cannot close the connecting socket (finish raises IOError), and
    # the per-read timeout resets on every dripped byte. A proxy dripping the
    # CONNECT response below read_timeout must be cut by the whole-phase
    # deadline bound, not left to per-read timeouts that never fire.
    #
    # The drip is ENDLESS: the terminating blank line never arrives, and every
    # 0.2s gap sits well under the 0.5s per-read timeout — so no per-read
    # timeout can ever fire and the call can NEVER return on its own. The only
    # way out is the whole-phase connect bound's asynchronous cut closing the
    # socket MID-DRIP, which the proxy observes as a write error and reports
    # on the queue. (The drip does stop after ~20s — far past every assertion
    # horizon — solely so a broken bound fails the hang guard below instead of
    # wedging the suite.)
    cut = Queue.new
    proxy = TCPServer.new("127.0.0.1", 0)
    @servers << proxy
    @server_threads << Thread.new do
      loop do
        conn = proxy.accept
        @conns << conn
        while (line = conn.gets) && line != "\r\n"; end
        begin
          conn.write("HTTP/1.1 200 Connection Established\r\n")
          100.times do
            conn.write("a")
            sleep(0.2)
          end
          conn.close
        rescue IOError, SystemCallError
          # The rescue must wrap the WHOLE drip loop, not a single write: the
          # first write after the client closes can still land in the socket
          # buffer, and only a later write raises. Reaching here means the
          # client cut the socket mid-drip.
          cut << true
        end
      rescue IOError, SystemCallError
        break # listener closed in teardown
      end
    end

    # The transport resolves the ENV proxy against the REAL scheme (https →
    # https_proxy), unlike Net::HTTP's broken built-in :ENV detection which
    # reads http_proxy even for TLS requests — so this ALSO pins that an
    # https_proxy-only environment routes HTTPS requests through its proxy.
    with_proxy_env({ "https_proxy" => "http://127.0.0.1:#{proxy.addr[1]}" }) do
      took = elapsed do
        # Discriminating on its own now that the drip is endless: without the
        # whole-phase connect bound the call cannot return before the drip's
        # ~20s cap (per-read timeouts never fire between 0.2s gaps, and the
        # watchdog cannot reach a pre-started? session).
        assert_raises(Faraday::TimeoutError) do
          Basecamp::Oauth::Fetcher.stream_http(:get, "https://proxy-drip.test/token", timeout: 0.5)
        end
      end
      # Assert the MECHANISM, not wall clock (#708): the proxy observed the
      # CLIENT cutting the socket mid-drip. Only the whole-phase
      # Timeout.timeout(connect_remaining) bound closes this socket — the
      # watchdog's finish raises IOError until the session is started, and no
      # per-read timeout can fire. A drip that ran out on the proxy's side
      # instead leaves the queue empty. The pop timeout is a WAIT bound, not a
      # timing assertion.
      #
      # #708 sized that wait bound at 15s on the theory that no plausible
      # scheduling delay could exceed it, and #720 recorded it failing anyway.
      # The bound was not the problem: with a slow resolver the request died in
      # find_proxy, the proxy never received a CONNECT at all, and an empty
      # queue is indistinguishable from a late one no matter how long the wait.
      # with_proxy_env removes the resolver, which is what actually closes it —
      # 15s stays as the hang guard #708 meant it to be.
      assert cut.pop(timeout: 15), \
        "the proxy never saw the client cut the CONNECT drip — the whole-phase connect bound did not fire"
      # Hang guard ONLY — generous on purpose: it protects the suite from a
      # wedged connect phase and asserts nothing about timing tightness.
      assert_operator took, :<, 15, \
        "hang guard, not a timing bound: the endless drip should be cut in ~0.5s; #{took.round(2)}s means nothing cut it"
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
