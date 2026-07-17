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

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover_launchpad
    end
    assert_includes error.message, "missing required fields"
  end

  def test_discover_raises_api_error_on_non_2xx
    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 500, body: "Internal Server Error")

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover_launchpad
    end
    # Non-2xx is a server-side api_error, not a transport (network) error.
    assert_equal "api_error", error.type
    assert_equal 500, error.http_status
  end

  def test_discover_rejects_wrong_typed_endpoint
    # A non-string endpoint is malformed metadata: reject it rather than trusting
    # a truthy value.
    discovery_response = {
      "issuer" => "https://launchpad.37signals.com",
      "token_endpoint" => "https://launchpad.37signals.com/token",
      "device_authorization_endpoint" => 42
    }
    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover_launchpad
    end
    assert_equal "api_error", error.type
    assert_includes error.message, "device_authorization_endpoint"
  end

  def test_discover_rejects_present_null_endpoint
    # A present JSON null endpoint is malformed metadata, NOT an absent key.
    discovery_response = {
      "issuer" => "https://launchpad.37signals.com",
      "token_endpoint" => "https://launchpad.37signals.com/token",
      "registration_endpoint" => nil
    }
    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover_launchpad
    end
    assert_equal "api_error", error.type
    assert_includes error.message, "registration_endpoint"
  end

  def test_discover_rejects_present_null_grant_types
    discovery_response = {
      "issuer" => "https://launchpad.37signals.com",
      "token_endpoint" => "https://launchpad.37signals.com/token",
      "grant_types_supported" => nil
    }
    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover_launchpad
    end
    assert_equal "api_error", error.type
    assert_includes error.message, "grant_types_supported"
  end

  def test_resource_first_binds_metadata_issuer_to_advertised_code_point
    # The RFC 8414 code-point bind is against the ADVERTISED issuer, so an AS
    # whose issuer matches the advertised trailing-slash form must bind rather
    # than be normalized away into a false issuer_mismatch. Binding is internal
    # (Oauth.discover exposes no override), so drive it through the resource-first
    # orchestrator: the resource advertises the trailing-slash issuer and the AS
    # echoes it.
    advertised = "https://bc5.example/"
    stub_request(:get, "https://api.example.com/.well-known/oauth-protected-resource")
      .to_return(status: 200,
        body: { resource: "https://api.example.com", authorization_servers: [ advertised ] }.to_json,
        headers: { "Content-Type" => "application/json" })
    stub_request(:get, "https://bc5.example/.well-known/oauth-authorization-server")
      .to_return(status: 200,
        body: { issuer: advertised, token_endpoint: "https://bc5.example/token" }.to_json,
        headers: { "Content-Type" => "application/json" })

    result = Basecamp::Oauth.discover_from_resource("https://api.example.com")
    assert result.selected?
    assert_equal advertised, result.config.issuer
  end

  def test_discover_rejects_scopes_supported_not_array_of_strings
    discovery_response = {
      "issuer" => "https://launchpad.37signals.com",
      "token_endpoint" => "https://launchpad.37signals.com/token",
      "scopes_supported" => "read write" # a bare string, not an array
    }
    stub_request(:get, "https://launchpad.37signals.com/.well-known/oauth-authorization-server")
      .to_return(status: 200, body: discovery_response.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover_launchpad
    end
    assert_equal "api_error", error.type
    assert_includes error.message, "scopes_supported"
  end

  def test_discover_protected_resource_rejects_wrong_typed_resource
    # A numeric resource must not be indexed/`.empty?`-probed as if it were a
    # string; it is malformed metadata → api_error.
    stub_request(:get, "https://api.example.com/.well-known/oauth-protected-resource")
      .to_return(status: 200, body: { resource: 12_345 }.to_json,
        headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OauthError) do
      Basecamp::Oauth.discover_protected_resource("https://api.example.com")
    end
    assert_equal "api_error", error.type
  end

  def test_resource_binds_against_raw_caller_default_port
    # ":443" normalizes away for the fetch URL, but the metadata resource is bound
    # code-point-exact against the ORIGINAL caller identifier (RFC 9728 §3.3).
    res = "https://api.example.com"
    stub_request(:get, "#{res}/.well-known/oauth-protected-resource")
      .to_return(status: 200, body: { resource: "#{res}:443" }.to_json,
        headers: { "Content-Type" => "application/json" })

    meta = Basecamp::Oauth.discover_protected_resource("#{res}:443")
    assert_equal "#{res}:443", meta.resource
  end

  def test_present_null_authorization_servers_is_malformed_not_empty
    # A present JSON null authorization_servers is MALFORMED metadata, not
    # "present but empty": it must fail hop-1 (soft resource_discovery_failed),
    # never be normalized to [] and read as no_as_advertised.
    stub_request(:get, "https://api.example.com/.well-known/oauth-protected-resource")
      .to_return(status: 200,
        body: { resource: "https://api.example.com", authorization_servers: nil }.to_json,
        headers: { "Content-Type" => "application/json" })

    result = Basecamp::Oauth.discover_from_resource("https://api.example.com")
    assert result.fallback?
    assert_equal "resource_discovery_failed", result.reason
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

    assert_not token.expired?
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

  # --- Issuer-mismatch classified by CLASS, never by message text -------------

  def test_as_failure_error_classifies_binding_mismatch_by_class
    # The structured marker raised by the RFC 8414 binding check is classified
    # as issuer_mismatch by its CLASS.
    marker = Basecamp::Oauth::Discovery::IssuerBindingError.new(
      "OAuth issuer mismatch: metadata issuer \"x\" does not equal \"y\""
    )

    error = Basecamp::Oauth.send(:as_failure_error, "https://bc5.example.test", marker)

    assert_instance_of Basecamp::Oauth::DiscoverySelectionError, error
    assert_equal "issuer_mismatch", error.reason
  end

  def test_as_failure_error_does_not_message_match_for_generic_failure
    # A generic AS-fetch OauthError whose MESSAGE merely contains "issuer
    # mismatch" must NOT be misclassified: the old substring match would have
    # called this issuer_mismatch; class-based dispatch keeps it as_fetch_failed.
    generic = Basecamp::Oauth::OauthError.new(
      "api_error", "Server error mentioning issuer mismatch in passing"
    )

    error = Basecamp::Oauth.send(:as_failure_error, "https://bc5.example.test", generic)

    assert_equal "as_fetch_failed", error.reason
  end
end
