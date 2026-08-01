# frozen_string_literal: true

require "test_helper"

class OperationHooksTest < Minitest::Test
  include TestHelper

  # -- Eager operations (with_operation) --

  def test_eager_operation_fires_hooks_immediately
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_get("/12345/projects/123", response_body: { "id" => 123, "name" => "Test" })

    account.projects.get(project_id: 123)

    op_events = events.select { |e| e[:event].to_s.start_with?("on_operation") }
    assert_equal [ :on_operation_start, :on_operation_end ], op_events.map { |e| e[:event] }
    assert_equal "projects", op_events[0][:info].service
    assert_equal "get", op_events[0][:info].operation
    assert_not_nil op_events[1][:result].duration_ms
    assert_nil op_events[1][:result].error
  end

  def test_eager_operation_reports_error_on_failure
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_request(:get, "https://3.basecampapi.com/12345/projects/999")
      .to_return(status: 404, body: { "error" => "not found" }.to_json)

    assert_raises(Basecamp::NotFoundError) { account.projects.get(project_id: 999) }

    end_event = events.find { |e| e[:event] == :on_operation_end }
    assert_not_nil end_event
    assert_kind_of Basecamp::NotFoundError, end_event[:result].error
  end

  # -- Paginated operations (wrap_paginated) --

  def test_paginated_start_hook_fires_eagerly_and_end_after_consumption
    # Page 1 is fetched eagerly at call time, so on_operation_start must fire
    # then (before the page-1 request hooks); on_operation_end fires only
    # once iteration completes.
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_get("/12345/projects.json", response_body: [ { "id" => 1 } ])

    enum = account.projects.list
    op_events = events.select { |e| e[:event].to_s.start_with?("on_operation") }
    assert_equal [ :on_operation_start ], op_events.map { |e| e[:event] }
    assert_equal :on_operation_start, events.first[:event],
      "start hook must precede the eager page-1 request hooks"

    enum.to_a
    op_events = events.select { |e| e[:event].to_s.start_with?("on_operation") }
    assert_equal [ :on_operation_start, :on_operation_end ], op_events.map { |e| e[:event] }
  end

  def test_paginated_hooks_fire_on_iteration
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_get("/12345/projects.json", response_body: [ { "id" => 1 }, { "id" => 2 } ])

    account.projects.list.to_a

    start_event = events.find { |e| e[:event] == :on_operation_start }
    end_event = events.find { |e| e[:event] == :on_operation_end }

    assert_not_nil start_event
    assert_equal "projects", start_event[:info].service
    assert_equal "list", start_event[:info].operation

    assert_not_nil end_event
    assert_nil end_event[:result].error
    assert end_event[:result].duration_ms >= 0
  end

  def test_paginated_hooks_fire_on_partial_consumption
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_get("/12345/projects.json", response_body: [ { "id" => 1 }, { "id" => 2 }, { "id" => 3 } ])

    account.projects.list.first(1)

    end_event = events.find { |e| e[:event] == :on_operation_end }
    assert_not_nil end_event, "on_operation_end should fire even on partial consumption"
    assert_nil end_event[:result].error
  end

  def test_paginated_hooks_report_page_one_error_at_call_time
    # The eager page-1 fetch surfaces errors from the list call itself,
    # before any iteration, with the end hook carrying the error.
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_request(:get, "https://3.basecampapi.com/12345/projects.json")
      .to_return(status: 404, body: { "error" => "not found" }.to_json)

    assert_raises(Basecamp::NotFoundError) { account.projects.list }

    end_event = events.find { |e| e[:event] == :on_operation_end }
    assert_not_nil end_event
    assert_kind_of Basecamp::NotFoundError, end_event[:result].error
  end

  def test_paginated_hooks_report_error_during_iteration
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    page2_url = "https://3.basecampapi.com/12345/projects.json?page=2"
    stub_request(:get, "https://3.basecampapi.com/12345/projects.json")
      .to_return(
        status: 200,
        body: [ { "id" => 1 } ].to_json,
        headers: { "Content-Type" => "application/json", "Link" => "<#{page2_url}>; rel=\"next\"" }
      )
    stub_request(:get, page2_url)
      .to_return(status: 404, body: { "error" => "not found" }.to_json)

    enum = account.projects.list
    assert_raises(Basecamp::NotFoundError) { enum.to_a }

    end_event = events.find { |e| e[:event] == :on_operation_end }
    assert_not_nil end_event
    assert_kind_of Basecamp::NotFoundError, end_event[:result].error
  end

  def test_paginated_request_hooks_fire_per_page
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    page2_url = "https://3.basecampapi.com/12345/projects.json?page=2"

    stub_request(:get, "https://3.basecampapi.com/12345/projects.json")
      .to_return(
        status: 200,
        body: [ { "id" => 1 } ].to_json,
        headers: { "Content-Type" => "application/json", "Link" => "<#{page2_url}>; rel=\"next\"" }
      )
    stub_request(:get, page2_url)
      .to_return(
        status: 200,
        body: [ { "id" => 2 } ].to_json,
        headers: { "Content-Type" => "application/json" }
      )

    account.projects.list.to_a

    # Operation hooks: exactly one start, one end
    op_events = events.select { |e| e[:event].to_s.start_with?("on_operation") }
    assert_equal [ :on_operation_start, :on_operation_end ], op_events.map { |e| e[:event] }

    # Request hooks: one per page (2 pages = 2 start + 2 end)
    req_starts = events.count { |e| e[:event] == :on_request_start }
    req_ends = events.count { |e| e[:event] == :on_request_end }
    assert_equal 2, req_starts, "should fire on_request_start for each page"
    assert_equal 2, req_ends, "should fire on_request_end for each page"
  end

  def test_paginated_start_hook_exception_does_not_break_iteration
    events = []
    hooks = ExplodingStartHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_get("/12345/projects.json", response_body: [ { "id" => 1 } ])

    # Hook exception is swallowed — iteration proceeds normally
    results = account.projects.list.to_a
    assert_equal 1, results.length

    assert_equal [ :on_operation_start, :on_operation_end ], events.map { |e| e[:event] }
    end_event = events.find { |e| e[:event] == :on_operation_end }
    assert_nil end_event[:error]
  end

  def test_paginated_duration_spans_call_to_consumption
    # Matches wrap_paginated_wrapped: with the first page fetched eagerly at
    # call time, duration measures from the call through the end of iteration.
    events = []
    hooks = TrackingHooks.new(events)
    account = create_account_client(hooks: hooks)

    stub_get("/12345/projects.json", response_body: [ { "id" => 1 } ])

    enum = account.projects.list
    sleep 0.01 # time between call and iteration counts toward the duration
    enum.to_a

    end_event = events.find { |e| e[:event] == :on_operation_end }
    assert end_event[:result].duration_ms >= 10, "duration spans call to consumption"
  end

  private

  class TrackingHooks
    include Basecamp::Hooks

    def initialize(events)
      @events = events
    end

    def on_operation_start(info)
      @events << { event: :on_operation_start, info: info }
    end

    def on_operation_end(info, result)
      @events << { event: :on_operation_end, info: info, result: result }
    end

    def on_request_start(info)
      @events << { event: :on_request_start, info: info }
    end

    def on_request_end(info, result)
      @events << { event: :on_request_end, info: info, result: result }
    end
  end

  class ExplodingStartHooks
    include Basecamp::Hooks

    def initialize(events)
      @events = events
    end

    def on_operation_start(_info)
      @events << { event: :on_operation_start }
      raise "start hook exploded"
    end

    def on_operation_end(_info, result)
      @events << { event: :on_operation_end, error: result.error }
    end
  end
end
