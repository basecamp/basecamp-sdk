# frozen_string_literal: true

require "test_helper"

class AuthorizationServiceTest < Minitest::Test
  include TestHelper

  def setup
    @client = create_client
  end

  def test_get_authorization
    stub_discovery_failure
    stub_launchpad_get("/authorization.json", response_body: sample_authorization)

    auth = @client.authorization.get

    assert_equal "test@example.com", auth["identity"]["email_address"]
    assert_equal 1, auth["accounts"].length
    assert_equal 12_345, auth["accounts"].first["id"]
  end

  def test_authorization_returns_accounts
    stub_discovery_failure
    stub_launchpad_get("/authorization.json", response_body: sample_authorization)

    auth = @client.authorization.get

    accounts = auth["accounts"]
    assert_equal "Test Account", accounts.first["name"]
    assert_equal "bc3", accounts.first["product"]
  end

  def test_authorization_returns_identity
    stub_discovery_failure
    stub_launchpad_get("/authorization.json", response_body: sample_authorization)

    auth = @client.authorization.get

    identity = auth["identity"]
    assert_equal "Test", identity["first_name"]
    assert_equal "User", identity["last_name"]
  end

  def test_authorization_uses_discovered_endpoint
    stub_discovery_success
    stub_launchpad_get("/authorization.json", response_body: sample_authorization)

    auth = @client.authorization.get

    assert_equal "test@example.com", auth["identity"]["email_address"]
  end
end
