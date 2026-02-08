# frozen_string_literal: true

require "test_helper"
require "concurrent/atomic/atomic_fixnum"

class Basecamp::Webhooks::ReceiverTest < Minitest::Test
  def fixtures_dir
    File.expand_path("../../../../spec/fixtures/webhooks", __dir__)
  end

  def fixture_body(name)
    File.read(File.join(fixtures_dir, name))
  end

  def empty_headers
    {}
  end

  def test_routes_to_exact_handler
    receiver = Basecamp::Webhooks::Receiver.new
    events = []
    receiver.on("todo_created") { |e| events << e }

    receiver.handle_request(raw_body: fixture_body("event-todo-created.json"), headers: empty_headers)

    assert_equal 1, events.size
    assert_equal "todo_created", events.first.kind
  end

  def test_routes_to_glob_handler
    receiver = Basecamp::Webhooks::Receiver.new
    events = []
    receiver.on("todo_*") { |e| events << e }

    receiver.handle_request(raw_body: fixture_body("event-todo-created.json"), headers: empty_headers)

    assert_equal 1, events.size
  end

  def test_suffix_glob_pattern
    receiver = Basecamp::Webhooks::Receiver.new
    events = []
    receiver.on("*_created") { |e| events << e }

    receiver.handle_request(raw_body: fixture_body("event-todo-created.json"), headers: empty_headers)
    assert_equal 1, events.size
  end

  def test_on_any_receives_all_events
    receiver = Basecamp::Webhooks::Receiver.new
    events = []
    receiver.on_any { |e| events << e }

    receiver.handle_request(raw_body: fixture_body("event-todo-created.json"), headers: empty_headers)
    receiver.handle_request(raw_body: fixture_body("event-message-copied.json"), headers: empty_headers)

    assert_equal 2, events.size
  end

  def test_unknown_event_kind_does_not_error
    receiver = Basecamp::Webhooks::Receiver.new
    receiver.on("todo_created") { |_e| }

    # Should not raise for unknown event kind
    event = receiver.handle_request(raw_body: fixture_body("event-unknown-future.json"), headers: empty_headers)
    assert_equal "new_thing_activated", event.kind
  end

  def test_unknown_event_routes_to_catch_all
    receiver = Basecamp::Webhooks::Receiver.new
    events = []
    receiver.on("todo_created") { |_e| }
    receiver.on_any { |e| events << e }

    receiver.handle_request(raw_body: fixture_body("event-unknown-future.json"), headers: empty_headers)
    assert_equal 1, events.size
  end

  def test_dedup_prevents_double_handling
    receiver = Basecamp::Webhooks::Receiver.new
    count = 0
    receiver.on("todo_created") { |_e| count += 1 }

    body = fixture_body("event-todo-created.json")
    receiver.handle_request(raw_body: body, headers: empty_headers)
    receiver.handle_request(raw_body: body, headers: empty_headers)

    assert_equal 1, count
  end

  def test_dedup_only_after_success
    receiver = Basecamp::Webhooks::Receiver.new
    calls = 0
    receiver.on("todo_created") do |_e|
      calls += 1
      raise "transient failure" if calls == 1
    end

    body = fixture_body("event-todo-created.json")

    # First attempt fails
    assert_raises(RuntimeError) do
      receiver.handle_request(raw_body: body, headers: empty_headers)
    end
    assert_equal 1, calls

    # Retry of same event should run handlers again (not suppressed by dedup)
    receiver.handle_request(raw_body: body, headers: empty_headers)
    assert_equal 2, calls

    # Third delivery is now a true duplicate (second succeeded)
    receiver.handle_request(raw_body: body, headers: empty_headers)
    assert_equal 2, calls
  end

  def test_concurrent_dedup_claim
    receiver = Basecamp::Webhooks::Receiver.new
    call_count = Concurrent::AtomicFixnum.new(0)
    receiver.on_any do |_e|
      call_count.increment
      sleep 0.01 # simulate slow handler
    end

    body = '{"id":42,"kind":"a","created_at":"2022-01-01T00:00:00Z","recording":{"id":1},"creator":{"id":1}}'

    threads = 2.times.map do
      Thread.new { receiver.handle_request(raw_body: body, headers: empty_headers) }
    end
    threads.each(&:join)

    assert_equal 1, call_count.value
  end

  def test_dedup_disabled
    receiver = Basecamp::Webhooks::Receiver.new(dedup_window_size: 0)
    count = 0
    receiver.on("todo_created") { |_e| count += 1 }

    body = fixture_body("event-todo-created.json")
    receiver.handle_request(raw_body: body, headers: empty_headers)
    receiver.handle_request(raw_body: body, headers: empty_headers)

    assert_equal 2, count
  end

  def test_middleware_runs_in_order
    receiver = Basecamp::Webhooks::Receiver.new
    order = []

    receiver.use { |_event, next_fn| order << 1; next_fn.call }
    receiver.use { |_event, next_fn| order << 2; next_fn.call }
    receiver.on_any { |_e| order << 3 }

    receiver.handle_request(raw_body: fixture_body("event-todo-created.json"), headers: empty_headers)

    assert_equal [ 1, 2, 3 ], order
  end

  def test_verification_with_valid_signature
    secret = "test-secret"
    body = fixture_body("event-todo-created.json")
    signature = Basecamp::Webhooks::Verify.compute_signature(payload: body, secret: secret)

    receiver = Basecamp::Webhooks::Receiver.new(secret: secret)
    events = []
    receiver.on_any { |e| events << e }

    headers = { "X-Basecamp-Signature" => signature }
    receiver.handle_request(raw_body: body, headers: headers)

    assert_equal 1, events.size
  end

  def test_verification_rejects_bad_signature
    receiver = Basecamp::Webhooks::Receiver.new(secret: "test-secret")

    headers = { "X-Basecamp-Signature" => "bad-sig" }
    assert_raises(Basecamp::Webhooks::VerificationError) do
      receiver.handle_request(raw_body: fixture_body("event-todo-created.json"), headers: headers)
    end
  end

  def test_multiple_handlers_per_kind
    receiver = Basecamp::Webhooks::Receiver.new
    results = []
    receiver.on("todo_created") { |_e| results << "a" }
    receiver.on("todo_created") { |_e| results << "b" }

    receiver.handle_request(raw_body: fixture_body("event-todo-created.json"), headers: empty_headers)
    assert_equal [ "a", "b" ], results
  end
end
