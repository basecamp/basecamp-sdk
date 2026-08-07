# frozen_string_literal: true

require "test_helper"

# RFC 8628 device authorization grant tests (SPEC.md §16).
#
# Timing is deterministic: an injected sleeper records the requested waits and
# returns immediately, and an injected monotonic clock advances on demand. No
# test performs a real delay.
class OAuthDeviceTest < Minitest::Test
  include TestHelper

  ORIGIN = "https://issuer.device-test.example"
  DEVICE_ENDPOINT = "#{ORIGIN}/oauth/device".freeze
  TOKEN_ENDPOINT = "#{ORIGIN}/oauth/token".freeze
  DEVICE_GRANT = Basecamp::Oauth::DeviceFlow::DEVICE_CODE_GRANT_TYPE

  # A Faraday-shaped double that returns a scripted sequence of outcomes. Each
  # step is either a StandardError (raised) or a Hash (a status/body response).
  class SequencedHttpClient
    Response = Struct.new(:status, :body, :headers)

    # Exposed so cancellation tests can flip a probe the moment a request
    # has been attempted (index goes positive before a step raises).
    attr_reader :index

    def initialize(steps)
      @steps = steps
      @index = 0
    end

    def post(_url)
      step = @steps[@index]
      @index += 1
      raise step if step.is_a?(StandardError)

      Response.new(step[:status], step[:body], step[:headers] || {})
    end
  end

  def device_auth_response(overrides = {})
    {
      "device_code" => "dev-code-123",
      "user_code" => "WDJB-MJHT",
      "verification_uri" => "#{ORIGIN}/device",
      "verification_uri_complete" => "#{ORIGIN}/device?user_code=WDJB-MJHT",
      "expires_in" => 900,
      "interval" => 5
    }.merge(overrides)
  end

  def token_response
    {
      "access_token" => "device_access_token",
      "refresh_token" => "device_refresh_token",
      "token_type" => "Bearer",
      "expires_in" => 3600
    }
  end

  def recording_sleeper
    waits = []
    [ waits, ->(seconds) { waits << seconds } ]
  end

  # A monotonic clock that returns a scripted sequence, holding the final value.
  def scripted_clock(values)
    i = -1
    lambda do
      i += 1
      values[[ i, values.length - 1 ].min]
    end
  end

  def json(body, status: 200)
    { status: status, body: body.to_json, headers: { "Content-Type" => "application/json" } }
  end

  # Polls the already-stubbed TOKEN_ENDPOINT once and asserts the token response
  # was rejected as api_error. Returns the raised error for further assertions.
  def assert_poll_api_error
    _waits, sleeper = recording_sleeper
    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )
    end
    assert_equal "api_error", error.type
    error
  end

  # --- request_device_authorization -----------------------------------------

  def test_request_maps_an_oversized_default_transport_body_to_api_error
    # The nil-client path early-returns Fetcher.stream_http, whose raw
    # BodyTooLarge marker is still mapped by post_form's def-level rescues —
    # a raise inside `return expr` does not bypass a method-level rescue.
    stub_request(:post, DEVICE_ENDPOINT).to_return(
      status: 200, headers: { "Content-Type" => "application/json" },
      body: "x" * 2048
    )

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::DeviceFlow.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli",
        max_body_bytes: 1024
      )
    end

    assert_equal "api_error", error.type
    assert_match(/size cap/i, error.message)
  end

  def test_request_omits_scope_when_unset_and_validates_response
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))

    auth = Basecamp::Oauth.request_device_authorization(
      device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
    )

    assert_equal "dev-code-123", auth.device_code
    assert_equal "WDJB-MJHT", auth.user_code
    assert_equal 5, auth.interval
    assert_requested(:post, DEVICE_ENDPOINT) do |req|
      params = URI.decode_www_form(req.body).to_h
      assert_equal "basecamp-cli", params["client_id"]
      assert_not params.key?("scope") # omitted → server default (read)
    end
  end

  def test_request_sends_scope_when_set
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))

    Basecamp::Oauth.request_device_authorization(
      device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli", scope: "read write"
    )

    assert_requested(:post, DEVICE_ENDPOINT) do |req|
      assert_equal "read write", URI.decode_www_form(req.body).to_h["scope"]
    end
  end

  def test_request_omits_blank_scope
    # Ruby treats "" as truthy, so a blank scope must still be omitted — otherwise
    # the server can't apply its default (read).
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))

    Basecamp::Oauth.request_device_authorization(
      device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli", scope: ""
    )

    assert_requested(:post, DEVICE_ENDPOINT) do |req|
      assert_not URI.decode_www_form(req.body).to_h.key?("scope")
    end
  end

  def test_request_rejects_fractional_expires_in
    # RFC 8628 durations are integer seconds; a fractional value is malformed.
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("expires_in" => 0.5)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_fractional_interval
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("interval" => 2.5)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_oversized_expires_in
    # 1e100 is integer-valued, so whole-second checking alone would admit it;
    # the shared cross-SDK ceiling (2147483 s) makes it api_error.
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("expires_in" => 1e100)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_oversized_interval
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("interval" => 1e100)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_just_past_max_duration
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("expires_in" => 2_147_484)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_accepts_max_duration
    # The 2147483 s ceiling itself is valid — the bound is inclusive.
    stub_request(:post, DEVICE_ENDPOINT)
      .to_return(json(device_auth_response("expires_in" => 2_147_483, "interval" => 2_147_483)))

    auth = Basecamp::Oauth.request_device_authorization(
      device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
    )
    assert_equal 2_147_483, auth.expires_in
    assert_equal 2_147_483, auth.interval
  end

  def test_request_accepts_integer_valued_float_expires_in
    # 900.0 carries no fractional part, so it is a valid integer number of seconds.
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("expires_in" => 900.0)))

    auth = Basecamp::Oauth.request_device_authorization(
      device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
    )
    assert_equal 900, auth.expires_in.to_i
  end

  def test_request_defaults_interval_to_5_when_omitted
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("interval" => nil)))

    auth = Basecamp::Oauth.request_device_authorization(
      device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
    )

    assert_equal 5, auth.interval
  end

  def test_request_rejects_non_positive_expires_in
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("expires_in" => 0)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_non_positive_interval
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("interval" => 0)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_missing_field
    body = { "user_code" => "X", "verification_uri" => ORIGIN, "expires_in" => 900 }
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(body))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_wrong_typed_device_code
    # A numeric device_code must be rejected: the old `.to_s.empty?` probe would
    # have coerced 123456 to "123456" and accepted it as a valid code.
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response("device_code" => 123_456)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_rejects_wrong_typed_verification_uri_complete
    stub_request(:post, DEVICE_ENDPOINT)
      .to_return(json(device_auth_response("verification_uri_complete" => 42)))

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: "basecamp-cli"
      )
    end
    assert_equal "api_error", error.type
  end

  def test_request_requires_client_id
    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.request_device_authorization(
        device_authorization_endpoint: DEVICE_ENDPOINT, client_id: ""
      )
    end
    assert_equal "validation", error.type
  end

  # --- poll_device_token -----------------------------------------------------

  def test_poll_pending_then_slow_down_then_token_sustains_plus_5s
    stub_request(:post, TOKEN_ENDPOINT).to_return(
      json({ "error" => "authorization_pending" }, status: 400),
      json({ "error" => "slow_down" }, status: 400),
      json({ "error" => "authorization_pending" }, status: 400),
      json(token_response)
    )
    waits, sleeper = recording_sleeper

    token = Basecamp::Oauth.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
    )

    assert_equal "device_access_token", token.access_token
    # 5s (pending), 5s (before slow_down), then +5 sustained → 10s, 10s.
    assert_equal [ 5, 5, 10, 10 ], waits
  end

  def test_poll_doubles_interval_after_connection_timeout_then_recovers
    client = SequencedHttpClient.new([
      Faraday::TimeoutError.new("timed out"),
      { status: 200, body: token_response.to_json }
    ])
    waits, sleeper = recording_sleeper

    token = Basecamp::Oauth::DeviceFlow.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900,
      sleeper: sleeper, http_client: client
    )

    assert_equal "device_access_token", token.access_token
    # First wait 5s; timeout doubles the backoff to 10s for the next wait.
    assert_equal 5, waits[0]
    assert_equal 10, waits[1]
  end

  def test_poll_backoff_resets_to_server_interval_after_completed_round_trip
    # Contract: the server-driven interval and the transient timeout backoff are
    # SEPARATE schedules. Two timeouts double the backoff (10s, 20s) without
    # touching the 5s server interval; the first completed round-trip (a plain
    # authorization_pending) resets the backoff, so waits return to 5s.
    client = SequencedHttpClient.new([
      Faraday::TimeoutError.new("timed out"),
      Faraday::TimeoutError.new("timed out"),
      { status: 400, body: { "error" => "authorization_pending" }.to_json },
      { status: 400, body: { "error" => "authorization_pending" }.to_json },
      { status: 200, body: token_response.to_json }
    ])
    waits, sleeper = recording_sleeper

    token = Basecamp::Oauth::DeviceFlow.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900,
      sleeper: sleeper, http_client: client
    )

    assert_equal "device_access_token", token.access_token
    assert_equal [ 5, 10, 20, 5, 5 ], waits
  end

  def test_poll_rejects_non_numeric_expires_in_on_token_response
    # A 2xx with a valid access_token but expires_in "3600" (string) is
    # malformed: it must surface as api_error, not escape as a TypeError from
    # Token's expiry arithmetic.
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("expires_in" => "3600")))
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )
    end
    assert_equal "api_error", error.type
    assert_equal 200, error.http_status
  end

  def test_poll_non_200_success_is_terminal
    # RFC 8628/6749 token responses are exactly 200 (SPEC.md §16): a
    # nonstandard 201/202 carrying an access_token must not complete polling.
    [ 201, 202 ].each do |status|
      stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response, status: status))
      _waits, sleeper = recording_sleeper

      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
        )
      end
      assert_equal "api_error", error.type
      assert_equal status, error.http_status
    end
  end

  def test_poll_protocol_errors_only_on_4xx
    # OAuth protocol states are recognized only on a 4xx: a nonstandard 2xx or
    # a 5xx carrying a crafted authorization_pending body must terminate as
    # api_error, never extend polling.
    [ 201, 202, 500 ].each do |status|
      stub_request(:post, TOKEN_ENDPOINT)
        .to_return(json({ "error" => "authorization_pending" }, status: status))
      _waits, sleeper = recording_sleeper

      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
        )
      end
      assert_equal "api_error", error.type
      assert_equal status, error.http_status
      assert_requested :post, TOKEN_ENDPOINT, times: 1
      WebMock.reset!
    end
  end

  def test_poll_terminal_status_classified_without_draining_body
    # An oversized body on a terminal non-4xx would trip the size cap if
    # drained — skip_status refuses the body and the status api_error surfaces.
    [ 201, 500 ].each do |status|
      stub_request(:post, TOKEN_ENDPOINT)
        .to_return(status: status, body: "x" * (2 * 1024 * 1024))
      _waits, sleeper = recording_sleeper

      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
        )
      end
      assert_equal "api_error", error.type
      assert_match(/status #{status}/, error.message)
      WebMock.reset!
    end
  end

  def test_poll_cancel_set_during_the_request_beats_a_200
    # The sync POST cannot observe the cancelled probe in flight; a cancel set
    # while the request runs must surface after the round-trip — never a token
    # returned post-cancellation. The stub itself flips the probe as it serves
    # the 200, so every check BEFORE the POST sees not-cancelled and only the
    # post-request re-check can catch it.
    cancelled = { flag: false }
    stub_request(:post, TOKEN_ENDPOINT).to_return do |_request|
      cancelled[:flag] = true # cancel arrives while the POST is in flight
      { status: 200, body: token_response.to_json, headers: { "Content-Type" => "application/json" } }
    end
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        sleeper: sleeper, cancelled: -> { cancelled[:flag] }
      )
    end
    assert_equal :cancelled, error.reason
  end

  def test_poll_captures_resource_and_treats_null_as_absent
    [ [ "urn:bc:account:42", "urn:bc:account:42" ], [ nil, nil ] ].each do |sent, expected|
      stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("resource" => sent)))
      _waits, sleeper = recording_sleeper

      token = Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )

      expected.nil? ? assert_nil(token.resource) : assert_equal(expected, token.resource)
    end
  end

  def test_poll_rejects_malformed_resource_on_token_response
    [ "", 7 ].each do |resource|
      stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("resource" => resource)))
      _waits, sleeper = recording_sleeper

      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
        )
      end
      assert_equal "api_error", error.type
      assert_match(/resource/, error.message)
    end
  end

  def test_poll_accepts_token_response_without_expires_in
    # expires_in is optional (RFC 6749 §5.1): absent means no known expiry, so
    # the Token carries nil expires_in/expires_at rather than raising.
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.tap { |body| body.delete("expires_in") }))
    _waits, sleeper = recording_sleeper

    token = Basecamp::Oauth.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
    )

    assert_equal "device_access_token", token.access_token
    assert_nil token.expires_in
    assert_nil token.expires_at
  end

  def test_poll_rejects_infinite_expires_in_on_token_response
    # A server sending expires_in: 1e400 parses to Float::INFINITY — numeric and
    # positive, but Token's expiry arithmetic would raise a raw FloatDomainError.
    # It must surface as api_error. Sent raw since to_json rejects Infinity.
    raw = token_response.reject { |k, _| k == "expires_in" }.to_json.sub(/\}\z/, ', "expires_in": 1e400}')
    stub_request(:post, TOKEN_ENDPOINT)
      .to_return(status: 200, body: raw, headers: { "Content-Type" => "application/json" })

    assert_equal 200, assert_poll_api_error.http_status
  end

  def test_poll_rejects_oversized_expires_in_on_token_response
    # One past the 2_147_483_647 s ceiling is a malformed lifetime, not schedulable.
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("expires_in" => 2_147_483_648)))

    assert_poll_api_error
  end

  def test_poll_accepts_max_token_lifetime
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("expires_in" => 2_147_483_647)))
    _waits, sleeper = recording_sleeper

    token = Basecamp::Oauth.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
    )

    assert_equal 2_147_483_647, token.expires_in
  end

  def test_poll_rejects_non_positive_expires_in_on_token_response
    [ 0, -1 ].each do |value|
      stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("expires_in" => value)))
      assert_poll_api_error
    end
  end

  def test_poll_rejects_fractional_expires_in_on_token_response
    # A fractional token lifetime is malformed under the whole-second contract
    # → api_error, uniform across SDKs (each validates the decoded value).
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("expires_in" => 1.5)))

    assert_poll_api_error
  end

  def test_poll_accepts_integer_valued_float_expires_in_on_token_response
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("expires_in" => 3600.0)))
    _waits, sleeper = recording_sleeper

    token = Basecamp::Oauth.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
    )

    assert_equal 3600, token.expires_in
    assert_kind_of Integer, token.expires_in
  end

  def test_poll_rejects_explicit_empty_token_type
    # An explicit "" token_type is malformed token metadata → api_error,
    # uniform across all five SDKs.
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("token_type" => "")))

    assert_poll_api_error
  end

  def test_poll_defaults_null_token_type_to_bearer
    # JSON null is treated as absent (the Go/Kotlin decoders cannot distinguish
    # them) → Bearer default, uniform across all five SDKs.
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge("token_type" => nil)))
    _waits, sleeper = recording_sleeper

    token = Basecamp::Oauth.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
    )

    assert_equal "Bearer", token.token_type
  end

  def test_poll_rejects_non_string_token_fields
    # A numeric refresh_token/token_type/scope is malformed, not a credential
    # field — surface api_error rather than store a non-String.
    %w[refresh_token token_type scope].each do |field|
      stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response.merge(field => 123)))
      assert_poll_api_error
    end
  end

  def test_poll_treats_redirect_as_api_error_even_with_pending_body
    # A 3xx is never a valid token-endpoint outcome. Redirects are not followed,
    # and a redirect whose body smuggles {"error":"authorization_pending"} must
    # end the flow as api_error — not keep the loop polling forever.
    stub_request(:post, TOKEN_ENDPOINT)
      .to_return(json({ "error" => "authorization_pending" }, status: 302))
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )
    end
    assert_equal "api_error", error.type
    assert_equal 302, error.http_status
    assert_requested(:post, TOKEN_ENDPOINT, times: 1)
  end

  def test_poll_invalid_body_cap_cannot_disable_the_bound
    # An invalid runtime cap (Infinity here; nil/negative are the same class) must
    # normalize to the default rather than disable the streaming/buffered bound —
    # a 2 MiB body still aborts as the size-cap api_error, matching discovery.
    stub_request(:post, TOKEN_ENDPOINT).to_return(json({ "pad" => "x" * (2 * 1024 * 1024) }))
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::DeviceFlow.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        sleeper: sleeper, max_body_bytes: Float::INFINITY
      )
    end

    assert_equal "api_error", error.type
    assert_match(/size cap/i, error.message)
  end

  def test_poll_parse_fault_suppresses_the_parser_cause
    # A malformed 200 token body can carry the access token, and
    # JSON::ParserError embeds the offending input in its message — the
    # mapped fault must not chain it (full_message and cause-aware loggers
    # would disclose the body).
    secret = "sk-live-SUPERSECRET"
    stub_request(:post, TOKEN_ENDPOINT).to_return(
      status: 200, headers: { "Content-Type" => "application/json" },
      body: "{\"access_token\": \"#{secret}' oops"
    )
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )
    end

    assert_equal "api_error", error.type
    assert_not_includes error.message, secret
    assert_nil error.cause
  end

  def test_poll_rejects_out_of_range_caller_durations
    # Caller-input sanity on the public entry point: nil/non-numeric would raise
    # NoMethodError/TypeError deeper in, a non-finite expires_in builds a deadline
    # that NEVER passes (an unbounded poll loop), and zero/negative/oversized
    # values are not schedulable waits. All surface as usage before any request.
    [
      { interval: 5, expires_in: nil },
      { interval: 5, expires_in: 0 },
      { interval: 5, expires_in: -1 },
      { interval: 5, expires_in: Float::INFINITY },
      { interval: 5, expires_in: Float::NAN },
      { interval: 5, expires_in: 2_147_484 },
      { interval: nil, expires_in: 900 },
      { interval: 0, expires_in: 900 },
      { interval: 2_147_484, expires_in: 900 },
      # Whole seconds (RFC 8628): a fractional interval would permit ~1000
      # polls per second. expires_in stays legitimately fractional.
      { interval: 0.001, expires_in: 900 },
      { interval: 2.5, expires_in: 900 }
    ].each do |args|
      error = assert_raises(Basecamp::Oauth::OauthError, args.inspect) do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", **args
        )
      end
      assert_equal "usage", error.type, args.inspect
    end
    assert_not_requested(:post, TOKEN_ENDPOINT)
  end

  def test_poll_expires_against_injected_monotonic_clock
    times = [ 0, 1_000_000 ]
    i = 0
    clock = lambda do
      t = times[[ i, times.length - 1 ].min]
      i += 1
      t
    end
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        sleeper: sleeper, clock: clock
      )
    end

    assert_equal :expired, error.reason
    assert_equal "auth", error.type
  end

  def test_poll_raises_access_denied
    stub_request(:post, TOKEN_ENDPOINT).to_return(json({ "error" => "access_denied" }, status: 400))
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )
    end

    assert_equal :access_denied, error.reason
    assert_equal "auth", error.type
  end

  def test_poll_raises_expired_on_expired_token_error
    stub_request(:post, TOKEN_ENDPOINT).to_return(json({ "error" => "expired_token" }, status: 400))
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )
    end

    assert_equal :expired, error.reason
    assert_equal "auth", error.type
  end

  def test_poll_rejects_wrong_typed_access_token
    # A 2xx body whose access_token is not a non-empty String is malformed: the
    # old `.to_s.empty?` probe would have accepted a numeric token.
    stub_request(:post, TOKEN_ENDPOINT).to_return(json({ "access_token" => 999, "token_type" => "Bearer" }))
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )
    end
    assert_equal "api_error", error.type
  end

  def test_poll_clamps_wait_to_deadline_so_backoff_never_overshoots
    # interval (100s) far exceeds the 10s remaining before expiry. The wait must
    # be clamped to the deadline, not the raw interval, so a long interval or a
    # timeout backoff can never blow past the code lifetime.
    clock = scripted_clock([ 0, 0, 10 ])
    waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 100, expires_in: 10,
        clock: clock, sleeper: sleeper
      )
    end

    assert_equal :expired, error.reason
    assert_equal [ 10 ], waits # clamped to remaining, not the 100s interval
    assert_not_requested(:post, TOKEN_ENDPOINT)
  end

  def test_poll_rejects_malformed_clock_samples_as_usage
    # A String or huge-int sample must surface as the typed usage fault —
    # never a raw TypeError out of the deadline arithmetic, and never an
    # Infinity that defeats every deadline comparison.
    [ "now", 10**400, Complex(1, 1) ].each do |bad|
      _waits, sleeper = recording_sleeper
      error = assert_raises(Basecamp::Oauth::OauthError, "clock sample #{bad.inspect}") do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", interval: 5, expires_in: 900,
          clock: -> { bad }, sleeper: sleeper
        )
      end
      assert_equal "usage", error.type, "clock sample #{bad.inspect}"
      assert_match(/clock must return a finite number of seconds/, error.message)
    end
  end

  def test_poll_rejects_a_malformed_later_clock_sample_as_usage
    # EVERY sample is validated, not just the first: a clock that goes bad
    # mid-flight (sample 2 here, before the first request) must surface the
    # same typed usage fault instead of a raw TypeError.
    clock = scripted_clock([ 0, "now" ])
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        clock: clock, sleeper: sleeper
      )
    end

    assert_equal "usage", error.type
    assert_not_requested(:post, TOKEN_ENDPOINT)
  end

  def test_poll_header_drip_on_injected_client_is_bounded
    # An injected Faraday client's per-read timeout resets on every socket
    # read: a peer dripping HEADER bytes under it would hold the POST open
    # indefinitely (on_data never runs during the header phase). The outer
    # wall clock must cut it at ~timeout and surface the transport-shaped
    # timeout the poll loop's backoff understands.
    server = TCPServer.new("127.0.0.1", 0)
    port = server.addr[1]
    thread = Thread.new do
      conn = server.accept
      # Drip one header byte per 0.1s for ~5s — far past the 0.5s timeout,
      # each gap well under the per-read timeout.
      "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n".each_char do |ch|
        conn.write(ch)
        sleep(0.1)
      end
    rescue IOError, SystemCallError
      nil
    end

    WebMock.disable!
    # Adapter-only: the SSRF guard refuses middleware it cannot verify
    # redirect-free (Faraday.new's default stack includes UrlEncoded).
    connection = Faraday.new { |f| f.adapter :net_http }
    started = Process.clock_gettime(Process::CLOCK_MONOTONIC)
    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth::DeviceFlow.request_device_authorization(
        device_authorization_endpoint: "http://127.0.0.1:#{port}/device",
        client_id: "basecamp-cli", http_client: connection, timeout: 0.5
      )
    end
    took = Process.clock_gettime(Process::CLOCK_MONOTONIC) - started

    assert_equal :transport, error.reason
    assert_operator took, :<, 2.0, "header drip must be cut by the wall clock, took #{took.round(2)}s"
  ensure
    WebMock.enable!
    thread&.kill&.join
    server&.close
  end

  def test_injected_client_dispatch_refused_when_budget_already_spent
    # The wall-clock wrap must grant the REMAINING budget, not a fresh full
    # timeout: when the deadline has already passed by dispatch time (thread
    # descheduled after the deadline was captured), the POST must be refused
    # rather than handed a whole new request window.
    fetcher = Basecamp::Oauth::Fetcher
    original = fetcher.method(:bounded_reader)
    fetcher.define_singleton_method(:bounded_reader) do |*args, **kwargs|
      sleep 0.1 # burn the entire budget between deadline capture and dispatch
      original.call(*args, **kwargs)
    end
    dispatched = false
    client = Object.new
    client.define_singleton_method(:post) { |_url| dispatched = true }

    assert_raises(Faraday::TimeoutError) do
      Basecamp::Oauth::DeviceFlow.send(
        :post_form, client, TOKEN_ENDPOINT, { "grant_type" => "test" },
        timeout: 0.05, max_body_bytes: 1024
      )
    end
    assert_equal false, dispatched, "POST must not dispatch on an exhausted budget"
  ensure
    fetcher.define_singleton_method(:bounded_reader, original)
  end

  def test_injected_client_response_completing_past_the_deadline_is_refused
    # Timeout.timeout's interrupt can be delivered late: simulate that race by
    # neutering the wrap so the request returns a 200 after the deadline has
    # passed. The post-return monotonic re-check must refuse the late response
    # as the same transport-shaped timeout — never hand back a token past the
    # wall clock.
    original = Timeout.method(:timeout)
    Timeout.define_singleton_method(:timeout) { |*_args, &block| block.call }
    client = Object.new
    client.define_singleton_method(:post) do |_url|
      sleep 0.1 # completes past the 0.05s deadline; the interrupt never lands
      SequencedHttpClient::Response.new(200, +%({"access_token":"late"}))
    end

    assert_raises(Faraday::TimeoutError) do
      Basecamp::Oauth::DeviceFlow.send(
        :post_form, client, TOKEN_ENDPOINT, { "grant_type" => "test" },
        timeout: 0.05, max_body_bytes: 1024
      )
    end
  ensure
    Timeout.define_singleton_method(:timeout, original)
  end

  def test_poll_cancellation_wins_over_a_transport_fault
    # Cancellation-beats-classification on the error path: the probe flips as
    # the doomed request fails (index goes positive before the step raises),
    # so the poll must surface cancelled — never the transport fault.
    client = SequencedHttpClient.new([ Faraday::ConnectionFailed.new("boom") ])
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth::DeviceFlow.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        http_client: client, sleeper: sleeper,
        cancelled: -> { client.index.positive? }
      )
    end

    assert_equal :cancelled, error.reason
  end

  def test_poll_raises_transport_on_non_timeout_failure
    client = SequencedHttpClient.new([ Faraday::ConnectionFailed.new("boom") ])
    _waits, sleeper = recording_sleeper

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth::DeviceFlow.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        sleeper: sleeper, http_client: client
      )
    end

    assert_equal :transport, error.reason
    assert_equal "network", error.type
    assert error.retryable
  end

  def test_poll_raises_cancelled_when_cancellation_probe_trips
    cancel_flag = false
    sleeper = ->(_seconds) { cancel_flag = true }
    cancelled = -> { cancel_flag }

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        sleeper: sleeper, cancelled: cancelled
      )
    end

    assert_equal :cancelled, error.reason
    assert_equal "usage", error.type
  end

  # Streams an oversized 302 body to on_data WITH env (as the live net/http adapter
  # does), so the status-skip path is exercised without a real socket (which WebMock
  # blocks in the suite). If the body were drained it would trip the size cap.
  class RedirectStreamAdapter < Faraday::Adapter
    def call(env)
      on_data = env.request.on_data
      if on_data
        env.status = 302
        on_data.call("x" * (2 * 1024 * 1024), 2 * 1024 * 1024, env)
      end
      save_response(env, 302, "", { "Location" => "https://x/" })
      @app.call(env)
    end
  end

  def test_token_poll_redirect_classified_by_status_without_draining_body
    # A 3xx must fail by STATUS before the body is drained — surfacing a redirect
    # api_error, not the size cap (which draining the oversized body would trip) and
    # not a timeout the poll loop would retry.
    connection = Faraday.new { |conn| conn.adapter RedirectStreamAdapter }

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::DeviceFlow.poll_device_token(
        token_endpoint: "https://issuer.example/token", client_id: "basecamp-cli",
        device_code: "d", interval: 5, expires_in: 900,
        http_client: connection, clock: -> { 0.0 }, sleeper: ->(_seconds) { }
      )
    end

    assert_equal "api_error", error.type
    assert_match(/redirect/i, error.message)
  end

  def test_token_poll_redirect_classified_by_status_on_buffered_adapter
    # Faraday's built-in Test adapter buffers the whole body and IGNORES +on_data+ —
    # exactly like any adapter on Faraday 2.0–2.4, which never passes +env+ to
    # +on_data+. The SkipBody fast-path can never fire, so the post-request status
    # backstop in +post_form+ must classify the redirect by its completed status
    # BEFORE the oversized buffered body trips the size cap.
    stubs = Faraday::Adapter::Test::Stubs.new do |stub|
      stub.post("https://issuer.example/token") do
        [ 302, { "Location" => "https://x/" }, "x" * (2 * 1024 * 1024) ]
      end
    end
    connection = Faraday.new { |conn| conn.adapter :test, stubs }

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth::DeviceFlow.poll_device_token(
        token_endpoint: "https://issuer.example/token", client_id: "basecamp-cli",
        device_code: "d", interval: 5, expires_in: 900,
        http_client: connection, clock: -> { 0.0 }, sleeper: ->(_seconds) { }
      )
    end

    # Redirect api_error by status — NOT the size cap the 2 MiB buffered body would
    # trip if the backstop failed to short-circuit before reading it.
    assert_equal "api_error", error.type
    assert_equal 302, error.http_status
    assert_match(/redirect/i, error.message)
    assert_no_match(/size cap/i, error.message)
  end

  def test_poll_cancellation_during_wait_is_prompt
    # A long interval must not delay cancellation: the wait polls the cancelled
    # probe in small chunks, so a cancel set mid-wait raises without sleeping the
    # whole interval at once (a plain sleep is not interruptible).
    recorded = []
    sleeper = ->(seconds) { recorded << seconds }
    cancelled = -> { recorded.length >= 3 } # cancel after the 3rd chunk

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        sleeper: sleeper, cancelled: cancelled
      )
    end

    assert_equal :cancelled, error.reason
    # Chunked into small waits, never one sleeper.call(5).
    assert recorded.any?
    assert(recorded.all? { |s| s <= Basecamp::Oauth::DeviceFlow::CANCEL_POLL_INTERVAL_SECONDS })
  end

  # --- perform_device_login --------------------------------------------------

  def test_perform_guards_capability_endpoint_present_but_no_device_grant
    polled = stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response))
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ "refresh_token" ] # no device_code grant
    )

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli", display: ->(_auth) { }
      )
    end

    assert_equal :unavailable, error.reason
    assert_equal "validation", error.type
    assert_not_requested(polled)
  end

  def test_perform_guards_capability_grant_present_but_no_endpoint
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT ] # device grant but no endpoint
    )

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli", display: ->(_auth) { }
      )
    end

    assert_equal :unavailable, error.reason
  end

  def test_perform_fires_display_hook_then_completes
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))
    stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response))
    _waits, sleeper = recording_sleeper
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT, "refresh_token" ]
    )

    displayed = nil
    token = Basecamp::Oauth.perform_device_login(
      config: config, client_id: "basecamp-cli",
      display: ->(auth) { displayed = auth }, sleeper: sleeper
    )

    assert_equal "WDJB-MJHT", displayed.user_code
    assert_equal "device_access_token", token.access_token
  end

  def test_perform_cancel_during_the_post_display_clock_call_beats_expiry
    # The post-display sample is the same cancellation-capable callback seam
    # as the pre-request anchor: a cancel flipped inside it — even one
    # returning a beyond-deadline value — must surface cancelled, never
    # expired.
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))
    state = { cancelled: false, calls: 0 }
    clock = lambda do
      state[:calls] += 1
      if state[:calls] >= 2
        state[:cancelled] = true
        10_000
      else
        0
      end
    end
    _waits, sleeper = recording_sleeper
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT, "refresh_token" ]
    )

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli", display: ->(_auth) { },
        sleeper: sleeper, clock: clock, cancelled: -> { state[:cancelled] }
      )
    end

    assert_equal :cancelled, error.reason
  end

  def test_perform_cancel_during_the_anchor_clock_call_never_reaches_display
    # The injected clock is itself a callback seam: a cancel flipped inside
    # the post-response issuance anchor must surface cancelled before the
    # display hook fires.
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))
    state = { cancelled: false }
    clock = lambda do
      state[:cancelled] = true
      0
    end
    _waits, sleeper = recording_sleeper
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT, "refresh_token" ]
    )

    displayed = []
    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli",
        display: ->(auth) { displayed << auth },
        sleeper: sleeper, clock: clock, cancelled: -> { state[:cancelled] }
      )
    end

    assert_equal :cancelled, error.reason
    assert_empty displayed
  end

  def test_perform_expiry_anchors_at_response_receipt_not_request_start
    # SPEC §16: the deadline is clock() + expires_in taken AFTER the
    # authorization request returns — a 6s request leg with expires_in 5 must
    # NOT expire the fresh code client-side; expiry past receipt is
    # arbitrated by the server (expired_token).
    state = { t: 0 }
    auth_body = device_auth_response("expires_in" => 5).to_json
    token_body = token_response.to_json
    client = Object.new
    client.define_singleton_method(:post) do |url|
      if url.include?("/oauth/device")
        state[:t] += 6
        SequencedHttpClient::Response.new(200, auth_body)
      else
        SequencedHttpClient::Response.new(200, token_body)
      end
    end
    _waits, sleeper = recording_sleeper
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT, "refresh_token" ]
    )

    token = Basecamp::Oauth::DeviceFlow.perform_device_login(
      config: config, client_id: "basecamp-cli",
      display: ->(_auth) { }, sleeper: sleeper,
      http_client: client, clock: -> { state[:t] }
    )

    assert_equal "device_access_token", token.access_token
  end

  def test_perform_cancellation_wins_over_a_device_auth_fault
    # Same contract on the error path: the authorization request fails AND the
    # probe flipped mid-flight — cancelled must win over transport, and the
    # display hook must never fire.
    client = SequencedHttpClient.new([ Faraday::ConnectionFailed.new("boom") ])
    _waits, sleeper = recording_sleeper
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT, "refresh_token" ]
    )

    displayed = []
    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth::DeviceFlow.perform_device_login(
        config: config, client_id: "basecamp-cli",
        display: ->(auth) { displayed << auth }, sleeper: sleeper,
        http_client: client, cancelled: -> { client.index.positive? }
      )
    end

    assert_equal :cancelled, error.reason
    assert_empty displayed
  end

  def test_perform_already_cancelled_flow_makes_no_request
    requested = stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))
    _waits, sleeper = recording_sleeper
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT, "refresh_token" ]
    )

    displayed = []
    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli",
        display: ->(auth) { displayed << auth }, sleeper: sleeper,
        cancelled: -> { true }
      )
    end

    assert_equal :cancelled, error.reason
    assert_not_requested(requested)
    assert_empty displayed
  end

  def test_perform_cancel_during_authorization_request_never_reaches_display
    # The stub flips the probe as it serves the code pair, so the entry check
    # passes and only the post-request re-check can catch it — the display
    # hook must never fire for a cancelled flow.
    cancelled = { flag: false }
    stub_request(:post, DEVICE_ENDPOINT).to_return do |_request|
      cancelled[:flag] = true
      { status: 200, body: device_auth_response.to_json, headers: { "Content-Type" => "application/json" } }
    end
    _waits, sleeper = recording_sleeper
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT, "refresh_token" ]
    )

    displayed = []
    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli",
        display: ->(auth) { displayed << auth }, sleeper: sleeper,
        cancelled: -> { cancelled[:flag] }
      )
    end

    assert_equal :cancelled, error.reason
    assert_empty displayed
  end

  def test_perform_raises_expired_when_display_hook_consumes_whole_lifetime
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response)) # expires_in 900
    polled = stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response))
    _waits, sleeper = recording_sleeper
    # issued_at = 0; the clock reads 950s AFTER the display hook returns — past
    # the 900s code lifetime. The deadline is anchored at ISSUANCE, so a slow
    # display cannot reset it: expiry is raised without a single poll.
    clock = scripted_clock([ 0, 950 ])
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT ]
    )

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli",
        display: ->(_auth) { }, clock: clock, sleeper: sleeper
      )
    end

    assert_equal :expired, error.reason
    assert_not_requested(polled)
  end

  def test_perform_rejects_a_non_callable_display_before_any_request
    # display is the only mechanism surfacing the verification code:
    # dereferencing nil AFTER the request would mint a code nobody can
    # approve. Reject as usage with zero network activity.
    requested = stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response))
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT ]
    )

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli", display: nil
      )
    end

    assert_equal "usage", error.type
    assert_not_requested(requested)
  end

  def test_poll_rejects_a_deadline_at_beyond_the_code_lifetime
    # deadline_at can only SHORTEN the validated lifetime (mirrors the TS
    # deadlineAtMs bound).
    [ Float::NAN, 10_000_000.0, Complex(1, 2), "soon" ].each do |bad|
      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", interval: 5, expires_in: 900,
          clock: -> { 0.0 }, deadline_at: bad, sleeper: ->(_s) { }
        )
      end
      assert_equal "usage", error.type
    end
  end

  def test_perform_cancel_during_display_beats_expiry
    # A display hook that both cancels the flow and consumes the whole code
    # lifetime (a prompt closing in response to cancellation) must surface
    # cancelled, not expired.
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response)) # expires_in 900
    _waits, sleeper = recording_sleeper
    clock = scripted_clock([ 0, 950 ])
    state = { cancelled: false }
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT ]
    )

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli",
        display: ->(_auth) { state[:cancelled] = true },
        cancelled: -> { state[:cancelled] },
        clock: clock, sleeper: sleeper
      )
    end

    assert_equal :cancelled, error.reason
  end

  def test_perform_anchors_deadline_at_issuance_so_slow_display_shrinks_remaining
    stub_request(:post, DEVICE_ENDPOINT).to_return(json(device_auth_response)) # expires_in 900, interval 5
    polled = stub_request(:post, TOKEN_ENDPOINT).to_return(json(token_response))
    waits, sleeper = recording_sleeper
    # issued_at = 0; display returns at t=897, leaving only 3s of the 900s budget.
    # poll must see remaining = 3 (deadline anchored at issuance), so its first
    # wait clamps to 3s and it expires — proving the slow display did NOT reset
    # the code lifetime back to the full 900s.
    clock = scripted_clock([ 0, 897, 897, 897, 900 ])
    config = Basecamp::Oauth::Config.new(
      issuer: ORIGIN, token_endpoint: TOKEN_ENDPOINT,
      device_authorization_endpoint: DEVICE_ENDPOINT,
      grant_types_supported: [ DEVICE_GRANT ]
    )

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.perform_device_login(
        config: config, client_id: "basecamp-cli",
        display: ->(_auth) { }, clock: clock, sleeper: sleeper
      )
    end

    assert_equal :expired, error.reason
    assert_equal [ 3 ], waits # clamped to the 3s remaining, not the full 900s
    assert_not_requested(polled)
  end

  # --- poll_device_token 429 handling ---------------------------------------

  def json429(retry_after: nil)
    headers = { "Content-Type" => "application/json" }
    headers["Retry-After"] = retry_after if retry_after
    { status: 429, body: { "error" => "too_many_requests" }.to_json, headers: headers }
  end

  def test_poll_retries_after_429_with_retry_after_override
    stub_request(:post, TOKEN_ENDPOINT).to_return(
      json429(retry_after: "30"),
      json(token_response)
    )
    waits, sleeper = recording_sleeper

    token = Basecamp::Oauth.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
    )

    assert_equal "device_access_token", token.access_token
    # Initial 5s wait, then the one-shot max(interval, Retry-After) = 30s.
    assert_equal [ 5, 30 ], waits
  end

  def test_poll_429_missing_or_malformed_retry_after_falls_back_to_interval
    [ nil, "abc", "1.5", "-1", "0", "99999999999999999999", "\u00a030", "\u200930" ].each do |header|
      stub_request(:post, TOKEN_ENDPOINT).to_return(
        json429(retry_after: header),
        json(token_response)
      )
      waits, sleeper = recording_sleeper

      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
      )

      assert_equal [ 5, 5 ], waits, "header=#{header.inspect}"
    end
  end

  def test_parse_retry_after_trims_only_ascii_ows
    # RFC 9110: delta-seconds is 1*DIGIT and OWS is only SP/HTAB. String#strip
    # also removes \v \f \r \n \0 — which would trim a malformed value into
    # validity — so the parser trims exactly SP/HTAB (SPEC \u00a716). Control
    # characters cannot ride a WebMock header, so the parser is exercised
    # directly, like the NBSP cases in the other SDKs.
    parse = ->(header) { Basecamp::Oauth::DeviceFlow.send(:parse_retry_after_seconds, header) }

    assert_equal 30, parse.call(" 30 ")
    assert_equal 30, parse.call("\t30\t")
    assert_equal 30, parse.call(" \t30\t ")
    assert_equal 0, parse.call("\v30")
    assert_equal 0, parse.call("\f30\f")
    assert_equal 0, parse.call("\r30\n")
    assert_equal 0, parse.call("\u00a030")
    assert_equal 0, parse.call("\u200930")
    # Representable over-ceiling clamps (the wait rule clips to the remaining
    # lifetime); >10 significant digits is unrepresentable -> fallback.
    assert_equal 2_147_483, parse.call("2147484")
    assert_equal 30, parse.call("00000000030")
    assert_equal 0, parse.call("99999999999")
  end

  def test_poll_429_retry_after_override_decays_after_one_wait
    stub_request(:post, TOKEN_ENDPOINT).to_return(
      json429(retry_after: "30"),
      json({ "error" => "authorization_pending" }, status: 400),
      json(token_response)
    )
    waits, sleeper = recording_sleeper

    Basecamp::Oauth.poll_device_token(
      token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
      device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
    )

    # 5s initial, 30s one-shot override, then back to the 5s interval.
    assert_equal [ 5, 30, 5 ], waits
  end

  def test_poll_429_wrong_pair_stays_terminal
    [
      json({ "error" => "rate_limited" }, status: 429),
      json({ "error" => "authorization_pending" }, status: 429),
      json({ "error" => "slow_down" }, status: 429),
      json({ "error" => "too_many_requests" }, status: 400)
    ].each do |response|
      stub_request(:post, TOKEN_ENDPOINT).to_return(response)
      _waits, sleeper = recording_sleeper

      error = assert_raises(Basecamp::Oauth::OauthError) do
        Basecamp::Oauth.poll_device_token(
          token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
          device_code: "dev-code-123", interval: 5, expires_in: 900, sleeper: sleeper
        )
      end
      assert_equal "api_error", error.type
    end
  end

  def test_poll_429_wait_clamped_to_expiry
    stub_request(:post, TOKEN_ENDPOINT).to_return(json429(retry_after: "3600"))
    waits, sleeper = recording_sleeper
    # Scripted monotonic clock: deadline anchors at t=0 with a 20s lifetime.
    # The second iteration's huge Retry-After override must clamp to the 14s
    # remaining, and the post-wait check then expires the flow.
    clock = scripted_clock([ 0, 0, 5, 6, 20 ])

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 20,
        sleeper: sleeper, clock: clock
      )
    end
    assert_equal :expired, error.reason
    assert_equal [ 5, 14 ], waits
  end

  def test_poll_cancellation_during_429_wait
    stub_request(:post, TOKEN_ENDPOINT).to_return(json429(retry_after: "30"))
    slept = { total: 0.0 }
    cancelled = { flag: false }
    # The cancellable wait chunks each interval, so count elapsed time: once
    # past the first 5s wait we are inside the post-429 override wait.
    sleeper = lambda do |seconds|
      slept[:total] += seconds
      cancelled[:flag] = true if slept[:total] > 5.0
    end

    error = assert_raises(Basecamp::Oauth::DeviceFlowError) do
      Basecamp::Oauth.poll_device_token(
        token_endpoint: TOKEN_ENDPOINT, client_id: "basecamp-cli",
        device_code: "dev-code-123", interval: 5, expires_in: 900,
        sleeper: sleeper, cancelled: -> { cancelled[:flag] }
      )
    end
    assert_equal :cancelled, error.reason
  end
end
