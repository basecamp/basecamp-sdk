# frozen_string_literal: true

require "test_helper"

class AuthorizationServiceTest < Minitest::Test
  include TestHelper

  RESOURCE_WELL_KNOWN = "/.well-known/oauth-protected-resource"
  AS_WELL_KNOWN = "/.well-known/oauth-authorization-server"
  BC5_ISSUER = "https://bc5.example.test"

  def setup
    @client = create_client
  end

  # --- Soft fallback paths → Launchpad ---------------------------------------

  def test_get_authorization_falls_back_to_launchpad_when_resource_discovery_fails
    stub_discovery_failure # 404 on the protected-resource well-known
    stub_launchpad_get("/authorization.json", response_body: sample_authorization)

    auth = @client.authorization.get

    assert_equal "test@example.com", auth["identity"]["email_address"]
    assert_equal 1, auth["accounts"].length
    assert_equal 12_345, auth["accounts"].first["id"]
  end

  def test_get_authorization_falls_back_when_only_launchpad_advertised
    stub_discovery_success # advertises only Launchpad → no_as_advertised
    stub_launchpad_get("/authorization.json", response_body: sample_authorization)

    auth = @client.authorization.get

    assert_equal "Test Account", auth["accounts"].first["name"]
    assert_equal "bc3", auth["accounts"].first["product"]
  end

  # --- Happy path: a discovered (same-origin) issuer is used -----------------

  def test_get_authorization_uses_discovered_issuer
    # Resource metadata advertises exactly one non-Launchpad issuer (here the API
    # host itself, so the authorization.json fetch stays same-origin), and its AS
    # metadata binds by code-point.
    stub_resource_metadata(authorization_servers: [ BASE_URL, LAUNCHPAD_URL ])
    stub_as_metadata(BASE_URL)
    stub_get("/authorization.json", response_body: sample_authorization)

    auth = @client.authorization.get

    assert_equal "Test", auth["identity"]["first_name"]
    assert_not_requested :get, "#{LAUNCHPAD_URL}/authorization.json"
  end

  def test_get_authorization_uses_cross_origin_discovered_issuer
    # The real BC5 topology: the selected issuer is a DISTINCT web origin from the
    # configured API base. The authorization.json fetch is therefore cross-origin
    # and must be permitted because discovery selected AND validated that issuer.
    stub_resource_metadata(authorization_servers: [ BC5_ISSUER, LAUNCHPAD_URL ])
    stub_as_metadata(BC5_ISSUER)
    bc5_auth = stub_request(:get, "#{BC5_ISSUER}/authorization.json")
      .to_return(status: 200, body: sample_authorization.to_json,
        headers: { "Content-Type" => "application/json" })

    auth = @client.authorization.get

    assert_equal "test@example.com", auth["identity"]["email_address"]
    assert_requested(bc5_auth)
    assert_not_requested :get, "#{LAUNCHPAD_URL}/authorization.json"
  end

  # --- Hard failures after BC5 advertisement raise, never hit Launchpad ------

  def test_as_metadata_500_after_bc5_advertised_raises_with_zero_launchpad_requests
    stub_resource_metadata(authorization_servers: [ BC5_ISSUER, LAUNCHPAD_URL ])
    stub_request(:get, "#{BC5_ISSUER}#{AS_WELL_KNOWN}")
      .to_return(status: 500, body: { error: "internal_server_error" }.to_json,
        headers: { "Content-Type" => "application/json" })
    launchpad_auth = stub_launchpad_get("/authorization.json", response_body: sample_authorization)
    launchpad_as = stub_request(:get, "#{LAUNCHPAD_URL}#{AS_WELL_KNOWN}")

    error = assert_raises(Basecamp::Oauth::DiscoverySelectionError) do
      @client.authorization.get
    end
    assert_equal "as_fetch_failed", error.reason
    assert_not_requested(launchpad_auth)
    assert_not_requested(launchpad_as)
  end

  def test_issuer_mismatch_after_bc5_advertised_raises_with_zero_launchpad_requests
    stub_resource_metadata(authorization_servers: [ BC5_ISSUER, LAUNCHPAD_URL ])
    stub_request(:get, "#{BC5_ISSUER}#{AS_WELL_KNOWN}")
      .to_return(status: 200, body: {
        issuer: "https://impostor.example.com",
        authorization_endpoint: "#{BC5_ISSUER}/oauth/authorize",
        token_endpoint: "#{BC5_ISSUER}/oauth/token"
      }.to_json, headers: { "Content-Type" => "application/json" })
    launchpad_auth = stub_launchpad_get("/authorization.json", response_body: sample_authorization)
    launchpad_as = stub_request(:get, "#{LAUNCHPAD_URL}#{AS_WELL_KNOWN}")

    error = assert_raises(Basecamp::Oauth::DiscoverySelectionError) do
      @client.authorization.get
    end
    assert_equal "issuer_mismatch", error.reason
    assert_not_requested(launchpad_auth)
    assert_not_requested(launchpad_as)
  end

  private

    def stub_resource_metadata(authorization_servers:)
      body = { resource: BASE_URL, authorization_servers: authorization_servers }
      stub_request(:get, "#{BASE_URL}#{RESOURCE_WELL_KNOWN}")
        .to_return(status: 200, body: body.to_json, headers: { "Content-Type" => "application/json" })
    end

    def stub_as_metadata(issuer)
      body = {
        issuer: issuer,
        authorization_endpoint: "#{issuer}/oauth/authorize",
        token_endpoint: "#{issuer}/oauth/token"
      }
      stub_request(:get, "#{issuer}#{AS_WELL_KNOWN}")
        .to_return(status: 200, body: body.to_json, headers: { "Content-Type" => "application/json" })
    end
end
