# frozen_string_literal: true

require "test_helper"

class OAuthTest < Minitest::Test
  include TestHelper

  def test_discover_launchpad
    discovery_response = {
      "issuer" => "https://launchpad.37signals.com",
      "authorization_endpoint" => "https://launchpad.37signals.com/authorization/new",
      "token_endpoint" => "https://launchpad.37signals.com/authorization/token"
    }

    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    config = Basecamp::Oauth.discover_launchpad
    assert_equal "https://launchpad.37signals.com", config.issuer
    assert_equal "https://launchpad.37signals.com/authorization/new", config.authorization_endpoint
    assert_equal "https://launchpad.37signals.com/authorization/token", config.token_endpoint
  end

  def test_discover_custom_url
    discovery_response = {
      "issuer" => "https://custom-auth.example.com",
      "authorization_endpoint" => "https://custom-auth.example.com/authorize",
      "token_endpoint" => "https://custom-auth.example.com/token"
    }

    stub_request(:get, "https://custom-auth.example.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    config = Basecamp::Oauth.discover("https://custom-auth.example.com")
    assert_equal "https://custom-auth.example.com", config.issuer
  end

  def test_discover_handles_trailing_slash
    discovery_response = {
      "issuer" => "https://launchpad.37signals.com",
      "authorization_endpoint" => "https://launchpad.37signals.com/authorization/new",
      "token_endpoint" => "https://launchpad.37signals.com/authorization/token"
    }

    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    config = Basecamp::Oauth.discover("https://launchpad.37signals.com/")
    assert_equal "https://launchpad.37signals.com", config.issuer
  end

  def test_discover_raises_on_missing_fields
    discovery_response = {
      "issuer" => "https://launchpad.37signals.com"
      # Missing authorization_endpoint and token_endpoint
    }

    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OAuthError) do
      Basecamp::Oauth.discover_launchpad
    end
    assert_includes error.message, "missing required fields"
  end

  def test_discover_raises_on_network_error
    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 500, body: "Internal Server Error")

    error = assert_raises(Basecamp::Oauth::OAuthError) do
      Basecamp::Oauth.discover_launchpad
    end
    assert_equal "network", error.type
  end

  def test_exchange_code
    token_response = {
      "access_token" => "access_token_123",
      "refresh_token" => "refresh_token_456",
      "expires_in" => 3600,
      "token_type" => "Bearer"
    }

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(status: 200, body: token_response.to_json, headers: { "Content-Type" => "application/json" })

    token = Basecamp::Oauth.exchange_code(
      token_endpoint: "https://launchpad.37signals.com/authorization/token",
      code: "auth_code_123",
      redirect_uri: "https://myapp.com/callback",
      client_id: "my_client_id",
      client_secret: "my_client_secret"
    )

    assert_equal "access_token_123", token.access_token
    assert_equal "refresh_token_456", token.refresh_token
    assert_equal 3600, token.expires_in
  end

  def test_exchange_code_with_legacy_format
    token_response = {
      "access_token" => "access_token_123",
      "refresh_token" => "refresh_token_456",
      "expires_in" => 3600
    }

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(status: 200, body: token_response.to_json, headers: { "Content-Type" => "application/json" })

    token = Basecamp::Oauth.exchange_code(
      token_endpoint: "https://launchpad.37signals.com/authorization/token",
      code: "auth_code_123",
      redirect_uri: "https://myapp.com/callback",
      client_id: "my_client_id",
      client_secret: "my_client_secret",
      use_legacy_format: true
    )

    assert_equal "access_token_123", token.access_token
  end

  def test_refresh_token
    token_response = {
      "access_token" => "new_access_token",
      "refresh_token" => "new_refresh_token",
      "expires_in" => 7200
    }

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(status: 200, body: token_response.to_json, headers: { "Content-Type" => "application/json" })

    token = Basecamp::Oauth.refresh_token(
      token_endpoint: "https://launchpad.37signals.com/authorization/token",
      refresh_token: "old_refresh_token",
      client_id: "my_client_id",
      client_secret: "my_client_secret"
    )

    assert_equal "new_access_token", token.access_token
    assert_equal "new_refresh_token", token.refresh_token
  end

  def test_token_expired
    token = Basecamp::Oauth::Token.new(
      access_token: "test",
      token_type: "Bearer",
      expires_at: Time.now - 100 # Already expired
    )

    assert token.expired?
  end

  def test_token_not_expired
    token = Basecamp::Oauth::Token.new(
      access_token: "test",
      token_type: "Bearer",
      expires_at: Time.now + 3600 # Expires in 1 hour
    )

    refute token.expired?
  end

  def test_config_struct
    config = Basecamp::Oauth::Config.new(
      issuer: "https://example.com",
      authorization_endpoint: "https://example.com/auth",
      token_endpoint: "https://example.com/token",
      registration_endpoint: "https://example.com/register",
      scopes_supported: %w[read write]
    )

    assert_equal "https://example.com", config.issuer
    assert_equal "https://example.com/auth", config.authorization_endpoint
    assert_equal "https://example.com/token", config.token_endpoint
    assert_equal %w[read write], config.scopes_supported
  end
end
