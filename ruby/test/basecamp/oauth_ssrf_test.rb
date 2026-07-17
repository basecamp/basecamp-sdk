# frozen_string_literal: true

require "test_helper"
require "faraday"

# SSRF hardening for OAuth discovery: bounded/streaming body reads and redirect
# suppression, exercised at runtime against a real injected Faraday connection.
class OAuthSsrfTest < Minitest::Test
  include TestHelper

  # A real Faraday adapter that streams a body to the request's +on_data+
  # callback in fixed-size chunks, recording how many bytes were delivered before
  # the consumer aborts. This lets the test prove the read is genuinely bounded —
  # it stops before the whole oversized body is buffered — rather than relying on
  # a post-hoc size check (WebMock delivers a body as a single chunk).
  class StreamingAdapter < Faraday::Adapter
    def initialize(app = nil, body:, chunk_size:, meter:, status: 200)
      super(app)
      @body = body
      @chunk_size = chunk_size
      @meter = meter
      @status = status
    end

    def call(env)
      on_data = env.request.on_data
      if on_data
        deliver_chunked(on_data)
        save_response(env, @status, "", { "Content-Type" => "application/json" })
      else
        save_response(env, @status, @body, { "Content-Type" => "application/json" })
      end
      @app.call(env)
    end

    private

      def deliver_chunked(on_data)
        sent = 0
        while sent < @body.bytesize
          piece = @body.byteslice(sent, @chunk_size)
          sent += piece.bytesize
          @meter[:delivered] = sent
          on_data.call(piece, sent)
        end
      end
  end

  def test_over_cap_body_aborts_before_buffering_whole_body
    issuer = "https://issuer.ssrf-test.example"
    # A well-formed but oversized document: valid JSON padded far past the cap.
    oversized = { "issuer" => issuer, "token_endpoint" => "#{issuer}/t", "pad" => "x" * (256 * 1024) }.to_json

    meter = { delivered: 0 }
    connection = Faraday.new do |conn|
      conn.adapter StreamingAdapter, body: oversized, chunk_size: 4096, meter: meter
    end

    cap = 8 * 1024
    discovery = Basecamp::Oauth::Discovery.new(http_client: connection, max_body_bytes: cap)

    error = assert_raises(Basecamp::Oauth::OauthError) do
      discovery.discover(issuer)
    end
    assert_equal "api_error", error.type

    # The streaming read aborted once the accumulated bytes exceeded the cap, so
    # only a bounded prefix — not the whole 256 KiB body — was ever delivered.
    assert_operator meter[:delivered], :<=, cap + 4096
    assert_operator meter[:delivered], :<, oversized.bytesize
  end

  # Streams tiny chunks with a real pause between each, staying well under the
  # byte cap forever — a slow-drip peer. Records how many bytes were delivered so
  # the test can prove the read aborted on the wall-clock deadline rather than
  # running to completion.
  class SlowDripAdapter < Faraday::Adapter
    def initialize(app = nil, body:, chunk_size:, pause:, meter:)
      super(app)
      @body = body
      @chunk_size = chunk_size
      @pause = pause
      @meter = meter
    end

    def call(env)
      on_data = env.request.on_data
      if on_data
        sent = 0
        while sent < @body.bytesize
          piece = @body.byteslice(sent, @chunk_size)
          sent += piece.bytesize
          @meter[:delivered] = sent
          sleep @pause # simulate a peer trickling data below the read timeout
          on_data.call(piece, sent)
        end
        save_response(env, 200, "", { "Content-Type" => "application/json" })
      else
        save_response(env, 200, @body, { "Content-Type" => "application/json" })
      end
      @app.call(env)
    end
  end

  def test_slow_drip_stream_aborts_on_wall_clock_deadline
    issuer = "https://issuer.ssrf-test.example"
    # A modest, in-cap body dripped one byte at a time: the per-read timeout never
    # trips (each chunk resets it), so only the whole-read deadline can stop it.
    body = { "issuer" => issuer, "token_endpoint" => "#{issuer}/t", "pad" => "x" * 200 }.to_json

    meter = { delivered: 0 }
    connection = Faraday.new do |conn|
      conn.adapter SlowDripAdapter, body: body, chunk_size: 1, pause: 0.02, meter: meter
    end

    # 1 MiB cap (never reached) + a 0.1s wall-clock deadline; ~5 chunks (0.1s) in.
    discovery = Basecamp::Oauth::Discovery.new(http_client: connection, timeout: 0.1)

    error = assert_raises(Basecamp::Oauth::OauthError) do
      discovery.discover(issuer)
    end
    # A slow-drip timeout is a retryable transport failure, not an api_error.
    assert_equal "network", error.type
    assert error.retryable, "wall-clock timeout must be retryable"

    # The read aborted mid-stream: fewer bytes were delivered than the full body.
    assert_operator meter[:delivered], :<, body.bytesize
  end

  # A middleware whose name matches the redirect detector. Standing in for
  # faraday-follow_redirects without taking the dependency.
  class RedirectFollowingMiddleware < Faraday::Middleware
    def call(env)
      @app.call(env)
    end
  end

  def test_injected_client_carrying_redirect_middleware_is_rejected
    connection = Faraday.new do |conn|
      conn.use RedirectFollowingMiddleware
      conn.adapter Faraday.default_adapter
    end

    # The hardening lives on the CONNECTION, so an injected redirect-following
    # client must be refused at construction — not silently trusted.
    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Discovery.new(http_client: connection)
    end
    assert_equal "validation", error.type

    resource_error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Resource.new(http_client: connection)
    end
    assert_equal "validation", resource_error.type
  end

  # A follower whose class name does NOT contain "redirect" — the old class-name
  # heuristic would have missed it.
  class SneakyLocationFollower < Faraday::Middleware
    def call(env)
      @app.call(env)
    end
  end

  def test_injected_client_with_non_redirect_named_middleware_is_rejected
    # The enforceable policy (adapter-only) refuses ANY middleware, so a redirect
    # follower under an innocuous name can no longer bypass the check.
    connection = Faraday.new do |conn|
      conn.use SneakyLocationFollower
      conn.adapter Faraday.default_adapter
    end

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Discovery.new(http_client: connection)
    end
    assert_equal "validation", error.type
  end

  def test_injected_client_with_follower_in_adapter_slot_is_rejected
    # Faraday keeps the terminal adapter handler OUTSIDE +builder.handlers+, so a
    # non-adapter follower smuggled into the adapter slot (+conn.adapter Follower+)
    # would evade a handlers-only scan yet run as the terminal app. The policy
    # check must fold the adapter in and refuse it.
    connection = Faraday.new do |conn|
      conn.adapter SneakyLocationFollower
    end

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::Discovery.new(http_client: connection)
    end
    assert_equal "validation", error.type
  end

  def test_discovery_normalizes_non_integer_body_cap_to_default
    # A nil/float/Float::INFINITY cap would disable the streaming memory bound
    # (an infinite/undefined cap never trips), reintroducing an SSRF/OOM risk.
    # Discovery must normalize it to the finite default, as Resource does.
    default = Basecamp::Oauth::Fetcher::DEFAULT_MAX_BODY_BYTES
    [ Float::INFINITY, nil, 1.5, -1, "big" ].each do |bad|
      discovery = Basecamp::Oauth::Discovery.new(max_body_bytes: bad)
      assert_equal default, discovery.instance_variable_get(:@max_body_bytes),
        "expected #{bad.inspect} to normalize to the default cap"
    end
  end

  def test_discovery_normalizes_non_finite_timeout_to_default
    # A nil/non-numeric/non-positive/Float::INFINITY timeout would disable both the
    # socket timeout and the wall-clock deadline (now + inf never trips), letting a
    # slow-drip peer hang the fetch. Both Discovery and Resource must normalize it.
    default = Basecamp::Oauth::Fetcher::DEFAULT_TIMEOUT
    [ Float::INFINITY, Float::NAN, nil, 0, -1, "10" ].each do |bad|
      [ Basecamp::Oauth::Discovery, Basecamp::Oauth::Resource ].each do |klass|
        instance = klass.new(timeout: bad)
        assert_equal default, instance.instance_variable_get(:@timeout),
          "expected #{klass}#new(timeout: #{bad.inspect}) to normalize to the default"
      end
    end
    # A valid positive timeout is preserved.
    assert_equal 2.5, Basecamp::Oauth::Discovery.new(timeout: 2.5).instance_variable_get(:@timeout)
  end

  def test_redirect_is_not_followed
    issuer = "https://issuer.redirect-test.example"
    attacker = "https://attacker.example.com"

    stub_request(:get, "#{issuer}/.well-known/oauth-authorization-server")
      .to_return(status: 302, headers: { "Location" => "#{attacker}/.well-known/oauth-authorization-server" })
    attacker_stub = stub_request(:get, "#{attacker}/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: { issuer: attacker, token_endpoint: "#{attacker}/t" }.to_json,
        headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover(issuer)
    end
    # The 3xx is surfaced as a non-2xx api_error rather than chased.
    assert_equal "api_error", error.type
    assert_equal 302, error.http_status
    assert_not_requested(attacker_stub)
  end
end
