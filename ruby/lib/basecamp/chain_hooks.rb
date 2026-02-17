# frozen_string_literal: true

module Basecamp
  # Composes multiple Hooks implementations, calling them in sequence.
  # Start events are called in order; end events are called in reverse order.
  class ChainHooks
    include Hooks

    def initialize(*hooks)
      @hooks = hooks
    end

    def on_operation_start(info)
      @hooks.each { |h| safe_call { h.on_operation_start(info) } }
    end

    def on_operation_end(info, result)
      @hooks.reverse_each { |h| safe_call { h.on_operation_end(info, result) } }
    end

    def on_request_start(info)
      @hooks.each { |h| safe_call { h.on_request_start(info) } }
    end

    def on_request_end(info, result)
      @hooks.reverse_each { |h| safe_call { h.on_request_end(info, result) } }
    end

    def on_retry(info, attempt, error, delay)
      @hooks.each { |h| safe_call { h.on_retry(info, attempt, error, delay) } }
    end

    def on_paginate(url, page)
      @hooks.each { |h| safe_call { h.on_paginate(url, page) } }
    end

    private

    def safe_call
      yield
    rescue => e
      warn "Basecamp::ChainHooks: hook raised #{e.class}: #{e.message}"
    end
  end
end
