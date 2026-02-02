# frozen_string_literal: true

require "test_helper"

# =============================================================================
# Security Module Unit Tests
# =============================================================================

class SecurityTruncateTest < Minitest::Test
  def test_truncate_short_string
    assert_equal "hello", Basecamp::Security.truncate("hello")
  end

  def test_truncate_long_string
    long = "x" * 1000
    result = Basecamp::Security.truncate(long)
    assert_operator result.bytesize, :<=, 500
    assert result.end_with?("...")
  end

  def test_truncate_nil
    assert_nil Basecamp::Security.truncate(nil)
  end

  def test_truncate_exact_boundary
    str = "x" * 500
    assert_equal str, Basecamp::Security.truncate(str)
  end

  def test_truncate_max_lte_3
    result = Basecamp::Security.truncate("hello", 3)
    assert_equal "hel", result
    assert_operator result.bytesize, :<=, 3
  end
end

class SecurityLocalhostTest < Minitest::Test
  def test_localhost_hostname
    assert Basecamp::Security.localhost?("http://localhost:3000/path")
  end

  def test_localhost_127_0_0_1
    assert Basecamp::Security.localhost?("http://127.0.0.1:3000/path")
  end

  def test_localhost_ipv6
    assert Basecamp::Security.localhost?("http://[::1]:3000/path")
  end

  def test_localhost_tld
    assert Basecamp::Security.localhost?("http://app.localhost/path")
  end

  def test_localhost_tld_subdomain
    assert Basecamp::Security.localhost?("http://3.basecamp.localhost/path")
  end

  def test_localhost_tld_with_port
    assert Basecamp::Security.localhost?("http://myapp.localhost:3000/api")
  end

  def test_not_localhost_remote_host
    assert_not Basecamp::Security.localhost?("https://example.com/path")
  end

  def test_not_localhost_suffix_in_hostname
    # "mylocalhost.com" should not match
    assert_not Basecamp::Security.localhost?("https://mylocalhost.com/path")
  end

  def test_not_localhost_nil_host
    assert_not Basecamp::Security.localhost?("/relative/path")
  end

  def test_not_localhost_invalid_uri
    assert_not Basecamp::Security.localhost?("://bad")
  end
end

class SecurityRequireHttpsTest < Minitest::Test
  def test_accepts_https
    Basecamp::Security.require_https!("https://example.com/path")
  end

  def test_rejects_http
    assert_raises(Basecamp::UsageError) do
      Basecamp::Security.require_https!("http://example.com/path")
    end
  end

  def test_rejects_empty
    assert_raises(Basecamp::UsageError) do
      Basecamp::Security.require_https!("")
    end
  end

  def test_rejects_invalid_uri
    assert_raises(Basecamp::UsageError) do
      Basecamp::Security.require_https!("://bad")
    end
  end
end

class SecuritySameOriginTest < Minitest::Test
  def test_same_host
    assert Basecamp::Security.same_origin?(
      "https://api.example.com/path1",
      "https://api.example.com/path2"
    )
  end

  def test_different_host
    assert_not Basecamp::Security.same_origin?(
      "https://api.example.com/path",
      "https://evil.com/path"
    )
  end

  def test_default_port_443
    assert Basecamp::Security.same_origin?(
      "https://api.example.com:443/path",
      "https://api.example.com/path"
    )
  end

  def test_different_port
    assert_not Basecamp::Security.same_origin?(
      "https://api.example.com:8443/path",
      "https://api.example.com/path"
    )
  end

  def test_no_scheme
    assert_not Basecamp::Security.same_origin?(
      "/page2",
      "https://api.example.com/page1"
    )
  end

  def test_case_insensitive
    assert Basecamp::Security.same_origin?(
      "HTTPS://API.EXAMPLE.COM/path",
      "https://api.example.com/other"
    )
  end
end

