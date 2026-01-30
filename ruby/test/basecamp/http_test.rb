# frozen_string_literal: true

require "test_helper"

class HTTPTest < Minitest::Test
  include TestHelper

  def setup
    @config = default_config
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_get_request
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: '{"result": "ok"}', headers: { "Content-Type" => "application/json" })

    response = @http.get("/test.json")

    assert_equal 200, response.status
    assert_equal({ "result" => "ok" }, response.json)
  end

  def test_get_with_params
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .with(query: { status: "active" })
      .to_return(status: 200, body: "[]", headers: { "Content-Type" => "application/json" })

    response = @http.get("/test.json", params: { status: "active" })

    assert_equal 200, response.status
  end

  def test_post_request
    stub_request(:post, "https://3.basecampapi.com/test.json")
      .with(body: { name: "Test" }.to_json)
      .to_return(status: 201, body: '{"id": 1}', headers: { "Content-Type" => "application/json" })

    response = @http.post("/test.json", body: { name: "Test" })

    assert_equal 201, response.status
    assert_equal({ "id" => 1 }, response.json)
  end

  def test_put_request
    stub_request(:put, "https://3.basecampapi.com/test/1.json")
      .to_return(status: 200, body: '{"updated": true}', headers: { "Content-Type" => "application/json" })

    response = @http.put("/test/1.json", body: { name: "Updated" })

    assert_equal 200, response.status
  end

  def test_delete_request
    stub_request(:delete, "https://3.basecampapi.com/test/1.json")
      .to_return(status: 204, body: "")

    response = @http.delete("/test/1.json")

    assert_equal 204, response.status
  end

  def test_authorization_header
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .with(headers: { "Authorization" => "Bearer test-access-token" })
      .to_return(status: 200, body: "{}")

    @http.get("/test.json")

    assert_requested(:get, "https://3.basecampapi.com/test.json",
                     headers: { "Authorization" => "Bearer test-access-token" })
  end

  def test_user_agent_header
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: "{}")

    @http.get("/test.json")

    assert_requested(:get, "https://3.basecampapi.com/test.json",
                     headers: { "User-Agent" => /basecamp-sdk-ruby/ })
  end

  def test_handles_absolute_url
    stub_request(:get, "https://other.api.com/path.json")
      .to_return(status: 200, body: "{}")

    response = @http.get("https://other.api.com/path.json")

    assert_equal 200, response.status
  end

  def test_401_raises_auth_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')

    assert_raises(Basecamp::AuthError) do
      @http.get("/test.json")
    end
  end

  def test_403_raises_forbidden_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 403, body: '{"error": "Forbidden"}')

    assert_raises(Basecamp::ForbiddenError) do
      @http.get("/test.json")
    end
  end

  def test_404_raises_not_found_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 404, body: '{"error": "Not found"}')

    assert_raises(Basecamp::NotFoundError) do
      @http.get("/test.json")
    end
  end

  def test_429_raises_rate_limit_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 429, body: "{}", headers: { "Retry-After" => "30" })

    error = assert_raises(Basecamp::RateLimitError) do
      @http.get("/test.json")
    end

    assert_equal 30, error.retry_after
    assert error.retryable?
  end

  def test_500_raises_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 500, body: '{"error": "Server error"}')

    # 5xx errors may raise APIError or NetworkError depending on Faraday error classification
    assert_raises(Basecamp::Error) do
      @http.get("/test.json")
    end
  end

  def test_response_json_parsing
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: '{"name": "Test", "count": 42}')

    response = @http.get("/test.json")
    json = response.json

    assert_equal "Test", json["name"]
    assert_equal 42, json["count"]
  end

  def test_response_success_predicate
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: "{}")

    response = @http.get("/test.json")

    assert response.success?
  end
end

class HTTPRetryTest < Minitest::Test
  include TestHelper

  def setup
    @config = Basecamp::Config.new(
      base_url: "https://3.basecampapi.com",
      timeout: 5,
      max_retries: 3,
      base_delay: 0.01, # Short delay for tests
      max_jitter: 0.001
    )
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_retries_on_5xx_for_get
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")
      .then.to_return(status: 200, body: '{"ok": true}')

    response = @http.get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
  end

  def test_does_not_retry_post_on_5xx
    stub_request(:post, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")

    # Mutations should not retry, error type depends on Faraday classification
    assert_raises(Basecamp::Error) do
      @http.post("/test.json", body: { data: "test" })
    end

    assert_requested(:post, "https://3.basecampapi.com/test.json", times: 1)
  end

  def test_respects_retry_after_header
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 429, body: "{}", headers: { "Retry-After" => "1" })

    error = assert_raises(Basecamp::RateLimitError) do
      @http.get("/test.json")
    end

    assert_equal 1, error.retry_after
  end

  def test_max_retries_exceeded
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")

    # After max retries, error type depends on Faraday classification
    assert_raises(Basecamp::Error) do
      @http.get("/test.json")
    end

    # Should have retried max_retries times
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 3)
  end
end

class HTTPPaginationTest < Minitest::Test
  include TestHelper

  def setup
    @config = default_config
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_paginate_single_page
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(status: 200, body: '[{"id": 1}, {"id": 2}]')

    items = @http.paginate("/items.json").to_a

    assert_equal 2, items.length
    assert_equal 1, items[0]["id"]
    assert_equal 2, items[1]["id"]
  end

  def test_paginate_multiple_pages
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next"' }
      )

    stub_request(:get, "https://3.basecampapi.com/items.json?page=2")
      .to_return(status: 200, body: '[{"id": 2}]')

    items = @http.paginate("/items.json").to_a

    assert_equal 2, items.length
    assert_equal 1, items[0]["id"]
    assert_equal 2, items[1]["id"]
  end

  def test_paginate_returns_enumerator
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(status: 200, body: '[{"id": 1}, {"id": 2}, {"id": 3}]')

    enum = @http.paginate("/items.json")

    assert_kind_of Enumerator, enum
  end

  def test_paginate_lazy_evaluation
    # Only stub first page - second should not be called if we only take 1
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next"' }
      )

    items = @http.paginate("/items.json").take(1).to_a

    assert_equal 1, items.length
    assert_requested(:get, "https://3.basecampapi.com/items.json", times: 1)
  end
end
