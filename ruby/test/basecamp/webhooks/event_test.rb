# frozen_string_literal: true

require "test_helper"

class Basecamp::Webhooks::EventTest < Minitest::Test
  def fixtures_dir
    File.expand_path("../../../../spec/fixtures/webhooks", __dir__)
  end

  def load_fixture(name)
    JSON.parse(File.read(File.join(fixtures_dir, name)))
  end

  def test_parses_todo_created_event
    data = load_fixture("event-todo-created.json")
    event = Basecamp::Webhooks::Event.new(data)

    assert_equal 9007199254741001, event.id
    assert_equal "todo_created", event.kind
    assert_equal "2022-11-22T16:00:00.000Z", event.created_at
    assert_equal "Todo", event.recording["type"]
    assert_equal "Ship the feature", event.recording["title"]
    assert_equal "<div>Ship the feature by Friday</div>", event.recording["content"]
    assert_equal "Annie Bryan", event.creator["name"]
    assert_nil event.copy
  end

  def test_parses_message_copied_event
    data = load_fixture("event-message-copied.json")
    event = Basecamp::Webhooks::Event.new(data)

    assert_equal 9007199254741002, event.id
    assert_equal "message_copied", event.kind
    assert_not_nil event.copy
    assert_equal 9007199254741350, event.copy["id"]
    assert_equal 2085958500, event.copy["bucket"]["id"]
  end

  def test_parses_unknown_future_event
    data = load_fixture("event-unknown-future.json")
    event = Basecamp::Webhooks::Event.new(data)

    assert_equal "new_thing_activated", event.kind
    assert_equal "NewRecordingType", event.recording["type"]
    assert_equal "something_new", event.details["future_field"]
  end

  def test_parsed_kind
    event = Basecamp::Webhooks::Event.new({ "kind" => "todo_created" })
    result = event.parsed_kind
    assert_equal "todo", result[:type]
    assert_equal "created", result[:action]
  end

  def test_parsed_kind_compound
    event = Basecamp::Webhooks::Event.new({ "kind" => "question_answer_created" })
    result = event.parsed_kind
    assert_equal "question_answer", result[:type]
    assert_equal "created", result[:action]
  end

  def test_raw_preserves_original_hash
    data = { "id" => 1, "kind" => "test", "extra_field" => "preserved" }
    event = Basecamp::Webhooks::Event.new(data)
    assert_equal "preserved", event.raw["extra_field"]
  end
end