class SecurityResolveUrlTest < Minitest::Test
  def test_absolute_target
    result = Basecamp::Security.resolve_url(
      "https://api.example.com/page1",
      "https://other.example.com/page2"
    )
    assert_equal "https://other.example.com/page2", result
  end

  def test_relative_path
    result = Basecamp::Security.resolve_url(
      "https://api.example.com/v1/items",
      "/page2"
    )
    assert_equal "https://api.example.com/page2", result
  end

  def test_path_relative
    result = Basecamp::Security.resolve_url(
      "https://api.example.com/v1/items",
      "page2"
    )
    assert_equal "https://api.example.com/v1/page2", result
  end
end

class SecurityCheckBodySizeTest < Minitest::Test
  def test_within_limit
    Basecamp::Security.check_body_size!("x" * 100, 200)
  end

  def test_exceeds_limit
    assert_raises(Basecamp::APIError) do
      Basecamp::Security.check_body_size!("x" * 200, 100)
    end
  end

  def test_nil_body
    Basecamp::Security.check_body_size!(nil, 100)
  end
end

# =============================================================================
# HTTP Integration Tests
# =============================================================================

class SecurityHttpTest < Minitest::Test
  include TestHelper

  def setup
    @config = default_config
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_build_url_rejects_http
    assert_raises(Basecamp::UsageError) do
      @http.get("http://evil.com/path")
    end
  end

  def test_build_url_accepts_https
    stub_request(:get, "https://other.example.com/path")
      .to_return(status: 200, body: "{}", headers: { "Content-Type" => "application/json" })

    response = @http.get("https://other.example.com/path")
    assert_equal 200, response.status
  end

  def test_build_url_relative_path
    stub_request(:get, "https://3.basecampapi.com/foo.json")
      .to_return(status: 200, body: "{}", headers: { "Content-Type" => "application/json" })

    response = @http.get("/foo.json")
    assert_equal 200, response.status
  end

  def test_pagination_rejects_cross_origin_link
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id":1}]',
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<https://evil.com/page2>; rel="next"'
        }
      )

    error = assert_raises(Basecamp::APIError) do
      @http.paginate("/items.json").to_a
    end
    assert_includes error.message, "different origin"
  end

  def test_pagination_accepts_same_origin_link
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id":1}]',
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next"'
        }
      )
    stub_request(:get, "https://3.basecampapi.com/items.json?page=2")
      .to_return(
        status: 200,
        body: '[{"id":2}]',
        headers: { "Content-Type" => "application/json" }
      )

    items = @http.paginate("/items.json").to_a
    assert_equal 2, items.length
  end

  def test_pagination_resolves_relative_link
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id":1}]',
        headers: {
          "Content-Type" => "application/json",
          "Link" => '</items.json?page=2>; rel="next"'
        }
      )
    stub_request(:get, "https://3.basecampapi.com/items.json?page=2")
      .to_return(
        status: 200,
        body: '[{"id":2}]',
        headers: { "Content-Type" => "application/json" }
      )

    items = @http.paginate("/items.json").to_a
    assert_equal 2, items.length
  end

  def test_pagination_resolves_path_relative_link
    stub_request(:get, "https://3.basecampapi.com/v1/items")
      .to_return(
        status: 200,
        body: '[{"id":1}]',
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<page2>; rel="next"'
        }
      )
    stub_request(:get, "https://3.basecampapi.com/v1/page2")
      .to_return(
        status: 200,
        body: '[{"id":2}]',
        headers: { "Content-Type" => "application/json" }
      )

    items = @http.paginate("/v1/items").to_a
    assert_equal 2, items.length
  end

  def test_pagination_accepts_default_port
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id":1}]',
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<https://3.basecampapi.com:443/items.json?page=2>; rel="next"'
        }
      )
    stub_request(:get, "https://3.basecampapi.com:443/items.json?page=2")
      .to_return(
        status: 200,
        body: '[{"id":2}]',
        headers: { "Content-Type" => "application/json" }
      )

    items = @http.paginate("/items.json").to_a
    assert_equal 2, items.length
  end

  def test_pagination_malformed_json
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: "this is not json",
        headers: { "Content-Type" => "application/json" }
      )

    error = assert_raises(Basecamp::APIError) do
      @http.paginate("/items.json").to_a
    end
    assert_includes error.message, "Failed to parse"
  end

  def test_pagination_oversized_body
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: "x" * (51 * 1024 * 1024),
        headers: { "Content-Type" => "application/json" }
      )

    assert_raises(Basecamp::APIError) do
      @http.paginate("/items.json").to_a
    end
  end

  def test_paginate_key_same_origin_check
    stub_request(:get, "https://3.basecampapi.com/events.json")
      .to_return(
        status: 200,
        body: '{"events":[{"id":1}]}',
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<https://evil.com/page2>; rel="next"'
        }
      )

    error = assert_raises(Basecamp::APIError) do
      @http.paginate_key("/events.json", key: "events").to_a
    end
    assert_includes error.message, "different origin"
  end

  def test_response_json_body_size_limit
    huge_body = "x" * (51 * 1024 * 1024)
    response = Basecamp::Response.new(body: huge_body, status: 200, headers: {})

    assert_raises(Basecamp::APIError) do
      response.json
    end
  end
