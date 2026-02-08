# frozen_string_literal: true

# Tests for the BoostsService (generated from OpenAPI spec)

require "test_helper"

class BoostsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_boost(id: 1)
    {
      "id" => id,
      "content" => "🎉",
      "created_at" => "2024-01-15T10:00:00Z",
      "booster" => { "id" => 100, "name" => "Jane Doe" },
      "recording" => { "id" => 200, "title" => "Some recording", "type" => "Todo" }
    }
  end

  def test_get_boost
    stub_get("/12345/boosts/42", response_body: sample_boost(id: 42))

    boost = @account.boosts.get_boost(boost_id: 42)

    assert_equal 42, boost["id"]
    assert_equal "🎉", boost["content"]
    assert_equal "Jane Doe", boost["booster"]["name"]
  end

  def test_delete_boost
    stub_delete("/12345/boosts/42")

    result = @account.boosts.delete_boost(boost_id: 42)

    assert_nil result
  end

  def test_list_recording_boosts
    stub_get("/12345/recordings/200/boosts.json",
             response_body: [ sample_boost(id: 1), sample_boost(id: 2) ])

    boosts = @account.boosts.list_recording_boosts(recording_id: 200).to_a

    assert_equal 2, boosts.length
    assert_equal 1, boosts[0]["id"]
    assert_equal 2, boosts[1]["id"]
  end

  def test_create_recording_boost
    stub_post("/12345/recordings/200/boosts.json",
              response_body: sample_boost(id: 99))

    boost = @account.boosts.create_recording_boost(recording_id: 200, content: "🔥")

    assert_equal 99, boost["id"]
  end

  def test_list_event_boosts
    stub_get("/12345/recordings/200/events/300/boosts.json",
             response_body: [ sample_boost(id: 5) ])

    boosts = @account.boosts.list_event_boosts(recording_id: 200, event_id: 300).to_a

    assert_equal 1, boosts.length
    assert_equal 5, boosts[0]["id"]
  end

  def test_create_event_boost
    stub_post("/12345/recordings/200/events/300/boosts.json",
              response_body: sample_boost(id: 77))

    boost = @account.boosts.create_event_boost(
      recording_id: 200, event_id: 300, content: "👍"
    )

    assert_equal 77, boost["id"]
  end
end
