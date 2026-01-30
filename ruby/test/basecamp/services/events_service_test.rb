# frozen_string_literal: true

require "test_helper"

class EventsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_event(id: 1, action: "created")
    {
      "id" => id,
      "action" => action,
      "creator" => { "id" => 1, "name" => "Test User" },
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_list_events
    stub_get("/12345/buckets/100/recordings/200/events.json",
             response_body: [
               sample_event(id: 1, action: "created"),
               sample_event(id: 2, action: "updated"),
               sample_event(id: 3, action: "completed")
             ])

    events = @account.events.list(project_id: 100, recording_id: 200).to_a

    assert_equal 3, events.length
    assert_equal "created", events[0]["action"]
    assert_equal "updated", events[1]["action"]
    assert_equal "completed", events[2]["action"]
  end

  def test_list_events_shows_creator
    stub_get("/12345/buckets/100/recordings/200/events.json",
             response_body: [ sample_event ])

    events = @account.events.list(project_id: 100, recording_id: 200).to_a

    assert_equal "Test User", events[0]["creator"]["name"]
  end
end
