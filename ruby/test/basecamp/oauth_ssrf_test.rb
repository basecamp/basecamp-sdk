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
