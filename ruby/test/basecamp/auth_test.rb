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

    refute provider.refreshable?
    refute provider.refresh
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

    refute provider.refreshable?
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

    refute provider.expired?
  end

  def test_not_expired_without_expiration
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "access-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    refute provider.expired?
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
end
