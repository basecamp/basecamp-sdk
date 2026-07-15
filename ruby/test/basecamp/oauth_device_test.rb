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
    Response = Struct.new(:status, :body)

    def initialize(steps)
      @steps = steps
      @index = 0
    end

    def post(_url)
      step = @steps[@index]
      @index += 1
      raise step if step.is_a?(StandardError)

      Response.new(step[:status], step[:body])
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

  # --- request_device_authorization -----------------------------------------

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
end
