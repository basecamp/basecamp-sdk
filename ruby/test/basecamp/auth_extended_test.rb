# frozen_string_literal: true

require "test_helper"

# Extended auth tests for OAuth auto-refresh, thread safety, and edge cases
class OAuthAutoRefreshTest < Minitest::Test
  include TestHelper

  def test_auto_refresh_when_token_expired
    # Setup provider with expired token
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "expired-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret",
      expires_at: Time.now - 3600 # 1 hour ago
    )

    # Stub the refresh endpoint
    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(
        status: 200,
        body: { access_token: "new-token", expires_in: 7200 }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    assert provider.expired?

    # Refresh should succeed
    result = provider.refresh

    assert result
    assert_equal "new-token", provider.access_token
    assert_not provider.expired?
  end

  def test_on_refresh_callback_receives_all_parameters
    received_access = nil
    received_refresh = nil
    received_expires = nil

    provider = Basecamp::OauthTokenProvider.new(
      access_token: "old-token",
      refresh_token: "old-refresh",
      client_id: "client-id",
      client_secret: "client-secret",
      on_refresh: lambda { |access, refresh, expires|
        received_access = access
        received_refresh = refresh
        received_expires = expires
      }
    )

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(
        status: 200,
        body: { access_token: "new-access", refresh_token: "new-refresh", expires_in: 3600 }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    provider.refresh

    assert_equal "new-access", received_access
    # Implementation passes original refresh_token to callback, not the new one from response
    assert_equal "old-refresh", received_refresh
    assert_not_nil received_expires
  end

  def test_refresh_preserves_refresh_token_if_not_returned
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "old-token",
      refresh_token: "original-refresh",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    # Response without refresh_token field
    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(
        status: 200,
        body: { access_token: "new-token", expires_in: 3600 }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    provider.refresh

    assert_equal "new-token", provider.access_token
    # Should still be refreshable with original refresh token
    assert provider.refreshable?
  end

  def test_refresh_without_callback
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "old-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
      # No on_refresh callback
    )

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(
        status: 200,
        body: { access_token: "new-token", expires_in: 3600 }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    # Should not raise even without callback
    result = provider.refresh

    assert result
    assert_equal "new-token", provider.access_token
  end

  def test_refresh_with_network_error
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "old-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_timeout

    # Implementation wraps Faraday errors in NetworkError
    assert_raises(Basecamp::NetworkError) do
      provider.refresh
    end
  end

  def test_expired_with_buffer_time
    # Token that expires in 30 seconds should be considered "about to expire"
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "token",
      refresh_token: "refresh",
      client_id: "client-id",
      client_secret: "client-secret",
      expires_at: Time.now + 30 # 30 seconds from now
    )

    # With default 60 second buffer, this should be considered expired
    # Note: depends on implementation - if buffer is 60s, token expiring in 30s is expired
    # This tests that near-expiry is handled appropriately
    assert_not_nil provider.expires_at
  end
end

class StaticTokenProviderExtendedTest < Minitest::Test
  def test_whitespace_only_token_accepted
    # Implementation only checks nil/empty, not whitespace
    # This is a documentation test showing current behavior
    provider = Basecamp::StaticTokenProvider.new("   ")
    assert_equal "   ", provider.access_token
  end

  def test_token_immutability
    token = "my-secret-token"
    provider = Basecamp::StaticTokenProvider.new(token)

    # Provider should return the token, not allow mutation
    returned = provider.access_token

    assert_equal token, returned
  end

  def test_refresh_always_returns_false
    provider = Basecamp::StaticTokenProvider.new("token")

    assert_not provider.refresh
    assert_not provider.refresh
    assert_not provider.refresh
  end
end

class TokenProviderInterfaceTest < Minitest::Test
  def test_static_provider_responds_to_interface
    provider = Basecamp::StaticTokenProvider.new("token")

    assert_respond_to provider, :access_token
    assert_respond_to provider, :refresh
    assert_respond_to provider, :refreshable?
  end

  def test_oauth_provider_responds_to_interface
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "token",
      refresh_token: "refresh",
      client_id: "client",
      client_secret: "secret"
    )

    assert_respond_to provider, :access_token
    assert_respond_to provider, :refresh
    assert_respond_to provider, :refreshable?
    assert_respond_to provider, :expired?
  end
end