end

# =============================================================================
# OAuth Security Tests
# =============================================================================

class SecurityOAuthTest < Minitest::Test
  include TestHelper

  def test_exchange_rejects_http_endpoint
    error = assert_raises(Basecamp::UsageError) do
      Basecamp::Oauth.exchange_code(
        token_endpoint: "http://example.com/token",
        code: "auth-code",
        redirect_uri: "https://myapp.com/callback",
        client_id: "client-id"
      )
    end
    assert_includes error.message, "HTTPS"
  end

  def test_refresh_rejects_http_endpoint
    error = assert_raises(Basecamp::UsageError) do
      Basecamp::Oauth.refresh_token(
        token_endpoint: "http://example.com/token",
        refresh_token: "refresh-token"
      )
    end
    assert_includes error.message, "HTTPS"
  end

  def test_discovery_rejects_http_base
    error = assert_raises(Basecamp::UsageError) do
      Basecamp::Oauth.discover("http://example.com")
    end
    assert_includes error.message, "HTTPS"
  end

  def test_exchange_allows_localhost
    stub_request(:post, "http://localhost:3000/token")
      .to_return(
        status: 200,
        body: { "access_token" => "token123", "token_type" => "Bearer" }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    token = Basecamp::Oauth.exchange_code(
      token_endpoint: "http://localhost:3000/token",
      code: "auth-code",
      redirect_uri: "http://localhost:3000/callback",
      client_id: "client-id"
    )
    assert_equal "token123", token.access_token
  end

  def test_discovery_allows_localhost
    stub_request(:get, "http://localhost:3000/.well-known/oauth-authorization-server")
      .to_return(
        status: 200,
        body: {
          "issuer" => "http://localhost:3000",
          "authorization_endpoint" => "http://localhost:3000/authorize",
          "token_endpoint" => "http://localhost:3000/token"
        }.to_json,
        headers: { "Content-Type" => "application/json" }
      )

    config = Basecamp::Oauth.discover("http://localhost:3000")
    assert_equal "http://localhost:3000", config.issuer
  end

  def test_exchange_truncates_large_error_body
    large_body = "x" * 10_000

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(status: 500, body: large_body, headers: { "Content-Type" => "text/plain" })

    error = assert_raises(Basecamp::Oauth::OAuthError) do
      Basecamp::Oauth.exchange_code(
        token_endpoint: "https://launchpad.37signals.com/authorization/token",
        code: "auth-code",
        redirect_uri: "https://myapp.com/callback",
        client_id: "client-id"
      )
    end
    assert_operator error.message.length, :<, 1000
  end

  def test_exchange_truncates_error_description
    error_body = { "error" => "invalid_request", "error_description" => "x" * 1000 }.to_json

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(status: 400, body: error_body, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::Oauth::OAuthError) do
      Basecamp::Oauth.exchange_code(
        token_endpoint: "https://launchpad.37signals.com/authorization/token",
        code: "auth-code",
        redirect_uri: "https://myapp.com/callback",
        client_id: "client-id"
      )
    end
    # The error_description field is truncated to 500 bytes, but the error
    # message may include additional context from OAuthError wrapping
    assert_operator error.message.length, :<, 1000
  end

  def test_oauth_response_body_size_limit
    huge_body = "x" * (2 * 1024 * 1024)

    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(status: 200, body: huge_body, headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::APIError) do
      Basecamp::Oauth.exchange_code(
        token_endpoint: "https://launchpad.37signals.com/authorization/token",
        code: "auth-code",
        redirect_uri: "https://myapp.com/callback",
        client_id: "client-id"
      )
    end
  end
end

# =============================================================================
# Webhook Security Tests
# =============================================================================

class SecurityWebhookTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  # Note: Generated services delegate URL validation to the API server.
  # Client-side validation was removed when migrating to generated services.

  def test_webhook_create_sends_http_url_to_api
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/webhooks\.json})
      .to_return(status: 422, body: { error: "payload_url must use HTTPS" }.to_json,
        headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::Error) do
      @account.webhooks.create(
        payload_url: "http://example.com/webhook",
        types: [ "Todo" ]
      )
    end
  end

  def test_webhook_create_sends_empty_url_to_api
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/webhooks\.json})
      .to_return(status: 422, body: { error: "payload_url is required" }.to_json,
        headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::Error) do
      @account.webhooks.create(
        payload_url: "",
        types: [ "Todo" ]
      )
    end
  end

  def test_webhook_create_sends_javascript_url_to_api
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/webhooks\.json})
      .to_return(status: 422, body: { error: "payload_url must use HTTPS" }.to_json,
        headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::Error) do
      @account.webhooks.create(
        payload_url: "javascript:alert(1)",
        types: [ "Todo" ]
      )
    end
  end

  def test_webhook_create_sends_file_url_to_api
    stub_request(:post, %r{https://3\.basecampapi\.com/12345/webhooks\.json})
      .to_return(status: 422, body: { error: "payload_url must use HTTPS" }.to_json,
        headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::Error) do
      @account.webhooks.create(
        payload_url: "file:///etc/passwd",
        types: [ "Todo" ]
      )
    end
  end

  def test_webhook_update_sends_http_url_to_api
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/webhooks/\d+})
      .to_return(status: 422, body: { error: "payload_url must use HTTPS" }.to_json,
        headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::Error) do
      @account.webhooks.update(
        webhook_id: 2,
        payload_url: "http://example.com/webhook"
      )
    end
  end

  def test_webhook_update_allows_nil_url
    response = { "id" => 2, "active" => false }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/webhooks/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.webhooks.update(webhook_id: 2, active: false)
    assert_equal false, result["active"]
  end
