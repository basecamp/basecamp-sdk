# frozen_string_literal: true

require "test_helper"

class ChainHooksTest < Minitest::Test
  def test_calls_start_hooks_in_order
    calls = []
    hook_a = RecordingHooks.new("A", calls)
    hook_b = RecordingHooks.new("B", calls)
    chain = Basecamp::ChainHooks.new(hook_a, hook_b)

    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    chain.on_request_start(info)

    assert_equal [ [ "A", :on_request_start ], [ "B", :on_request_start ] ], calls
  end

  def test_calls_end_hooks_in_reverse_order
    calls = []
    hook_a = RecordingHooks.new("A", calls)
    hook_b = RecordingHooks.new("B", calls)
    chain = Basecamp::ChainHooks.new(hook_a, hook_b)

    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    result = Basecamp::RequestResult.new(status_code: 200)
    chain.on_request_end(info, result)

    assert_equal [ [ "B", :on_request_end ], [ "A", :on_request_end ] ], calls
  end

  def test_calls_operation_start_in_order
    calls = []
    hook_a = RecordingHooks.new("A", calls)
    hook_b = RecordingHooks.new("B", calls)
    chain = Basecamp::ChainHooks.new(hook_a, hook_b)

    info = Basecamp::OperationInfo.new(service: "projects", operation: "list")
    chain.on_operation_start(info)

    assert_equal [ [ "A", :on_operation_start ], [ "B", :on_operation_start ] ], calls
  end

  def test_calls_operation_end_in_reverse_order
    calls = []
    hook_a = RecordingHooks.new("A", calls)
    hook_b = RecordingHooks.new("B", calls)
    chain = Basecamp::ChainHooks.new(hook_a, hook_b)

    info = Basecamp::OperationInfo.new(service: "projects", operation: "list")
    result = Basecamp::OperationResult.new(duration_ms: 100)
    chain.on_operation_end(info, result)

    assert_equal [ [ "B", :on_operation_end ], [ "A", :on_operation_end ] ], calls
  end

  def test_calls_retry_hooks_in_order
    calls = []
    hook_a = RecordingHooks.new("A", calls)
    hook_b = RecordingHooks.new("B", calls)
    chain = Basecamp::ChainHooks.new(hook_a, hook_b)

    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    chain.on_retry(info, 2, StandardError.new, 1.0)

    assert_equal [ [ "A", :on_retry ], [ "B", :on_retry ] ], calls
  end

  def test_calls_paginate_hooks_in_order
    calls = []
    hook_a = RecordingHooks.new("A", calls)
    hook_b = RecordingHooks.new("B", calls)
    chain = Basecamp::ChainHooks.new(hook_a, hook_b)

    chain.on_paginate("/url", 1)

    assert_equal [ [ "A", :on_paginate ], [ "B", :on_paginate ] ], calls
  end

  def test_swallows_exceptions_from_hooks
    bad_hook = ExplodingHooks.new
    calls = []
    good_hook = RecordingHooks.new("good", calls)
    chain = Basecamp::ChainHooks.new(bad_hook, good_hook)

    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    chain.on_request_start(info)

    # The good hook was still called despite the bad hook raising
    assert_equal [ [ "good", :on_request_start ] ], calls
  end

  def test_swallows_exceptions_on_end_hooks
    calls = []
    good_hook = RecordingHooks.new("good", calls)
    bad_hook = ExplodingHooks.new
    chain = Basecamp::ChainHooks.new(good_hook, bad_hook)

    info = Basecamp::RequestInfo.new(method: "GET", url: "/test")
    result = Basecamp::RequestResult.new(status_code: 200)
    # on_request_end calls in reverse: bad_hook first (raises), then good_hook
    chain.on_request_end(info, result)

    assert_equal [ [ "good", :on_request_end ] ], calls
  end

  private

  class RecordingHooks
    include Basecamp::Hooks

    def initialize(name, calls)
      @name = name
      @calls = calls
    end

    def on_operation_start(_info)
      @calls << [ @name, :on_operation_start ]
    end

    def on_operation_end(_info, _result)
      @calls << [ @name, :on_operation_end ]
    end

    def on_request_start(_info)
      @calls << [ @name, :on_request_start ]
    end

    def on_request_end(_info, _result)
      @calls << [ @name, :on_request_end ]
    end

    def on_retry(_info, _attempt, _error, _delay)
      @calls << [ @name, :on_retry ]
    end

    def on_paginate(_url, _page)
      @calls << [ @name, :on_paginate ]
    end
  end

  class ExplodingHooks
    include Basecamp::Hooks

    def on_request_start(_info)
      raise "boom"
    end

    def on_request_end(_info, _result)
      raise "boom"
    end

    def on_operation_start(_info)
      raise "boom"
    end

    def on_operation_end(_info, _result)
      raise "boom"
    end
  end
end
