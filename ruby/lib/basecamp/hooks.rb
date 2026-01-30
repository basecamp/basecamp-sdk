# frozen_string_literal: true

module Basecamp
  # Interface for observability hooks.
  # Implement this to add logging, metrics, or tracing to HTTP requests.
  #
  # @example Custom hooks with logging
  #   class LoggingHooks
  #     include Basecamp::Hooks
  #
  #     def on_request_start(info)
  #       puts "Starting #{info.method} #{info.url}"
  #     end
  #
  #     def on_request_end(info, result)
  #       puts "Completed #{info.method} #{info.url} - #{result.status_code} (#{result.duration}s)"
  #     end
  #   end
  #
  #   client = Basecamp::Client.new(config: config, token_provider: provider, hooks: LoggingHooks.new)
  module Hooks
    # Called when an HTTP request starts.
    # @param info [RequestInfo] request information
    # @return [void]
    def on_request_start(info)
      # Override in implementation
    end

    # Called when an HTTP request completes (success or failure).
    # @param info [RequestInfo] request information
    # @param result [RequestResult] result information
    # @return [void]
    def on_request_end(info, result)
      # Override in implementation
    end

    # Called when a request is retried.
    # @param info [RequestInfo] request information
    # @param attempt [Integer] the next attempt number
    # @param error [Exception] the error that triggered the retry
    # @param delay [Float] seconds until retry
    # @return [void]
    def on_retry(info, attempt, error, delay)
      # Override in implementation
    end

    # Called when pagination fetches the next page.
    # @param url [String] the next page URL
    # @param page [Integer] the page number
    # @return [void]
    def on_paginate(url, page)
      # Override in implementation
    end
  end
end