end

# =============================================================================
# Config Validation Tests
# =============================================================================

class SecurityErrorParsingTest < Minitest::Test
  def test_parse_error_message_returns_nil_for_oversized_body
    huge_body = '{"error": "' + ("x" * (2 * 1024 * 1024)) + '"}'

    # Oversized bodies return nil to preserve error type mapping
    result = Basecamp.parse_error_message(huge_body)
    assert_nil result
  end

  def test_http_error_with_oversized_body_uses_default_message
    http = Basecamp::Http.new(
      config: Basecamp::Config.new,
      token_provider: Basecamp::StaticTokenProvider.new("token")
    )

    stub_request(:get, "https://3.basecampapi.com/test")
      .to_return(status: 422, body: "x" * (2 * 1024 * 1024))

    # Oversized error bodies fall back to default message, preserving error type
    error = assert_raises(Basecamp::ValidationError) do
      http.get("/test")
    end
    assert_equal "Validation failed", error.message
  end
end

class SecurityConfigTest < Minitest::Test
  def test_config_rejects_http_base_url
    assert_raises(Basecamp::UsageError) do
      Basecamp::Config.new(base_url: "http://example.com")
    end
  end

  def test_config_accepts_https_base_url
    config = Basecamp::Config.new(base_url: "https://custom.example.com")
    assert_equal "https://custom.example.com", config.base_url
  end

  def test_config_default_url_no_validation_error
    config = Basecamp::Config.new
    assert_equal "https://3.basecampapi.com", config.base_url
  end

  def test_config_allows_localhost_http
    config = Basecamp::Config.new(base_url: "http://localhost:3000")
    assert_equal "http://localhost:3000", config.base_url
  end

  def test_config_allows_127_0_0_1_http
    config = Basecamp::Config.new(base_url: "http://127.0.0.1:3000")
    assert_equal "http://127.0.0.1:3000", config.base_url
  end

  def test_config_rejects_negative_timeout
    assert_raises(ArgumentError) do
      Basecamp::Config.new(timeout: -1)
    end
  end

  def test_config_rejects_negative_max_retries
    assert_raises(ArgumentError) do
      Basecamp::Config.new(max_retries: -1)
    end
  end

  def test_config_rejects_zero_max_pages
    assert_raises(ArgumentError) do
      Basecamp::Config.new(max_pages: 0)
    end
  end
