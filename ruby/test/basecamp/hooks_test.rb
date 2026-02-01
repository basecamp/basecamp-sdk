# frozen_string_literal: true

require "test_helper"
require "logger"

class HooksTest < Minitest::Test
  def test_request_info_data_class
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test", attempt: 1)

    assert_equal "GET", info.method
    assert_equal "/test", info.url
    assert_equal 1, info.attempt
  end

  def test_request_info_default_attempt
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")

    assert_equal 1, info.attempt
  end

  def test_request_result_success
    result = Basecamp::RequestResult.new(status_code: 200, duration: 0.5)

    assert result.success?
    assert_equal 200, result.status_code
    assert_equal 0.5, result.duration
  end

  def test_request_result_failure
    result = Basecamp::RequestResult.new(status_code: 500, duration: 0.1)

    assert_not result.success?
  end

  def test_request_result_with_error
    error = StandardError.new("test error")
    result = Basecamp::RequestResult.new(error: error, duration: 0.1)

    assert_not result.success?
    assert_equal error, result.error
  end

  def test_noop_hooks_does_nothing
    hooks = Basecamp::NoopHooks.new
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    result = Basecamp::RequestResult.new(status_code: 200)

    # Should not raise
    hooks.on_request_start(info)
    hooks.on_request_end(info, result)
    hooks.on_retry(info, 2, StandardError.new, 1.0)
    hooks.on_paginate("/url", 1)
  end
end

class LoggerHooksTest < Minitest::Test
  def setup
    @log_output = StringIO.new
    @logger = Logger.new(@log_output)
    @logger.level = Logger::DEBUG
    @hooks = Basecamp::LoggerHooks.new(@logger)
  end

  def test_logs_request_start
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test", attempt: 1)

    @hooks.on_request_start(info)

    assert_includes @log_output.string, "HTTP GET /test"
    assert_includes @log_output.string, "attempt 1"
  end

  def test_logs_request_end_success
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    result = Basecamp::RequestResult.new(status_code: 200, duration: 0.123)

    @hooks.on_request_end(info, result)

    assert_includes @log_output.string, "200"
    assert_includes @log_output.string, "0.123"
  end

  def test_logs_request_end_error
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    result = Basecamp::RequestResult.new(error: StandardError.new("Connection failed"))

    @hooks.on_request_end(info, result)

    assert_includes @log_output.string, "failed"
    assert_includes @log_output.string, "Connection failed"
  end

  def test_logs_retry
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    error = StandardError.new("Rate limited")

    @hooks.on_retry(info, 2, error, 1.5)

    assert_includes @log_output.string, "Retrying"
    assert_includes @log_output.string, "attempt 2"
    assert_includes @log_output.string, "1.50"
  end

  def test_logs_pagination
    @hooks.on_paginate("/items?page=2", 2)

    assert_includes @log_output.string, "page 2"
    assert_includes @log_output.string, "/items?page=2"
  end

  def test_logs_cached_response
    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    result = Basecamp::RequestResult.new(status_code: 200, duration: 0.001, from_cache: true)

    @hooks.on_request_end(info, result)

    assert_includes @log_output.string, "cached"
  end
end
