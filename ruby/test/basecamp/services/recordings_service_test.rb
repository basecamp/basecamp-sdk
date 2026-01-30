# frozen_string_literal: true

require "test_helper"

class RecordingsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_recording(id: 456, title: "Test Recording")
    {
      "id" => id,
      "title" => title,
      "status" => "active",
      "type" => "Todo",
      "created_at" => "2024-01-01T00:00:00Z"
    }
  end

  def test_list
    recordings = [ sample_recording, sample_recording(id: 457, title: "Another Recording") ]
    stub_request(:get, "https://3.basecampapi.com/12345/projects/recordings.json")
      .with(query: { type: "Todo" })
      .to_return(status: 200, body: recordings.to_json)

    result = @account.recordings.list(type: "Todo").to_a

    assert_equal 2, result.length
    assert_equal "Test Recording", result[0]["title"]
  end

  def test_list_with_filters
    recordings = [ sample_recording ]
    stub_request(:get, "https://3.basecampapi.com/12345/projects/recordings.json")
      .with(query: { type: "Message", bucket: "100", status: "archived" })
      .to_return(status: 200, body: recordings.to_json)

    result = @account.recordings.list(type: "Message", bucket: 100, status: "archived").to_a

    assert_equal 1, result.length
  end

  def test_get
    stub_get("/12345/buckets/100/recordings/456", response_body: sample_recording)

    result = @account.recordings.get(project_id: 100, recording_id: 456)

    assert_equal 456, result["id"]
    assert_equal "Test Recording", result["title"]
  end

  def test_archive
    stub_put("/12345/buckets/100/recordings/456/status/archived.json", response_body: {})

    result = @account.recordings.archive(project_id: 100, recording_id: 456)

    assert_nil result
  end

  def test_unarchive
    stub_put("/12345/buckets/100/recordings/456/status/active.json", response_body: {})

    result = @account.recordings.unarchive(project_id: 100, recording_id: 456)

    assert_nil result
  end

  def test_trash
    stub_put("/12345/buckets/100/recordings/456/status/trashed.json", response_body: {})

    result = @account.recordings.trash(project_id: 100, recording_id: 456)

    assert_nil result
  end

  def test_list_events
    events = [
      { "id" => 1, "action" => "created", "created_at" => "2024-01-01T00:00:00Z" },
      { "id" => 2, "action" => "updated", "created_at" => "2024-01-02T00:00:00Z" }
    ]
    stub_get("/12345/buckets/100/recordings/456/events.json", response_body: events)

    result = @account.recordings.list_events(project_id: 100, recording_id: 456).to_a

    assert_equal 2, result.length
    assert_equal "created", result[0]["action"]
  end

  def test_get_subscription
    subscription = { "subscribed" => true }
    stub_get("/12345/buckets/100/recordings/456/subscription.json", response_body: subscription)

    result = @account.recordings.get_subscription(project_id: 100, recording_id: 456)

    assert result["subscribed"]
  end

  def test_subscribe
    subscription = { "subscribed" => true }
    stub_post("/12345/buckets/100/recordings/456/subscription.json", response_body: subscription)

    result = @account.recordings.subscribe(project_id: 100, recording_id: 456)

    assert result["subscribed"]
  end

  def test_unsubscribe
    stub_delete("/12345/buckets/100/recordings/456/subscription.json")

    result = @account.recordings.unsubscribe(project_id: 100, recording_id: 456)

    assert_nil result
  end

  def test_set_client_visibility
    stub_put("/12345/buckets/100/recordings/456/client_visibility.json", response_body: {})

    result = @account.recordings.set_client_visibility(project_id: 100, recording_id: 456, visible: true)

    assert_nil result
  end
end
