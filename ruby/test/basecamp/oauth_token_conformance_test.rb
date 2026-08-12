# frozen_string_literal: true

require "test_helper"

# Drives the shared, data-only fixtures in conformance/oauth-token/fixtures:
# one refresh round-trip per fixture, asserting the sent resource form
# parameter and the response decode (round-trip, absent/null as unset,
# present-empty/non-string rejected). Lifecycle preservation across a stored
# credential is per-manager behavior — not modeled here.
class OAuthTokenConformanceTest < Minitest::Test
  include TestHelper

  FIXTURE_DIR = File.expand_path("../../../conformance/oauth-token/fixtures", __dir__)
  TOKEN_ENDPOINT = "https://issuer.token-fixtures.example/oauth/token"

  fixtures = Dir.glob(File.join(FIXTURE_DIR, "*.json")).sort
  raise "no fixtures found in #{FIXTURE_DIR}" if fixtures.empty?

  # UTF-8 regardless of process locale — see oauth_resource_discovery_test.rb
  fixtures.each do |path|
    fixture = JSON.parse(File.read(path, encoding: "UTF-8"))
    define_method("test_fixture_#{File.basename(path, '.json').tr('-', '_')}") do
      assert_equal "refreshToken", fixture["operation"]

      sent_form = nil
      stub_request(:post, TOKEN_ENDPOINT)
        .with { |req| sent_form = URI.decode_www_form(req.body).to_h }
        .to_return(
          status: fixture["response"].fetch("status", 200),
          body: fixture["response"].fetch("body").to_json,
          headers: { "Content-Type" => "application/json" }
        )

      resource = fixture.dig("request", "resource")
      expect = fixture.fetch("expect")

      run_refresh = lambda do
        Basecamp::Oauth.refresh_token(
          token_endpoint: TOKEN_ENDPOINT, refresh_token: "refresh-token",
          client_id: "basecamp-cli", resource: resource
        )
      end

      if expect.fetch("outcome") == "token"
        token = run_refresh.call
        assert_equal expect["resource"], token.resource if expect.key?("resource")
        assert_nil token.resource if expect["resourceAbsent"]
      else
        error = assert_raises(Basecamp::Oauth::OauthError) { run_refresh.call }
        assert_equal "api_error", error.type
      end

      assert_not_nil sent_form, "the refresh request never reached the stub"
      assert_equal expect["formResource"], sent_form["resource"] if expect.key?("formResource")
      assert_not sent_form.key?("resource") if expect["formResourceAbsent"]
    end
  end
end
