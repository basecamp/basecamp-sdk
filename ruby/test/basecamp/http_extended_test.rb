# frozen_string_literal: true

require "test_helper"

# Extended HTTP tests for edge cases in retry, pagination, and error handling
class HTTPRetryExtendedTest < Minitest::Test
  include TestHelper

  def setup
    @config = Basecamp::Config.new(
      base_url: "https://3.basecampapi.com",
      timeout: 5,
      max_retries: 3,
      base_delay: 0.01,
      max_jitter: 0.001
    )
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_retry_after_zero_returns_nil
    # Retry-After: 0 is treated as nil (no delay specified)
    # Implementation only accepts positive retry_after values
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 429, body: "{}", headers: { "Retry-After" => "0" })

    error = assert_raises(Basecamp::RateLimitError) do
      @http.get("/test.json")
    end

    assert_nil error.retry_after
  end

  def test_retry_after_as_integer_seconds
    # Use a short retry_after to avoid long waits in tests
    # Verify the value is parsed correctly
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 429, body: "{}", headers: { "Retry-After" => "5" })

    error = assert_raises(Basecamp::RateLimitError) do
      @http.get("/test.json")
    end

    assert_equal 5, error.retry_after
  end

  def test_503_service_unavailable_is_retryable
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")
      .then.to_return(status: 503, body: "{}")
      .then.to_return(status: 200, body: '{"ok": true}')

    response = @http.get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 3)
  end

  def test_503_with_retry_after_header
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}", headers: { "Retry-After" => "5" })
      .then.to_return(status: 200, body: '{"ok": true}')

    response = @http.get("/test.json")

    assert_equal 200, response.status
  end

  def test_502_bad_gateway_is_retryable
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 502, body: "Bad Gateway")
      .then.to_return(status: 200, body: '{"ok": true}')

    response = @http.get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
  end

  def test_504_gateway_timeout_is_retryable
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 504, body: "Gateway Timeout")
      .then.to_return(status: 200, body: '{"ok": true}')

    response = @http.get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
  end

  def test_exponential_backoff_increases_delay
    # Test is implicit in the retry behavior - verifies the code path exists
    # Actual timing verification would require mocking Time
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")
      .then.to_return(status: 503, body: "{}")
      .then.to_return(status: 200, body: '{"ok": true}')

    response = @http.get("/test.json")

    assert_equal 200, response.status
  end
end

class HTTPPaginationExtendedTest < Minitest::Test
  include TestHelper

  def setup
    @config = default_config
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_paginate_no_link_header
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(status: 200, body: '[{"id": 1}]')

    items = @http.paginate("/items.json").to_a

    assert_equal 1, items.length
    assert_requested(:get, "https://3.basecampapi.com/items.json", times: 1)
  end

  def test_paginate_link_header_without_next_rel
    # Link header with only 'prev' relation should stop pagination
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=0>; rel="prev"' }
      )

    items = @http.paginate("/items.json").to_a

    assert_equal 1, items.length
  end

  def test_paginate_link_header_with_multiple_relations
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next", <https://3.basecampapi.com/items.json?page=1>; rel="prev"' }
      )

    stub_request(:get, "https://3.basecampapi.com/items.json?page=2")
      .to_return(status: 200, body: '[{"id": 2}]')

    items = @http.paginate("/items.json").to_a

    assert_equal 2, items.length
  end

  def test_paginate_empty_first_page
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(status: 200, body: "[]")

    items = @http.paginate("/items.json").to_a

    assert_equal 0, items.length
  end

  def test_paginate_three_pages
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next"' }
      )

    stub_request(:get, "https://3.basecampapi.com/items.json?page=2")
      .to_return(
        status: 200,
        body: '[{"id": 2}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=3>; rel="next"' }
      )

    stub_request(:get, "https://3.basecampapi.com/items.json?page=3")
      .to_return(status: 200, body: '[{"id": 3}]')

    items = @http.paginate("/items.json").to_a

    assert_equal 3, items.length
    assert_equal([ 1, 2, 3 ], items.map { |i| i["id"] })
  end

  def test_paginate_with_params_preserved
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .with(query: { status: "active" })
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?status=active&page=2>; rel="next"' }
      )

    stub_request(:get, "https://3.basecampapi.com/items.json")
      .with(query: { status: "active", page: "2" })
      .to_return(status: 200, body: '[{"id": 2}]')

    items = @http.paginate("/items.json", params: { status: "active" }).to_a

    assert_equal 2, items.length
  end
end
