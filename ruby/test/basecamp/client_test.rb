# frozen_string_literal: true

require "test_helper"

class ClientTest < Minitest::Test
  include TestHelper

  def test_creates_client_with_config
    config = default_config
    token_provider = test_token_provider

    client = Basecamp::Client.new(config: config, token_provider: token_provider)

    assert_equal config, client.config
  end

  def test_for_account_returns_account_client
    client = create_client
    account = client.for_account("12345")

    assert_instance_of Basecamp::AccountClient, account
    assert_equal "12345", account.account_id
  end

  def test_for_account_accepts_integer
    client = create_client
    account = client.for_account(12_345)

    assert_equal "12345", account.account_id
  end

  def test_for_account_raises_for_empty_id
    client = create_client

    assert_raises(ArgumentError) do
      client.for_account("")
    end
  end

  def test_for_account_raises_for_non_numeric_id
    client = create_client

    assert_raises(ArgumentError) do
      client.for_account("abc")
    end
  end

  def test_authorization_returns_service
    stub_discovery_failure
    stub_launchpad_get("/authorization.json", response_body: sample_authorization)

    client = create_client
    auth = client.authorization.get

    assert_equal "Test Account", auth["accounts"].first["name"]
  end
end

class AccountClientTest < Minitest::Test
  include TestHelper

  def test_account_id_accessible
    account = create_account_client(account_id: "99999")

    assert_equal "99999", account.account_id
  end

  def test_config_accessible
    config = default_config
    client = Basecamp::Client.new(config: config, token_provider: test_token_provider)
    account = client.for_account("12345")

    assert_equal config, account.config
  end

  def test_projects_service_accessible
    account = create_account_client

    assert_instance_of Basecamp::Services::ProjectsService, account.projects
  end

  def test_services_are_memoized
    account = create_account_client

    projects1 = account.projects
    projects2 = account.projects

    assert_same projects1, projects2
  end
end
