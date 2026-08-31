# frozen_string_literal: true

require "test_helper"

class StaticTokenProviderTest < Minitest::Test
  def test_returns_token
    provider = Basecamp::StaticTokenProvider.new("my-token")

    assert_equal "my-token", provider.access_token
  end

  def test_raises_for_nil_token
    assert_raises(ArgumentError) do
      Basecamp::StaticTokenProvider.new(nil)
    end
  end

  def test_raises_for_empty_token
    assert_raises(ArgumentError) do
      Basecamp::StaticTokenProvider.new("")
    end
  end

  def test_not_refreshable
    provider = Basecamp::StaticTokenProvider.new("my-token")

    assert_not provider.refreshable?
    assert_not provider.refresh
  end
end

class OauthTokenProviderTest < Minitest::Test
  def test_returns_access_token
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "access-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    assert_equal "access-token", provider.access_token
  end

  def test_refreshable_with_refresh_token
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "access-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    assert provider.refreshable?
  end

  def test_not_refreshable_without_refresh_token
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "access-token",
      refresh_token: nil,
      client_id: "client-id",
      client_secret: "client-secret"
    )

    assert_not provider.refreshable?
  end

  def test_expired_with_past_time
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "access-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret",
      expires_at: Time.now - 3600
    )

    assert provider.expired?
  end

  def test_not_expired_with_future_time
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "access-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret",
      expires_at: Time.now + 3600
    )

    assert_not provider.expired?
  end

  def test_not_expired_without_expiration
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "access-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    assert_not provider.expired?
  end

  def test_refresh_success
    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(
        status: 200,
        body: { access_token: "new-token", expires_in: 3600 }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    callback_called = false
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "old-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret",
      on_refresh: ->(_access, _refresh, _expires) { callback_called = true }
    )

    result = provider.refresh

    assert result
    assert_equal "new-token", provider.access_token
    assert callback_called
  end

  def test_refresh_failure
    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(status: 401, body: "Unauthorized")

    provider = Basecamp::OauthTokenProvider.new(
      access_token: "old-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    assert_raises(Basecamp::AuthError) do
      provider.refresh
    end
  end

  def test_refresh_refuses_every_redirect_status_and_never_follows
    # SPEC §16 "Token-Endpoint Transport Policy": the refresh POST carries the
    # refresh token and client secret, so a redirect surfaces as a typed
    # api fault with the real status — never a re-POST toward Location.
    attacker_stub = stub_request(:post, "https://attacker.example.com/token")
      .to_return(status: 200, body: { access_token: "stolen" }.to_json,
        headers: { "Content-Type" => "application/json" })

    Basecamp::OauthTokenProvider::REDIRECT_STATUSES.each do |status|
      stub_request(:post, "https://launchpad.37signals.com/authorization/token")
        .to_return(status: status, headers: { "Location" => "https://attacker.example.com/token" })

      provider = Basecamp::OauthTokenProvider.new(
        access_token: "old-token",
        refresh_token: "refresh-token",
        client_id: "client-id",
        client_secret: "client-secret"
      )

      error = assert_raises(Basecamp::ApiError, status.to_s) { provider.refresh }
      assert_equal status, error.http_status, status.to_s
      assert_match(/not followed/, error.message, status.to_s)
      assert_equal "old-token", provider.instance_variable_get(:@access_token),
        "a refused redirect must not mutate the stored token"
    end
    assert_not_requested(attacker_stub)
  end

  def test_refresh_runs_on_the_bounded_headers_first_transport
    # perform_refresh must ride Fetcher.stream_http with the shared 30 s
    # budget, the body cap, and the redirect skip set. The transport's own
    # guarantees under that budget — the monotonic whole-request watchdog, the
    # header-time classification, the streaming cap — are proven against live
    # sockets in oauth_transport_test.rb; this pins that the provider actually
    # dispatches through it (TOKEN_URL is a fixed constant, so a live
    # stalled-server deadline test would need a seam the provider does not
    # otherwise want).
    captured = nil

    provider = Basecamp::OauthTokenProvider.new(
      access_token: "old-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    # Swap the module function and restore by re-delegating — the same idiom
    # the transport tests use for IPSocket.getaddress (minitest/mock is not
    # loadable under bundled_gems here).
    original = Basecamp::Oauth::Fetcher.method(:stream_http)
    Basecamp::Oauth::Fetcher.define_singleton_method(:stream_http) do |method, url, **kwargs|
      captured = [ method, url, kwargs ]
      [ 200, { access_token: "new-token", expires_in: 3600 }.to_json ]
    end
    begin
      assert provider.refresh
    ensure
      Basecamp::Oauth::Fetcher.define_singleton_method(:stream_http) do |*args, **kwargs, &blk|
        original.call(*args, **kwargs, &blk)
      end
    end

    method, url, kwargs = captured
    assert_equal :post, method
    assert_equal Basecamp::OauthTokenProvider::TOKEN_URL, url
    assert_equal Basecamp::OauthTokenProvider::REFRESH_TIMEOUT, kwargs[:timeout]
    assert_equal Basecamp::OauthTokenProvider::MAX_RESPONSE_BYTES, kwargs[:max_body_bytes]
    assert Basecamp::OauthTokenProvider::REDIRECT_STATUSES.all? { |s| kwargs[:skip_status].call(s) },
      "every refused redirect status must be in the skip set"
    assert_not kwargs[:skip_status].call(200), "success must not be skipped"
    assert_equal "new-token", provider.instance_variable_get(:@access_token)
  end
end