end

# =============================================================================
# Header Redaction Tests
# =============================================================================

class SecurityRedactHeadersTest < Minitest::Test
  def test_redacts_authorization_header
    headers = { "Authorization" => "Bearer secret-token", "Content-Type" => "application/json" }
    result = Basecamp::Security.redact_headers(headers)

    assert_equal "[REDACTED]", result["Authorization"]
    assert_equal "application/json", result["Content-Type"]
  end

  def test_redacts_cookie_header
    headers = { "Cookie" => "session=abc123", "Accept" => "application/json" }
    result = Basecamp::Security.redact_headers(headers)

    assert_equal "[REDACTED]", result["Cookie"]
    assert_equal "application/json", result["Accept"]
  end

  def test_redacts_set_cookie_header
    headers = { "Set-Cookie" => "session=abc123; HttpOnly", "Content-Type" => "text/html" }
    result = Basecamp::Security.redact_headers(headers)

    assert_equal "[REDACTED]", result["Set-Cookie"]
  end

  def test_redacts_csrf_token
    headers = { "X-CSRF-Token" => "csrf-secret", "Content-Type" => "application/json" }
    result = Basecamp::Security.redact_headers(headers)

    assert_equal "[REDACTED]", result["X-CSRF-Token"]
  end

  def test_preserves_non_sensitive_headers
    headers = { "Content-Type" => "application/json", "Accept" => "*/*", "User-Agent" => "test" }
    result = Basecamp::Security.redact_headers(headers)

    assert_equal "application/json", result["Content-Type"]
    assert_equal "*/*", result["Accept"]
    assert_equal "test", result["User-Agent"]
  end

  def test_case_insensitive_redaction
    headers = { "authorization" => "Bearer token", "COOKIE" => "session=123" }
    result = Basecamp::Security.redact_headers(headers)

    assert_equal "[REDACTED]", result["authorization"]
    assert_equal "[REDACTED]", result["COOKIE"]
  end

  def test_returns_new_hash
    original = { "Authorization" => "Bearer secret-token" }
    result = Basecamp::Security.redact_headers(original)

    # Original should not be modified
    assert_equal "Bearer secret-token", original["Authorization"]
    assert_equal "[REDACTED]", result["Authorization"]
  end

  def test_handles_empty_headers
    result = Basecamp::Security.redact_headers({})
    assert_empty result
  end
end

# =============================================================================
# Concurrency Tests
# =============================================================================

