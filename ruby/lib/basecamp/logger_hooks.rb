# frozen_string_literal: true

module Basecamp
  # Hooks implementation that logs to Ruby's Logger.
  #
  # @example
  #   require "logger"
  #   logger = Logger.new($stdout)
  #   hooks = Basecamp::LoggerHooks.new(logger)
  #   client = Basecamp::Client.new(config: config, token_provider: provider, hooks: hooks)
  class LoggerHooks
    include Hooks

    # @param logger [Logger] Ruby logger instance
    # @param level [Symbol] log level (:debug, :info, :warn, :error)
    def initialize(logger, level: :debug)
      @logger = logger
      @level = level
    end

    def on_request_start(info)
      @logger.send(@level, "HTTP #{info.method} #{info.url} (attempt #{info.attempt})")
    end

    def on_request_end(info, result)
      if result.error
        @logger.send(@level, "HTTP #{info.method} #{info.url} failed: #{result.error.message}")
      else
        cache_info = result.from_cache ? " (cached)" : ""
        @logger.send(@level,
                     "HTTP #{info.method} #{info.url} -> #{result.status_code}#{cache_info} (#{format("%.3f",
                                                                                                      result.duration)}s)")
      end
    end

    def on_retry(info, attempt, error, delay)
      @logger.send(@level,
                   "Retrying #{info.method} #{info.url} (attempt #{attempt}) in #{format("%.2f",
                                                                                         delay)}s: #{error.message}")
    end

    def on_paginate(url, page)
      @logger.send(@level, "Fetching page #{page}: #{url}")
    end
  end
end
