# frozen_string_literal: true

require "test_helper"

# Drives the shared, data-only fixtures in conformance/oauth/fixtures with this
# harness's mock origins substituted for the {{...}} placeholders, so issuer /
# resource binding stays code-point-exact against the mocked hosts.
class OAuthResourceDiscoveryTest < Minitest::Test
  include TestHelper

  FIXTURE_DIR = File.expand_path("../../../conformance/oauth/fixtures", __dir__)

  # Mock origins substituted for fixture placeholders. LAUNCHPAD must be the real
  # origin because the fallback consumer targets it.
  ORIGINS = {
    "{{RESOURCE_ORIGIN}}" => "https://api.basecamp-test.example",
    "{{ISSUER_ORIGIN}}" => "https://issuer.basecamp-test.example",
    "{{LAUNCHPAD_ORIGIN}}" => "https://launchpad.37signals.com",
    "{{BC5_ISSUER}}" => "https://bc5.basecamp-test.example"
  }.freeze

  WELL_KNOWN_RESOURCE = "/.well-known/oauth-protected-resource"
  WELL_KNOWN_AS = "/.well-known/oauth-authorization-server"

  # Generate one test method per fixture file.
  Dir.glob(File.join(FIXTURE_DIR, "*.json")).sort.each do |path|
    fixture = JSON.parse(File.read(path))
    # The oversized-body scenario needs a genuine streaming transport; it is
    # exercised by a dedicated test below (WebMock delivers the body as one
    # chunk, so it can't demonstrate a bounded read).
    next if fixture.dig("hop1", "oversized") || fixture.dig("hop2", "oversized")

    define_method(:"test_fixture_#{fixture["name"].tr("-", "_")}") do
      drive_fixture(substitute(fixture))
    end
  end

  private

    def substitute(value)
      json = JSON.generate(value)
      ORIGINS.each { |placeholder, origin| json = json.gsub(placeholder, origin) }
      JSON.parse(json)
    end

    def drive_fixture(fixture)
      # Track any request to the Launchpad well-known endpoints. The orchestrator
      # itself never contacts Launchpad; hard cases must never reach here.
      stub_launchpad_well_known

      # A bracketed IPv6 origin (e.g. +http://[::1]:3000+) is driven through the
      # transport end-to-end: WebMock stubs the well-known endpoint at the literal
      # bracketed origin by exact match (only regex/pattern matching breaks on
      # brackets, not exact-string stubs), so the fixture exercises the real fetch
      # path where a regex-based origin parser would fail.
      stub_hops(fixture)
      evaluate(fixture)

      if fixture.dig("expect", "launchpadContacted") == false
        assert_not_requested :get, "#{ORIGINS["{{LAUNCHPAD_ORIGIN}}"]}#{WELL_KNOWN_RESOURCE}"
        assert_not_requested :get, "#{ORIGINS["{{LAUNCHPAD_ORIGIN}}"]}#{WELL_KNOWN_AS}"
      end
    end

    def stub_launchpad_well_known
      launchpad = ORIGINS["{{LAUNCHPAD_ORIGIN}}"]
      stub_request(:get, "#{launchpad}#{WELL_KNOWN_RESOURCE}")
        .to_return(status: 200, body: { resource: launchpad }.to_json,
          headers: { "Content-Type" => "application/json" })
      stub_request(:get, "#{launchpad}#{WELL_KNOWN_AS}")
        .to_return(status: 200, body: {
          issuer: launchpad,
          authorization_endpoint: "#{launchpad}/authorization/new",
          token_endpoint: "#{launchpad}/authorization/token"
        }.to_json, headers: { "Content-Type" => "application/json" })
    end

    def stub_hops(fixture)
      if (hop1 = fixture["hop1"])
        # Register the mock at the NORMALIZED origin: the SDK builds the well-known
        # URL from the normalized origin even when the caller's spelling differs
        # (trailing slash, explicit :443), so the raw string would not match.
        origin = Basecamp::Security.require_origin_root!(fixture["resourceOrigin"])
        stub_exchange("#{origin}#{WELL_KNOWN_RESOURCE}", hop1)
      end

      if (hop2 = fixture["hop2"])
        origin = hop2["origin"] || fixture["issuerOrigin"]
        stub_exchange("#{origin}#{WELL_KNOWN_AS}", hop2)
      end
    end

    def stub_exchange(url, exchange)
      request = stub_request(:get, url)

      if exchange["transportError"]
        request.to_timeout
      elsif exchange["redirectTo"]
        request.to_return(status: exchange["status"] || 302,
          headers: { "Location" => exchange["redirectTo"] })
      elsif exchange.key?("body")
        request.to_return(status: exchange["status"] || 200,
          body: exchange["body"].to_json,
          headers: { "Content-Type" => "application/json" })
      else
        request.to_return(status: exchange["status"] || 200, body: "")
      end
    end

    def run_operation(fixture)
      case fixture["operation"]
      when "discoverFromResource"
        Basecamp::Oauth.discover_from_resource(fixture["resourceOrigin"], expected_issuer: fixture["expectedIssuer"])
      when "discoverProtectedResource"
        Basecamp::Oauth.discover_protected_resource(fixture["resourceOrigin"])
      when "discover"
        Basecamp::Oauth.discover(fixture["issuerOrigin"])
      end
    end

    def evaluate(fixture)
      expect = fixture["expect"]
      case expect["outcome"]
      when "raise"
        assert_raise_outcome(fixture, expect)
      when "fallback"
        result = run_operation(fixture)
        assert result.fallback?, "expected a fallback result"
        assert_equal expect["fallbackReason"], result.reason
      when "selected"
        assert_selected_outcome(fixture, expect)
      end
    end

    def assert_raise_outcome(fixture, expect)
      error = assert_raises(StandardError) { run_operation(fixture) }

      if expect["error"] == "usage"
        assert_instance_of Basecamp::UsageError, error
      elsif fixture["operation"] == "discoverFromResource"
        assert_instance_of Basecamp::Oauth::DiscoverySelectionError, error
        assert_equal expect["error"], error.reason
      else
        # discover / discoverProtectedResource hard failures are api_error.
        assert_kind_of Basecamp::Oauth::OauthError, error
        assert_equal "api_error", error.type
      end

      # Cross-SDK coarse-category assertion.
      assert_equal expect["errorCategory"], error_category(error) if expect["errorCategory"]
    end

    def error_category(error)
      case error
      when Basecamp::UsageError then "usage"
      when Basecamp::Oauth::OauthError then error.type
      end
    end

    def assert_selected_outcome(fixture, expect)
      result = run_operation(fixture)
      if fixture["operation"] == "discoverFromResource"
        assert result.selected?, "expected a selected result"
        assert_equal expect["selectedIssuer"], result.issuer if expect["selectedIssuer"]
      elsif fixture["operation"] == "discover" && expect["selectedIssuer"]
        # discover returns a Config; binding success means issuer matches.
        assert_equal expect["selectedIssuer"], result.issuer
      elsif fixture["operation"] == "discoverProtectedResource" && expect["selectedIssuer"]
        # discoverProtectedResource returns ProtectedResourceMetadata; a bound
        # resource equal to the requested origin (code-point exact) is success.
        assert_equal expect["selectedIssuer"], result.resource
      end
    end
end
