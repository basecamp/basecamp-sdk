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
    # Any other foreign origin is rejected before egress.
    error = assert_raises(Basecamp::UsageError) do
      @http.get_absolute("https://evil.example/steal")
    end
    assert_match(/origin/, error.message)
    assert_not_requested(:get, "https://evil.example/steal")
  end

  def test_launchpad_authorization_url_stays_in_lockstep
    # get_absolute scopes its cross-origin allowance to Security's constant, while
    # the (generated) AuthorizationService resolves the fallback to its own copy.
    # If a regeneration ever changes the generated literal, this catches the drift
    # before it silently breaks the legitimate Launchpad authorization call.
    assert_equal Basecamp::Security::LAUNCHPAD_AUTHORIZATION_URL,
      Basecamp::Services::AuthorizationService::LAUNCHPAD_AUTHORIZATION_URL
  end
end
