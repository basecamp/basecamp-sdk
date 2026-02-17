# frozen_string_literal: true

require "test_helper"

class AuthStrategyTest < Minitest::Test
  def test_bearer_auth_sets_authorization_header
    token_provider = Basecamp::StaticTokenProvider.new("my-token")
    auth = Basecamp::BearerAuth.new(token_provider)
    headers = {}

    auth.authenticate(headers)

    assert_equal "Bearer my-token", headers["Authorization"]
  end

  def test_bearer_auth_exposes_token_provider
    token_provider = Basecamp::StaticTokenProvider.new("my-token")
    auth = Basecamp::BearerAuth.new(token_provider)

    assert_same token_provider, auth.token_provider
  end

  def test_custom_auth_strategy
    custom_auth = CookieAuth.new("session=abc123")
    headers = {}

    custom_auth.authenticate(headers)

    assert_equal "session=abc123", headers["Cookie"]
  end

  def test_auth_strategy_module_raises_not_implemented
    auth = Object.new
    auth.extend(Basecamp::AuthStrategy)

    assert_raises(NotImplementedError) do
      auth.authenticate({})
    end
  end

  def test_client_with_access_token_backward_compatibility
    stub_request(:get, "https://3.basecampapi.com/12345/projects.json")
      .with(headers: { "Authorization" => "Bearer test-token" })
      .to_return(
        status: 200,
        body: [ { "id" => 1, "name" => "Test" } ].to_json,
        headers: { "Content-Type" => "application/json" }
      )

    client = Basecamp.client(access_token: "test-token", account_id: "12345")
    projects = client.projects.list.to_a

    assert_equal 1, projects.length
    assert_equal "Test", projects.first["name"]
  end

  def test_client_with_custom_auth_strategy
    custom_auth = CookieAuth.new("session=xyz")
    stub_request(:get, "https://3.basecampapi.com/12345/projects.json")
      .with(headers: { "Cookie" => "session=xyz" })
      .to_return(
        status: 200,
        body: [ { "id" => 1, "name" => "Test" } ].to_json,
        headers: { "Content-Type" => "application/json" }
      )

    client = Basecamp.client(auth: custom_auth, account_id: "12345")
    projects = client.projects.list.to_a

    assert_equal 1, projects.length
  end

  def test_client_raises_when_both_access_token_and_auth
    assert_raises(ArgumentError) do
      Basecamp.client(access_token: "token", auth: CookieAuth.new("session=abc"))
    end
  end

  def test_client_raises_when_neither_access_token_nor_auth
    assert_raises(ArgumentError) do
      Basecamp.client
    end
  end

  def test_client_new_raises_when_both_token_provider_and_auth_strategy
    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com")
    token_provider = Basecamp::StaticTokenProvider.new("token")
    auth_strategy = Basecamp::BearerAuth.new(token_provider)

    assert_raises(ArgumentError) do
      Basecamp::Client.new(config: config, token_provider: token_provider, auth_strategy: auth_strategy)
    end
  end

  def test_client_new_raises_when_neither_token_provider_nor_auth_strategy
    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com")

    assert_raises(ArgumentError) do
      Basecamp::Client.new(config: config)
    end
  end

  private

  # A custom auth strategy for testing
  class CookieAuth
    include Basecamp::AuthStrategy

    def initialize(cookie)
      @cookie = cookie
    end

    def authenticate(headers)
      headers["Cookie"] = @cookie
    end
  end
end
