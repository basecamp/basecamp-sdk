# frozen_string_literal: true

require "test_helper"
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
  end

  def teardown
    @servers.each { |server| server.close rescue nil }
    WebMock.enable!
    WebMock.disable_net_connect!
  end

  # Starts a real TCP server; the handler receives each accepted socket after
  # the request headers have been consumed. Returns [endpoint, accepts] where
  # +accepts+ counts connections — the zero-retry assertions read it.
  def start_server(&handler)
    server = TCPServer.new("127.0.0.1", 0)
    @servers << server
    accepts = []
    Thread.new do
      loop do
        conn = server.accept
        accepts << conn
        while (line = conn.gets) && line != "\r\n"; end
        handler.call(conn)
      rescue IOError, Errno::EBADF
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
      rescue Errno::EPIPE, IOError, Errno::ECONNRESET
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
        rescue IOError, Errno::EPIPE
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

  def test_body_slow_drip_is_bounded_for_discovery
    # A wanted (2xx) body dripped forever: the read-loop deadline bounds it and
    # discovery surfaces its retryable network timeout.
    endpoint, = start_server do |conn|
      conn.write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n")
      loop do
        conn.write("x")
        sleep 0.1
      rescue IOError, Errno::EPIPE
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
      rescue IOError, Errno::EPIPE
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