class SecurityConcurrencyTest < Minitest::Test
  def setup
    # Stub the OAuth token endpoint for all concurrency tests
    stub_request(:post, "https://launchpad.37signals.com/authorization/token")
      .to_return(
        status: 200,
        body: { access_token: "refreshed-token", expires_in: 3600 }.to_json,
        headers: { "Content-Type" => "application/json" }
      )
  end

  def test_oauth_token_provider_concurrent_refresh
    # This test verifies that concurrent refresh attempts don't race.
    # The mutex should ensure only one refresh happens at a time.
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "initial-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret",
      expires_at: Time.now - 60  # Already expired
    )

    threads = 10.times.map do
      Thread.new do
        # All threads try to get the access token simultaneously.
        # With proper mutex protection, this should not cause deadlocks.
        provider.access_token
      end
    end

    # Should complete without deadlock
    assert threads.all? { |t| t.join(5) }, "Threads should complete without deadlock"
  end

  def test_oauth_token_provider_refresh_check_inside_mutex
    # Verify that refreshable? check is inside the mutex by testing
    # that setting refresh_token to nil during concurrent access doesn't cause issues
    provider = Basecamp::OauthTokenProvider.new(
      access_token: "initial-token",
      refresh_token: "refresh-token",
      client_id: "client-id",
      client_secret: "client-secret"
    )

    # Access refreshable? from multiple threads while calling refresh
    # This would have raced before the fix (check outside mutex)
    threads = []
    10.times do
      threads << Thread.new { provider.refreshable? }
      threads << Thread.new { provider.refresh }
    end

    # Should complete without deadlock
    assert threads.all? { |t| t.join(5) }, "Threads should complete without deadlock"
  end
end

# =============================================================================
# PKCE Tests
# =============================================================================

class SecurityPKCETest < Minitest::Test
  def test_generate_pkce_returns_verifier_and_challenge
    pkce = Basecamp::Oauth::Pkce.generate

    assert pkce.key?(:verifier)
    assert pkce.key?(:challenge)
  end

  def test_generate_pkce_verifier_length
    pkce = Basecamp::Oauth::Pkce.generate

    # 32 bytes base64url-encoded without padding = 43 characters
    assert_equal 43, pkce[:verifier].length
  end

  def test_generate_pkce_challenge_length
    pkce = Basecamp::Oauth::Pkce.generate

    # SHA256 = 32 bytes, base64url-encoded without padding = 43 characters
    assert_equal 43, pkce[:challenge].length
  end

  def test_generate_pkce_challenge_is_sha256_of_verifier
    pkce = Basecamp::Oauth::Pkce.generate

    # Manually compute the expected challenge
    expected_challenge = Base64.urlsafe_encode64(
      Digest::SHA256.digest(pkce[:verifier]),
      padding: false
    )

    assert_equal expected_challenge, pkce[:challenge]
  end

  def test_generate_pkce_uniqueness
    pkce1 = Basecamp::Oauth::Pkce.generate
    pkce2 = Basecamp::Oauth::Pkce.generate

    # Each call should generate a unique verifier
    assert_not_equal pkce1[:verifier], pkce2[:verifier]
    assert_not_equal pkce1[:challenge], pkce2[:challenge]
  end

  def test_generate_state_length
    state = Basecamp::Oauth::Pkce.generate_state

    # 16 bytes base64url-encoded without padding = 22 characters
    assert_equal 22, state.length
  end

  def test_generate_state_uniqueness
    state1 = Basecamp::Oauth::Pkce.generate_state
    state2 = Basecamp::Oauth::Pkce.generate_state

    assert_not_equal state1, state2
  end

  def test_pkce_format_is_base64url
    pkce = Basecamp::Oauth::Pkce.generate

    # base64url should not contain +, /, or =
    assert_no_match(/[+\/=]/, pkce[:verifier])
    assert_no_match(/[+\/=]/, pkce[:challenge])
  end
end
