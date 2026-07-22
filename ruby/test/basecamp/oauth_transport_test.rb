# frozen_string_literal: true

require "test_helper"
require "openssl"
require "socket"

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

  def test_watchdog_threads_do_not_leak
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}")
    end

    baseline = Thread.list.length
    5.times do
      Basecamp::Oauth::Fetcher.stream_http(:get, "#{endpoint}/doc", timeout: TIMEOUT)
    end
    # The watchdog is killed and JOINED in the primitive's ensure, so no request
    # leaves a thread behind.
    assert_operator Thread.list.length, :<=, baseline
  end
end
