# frozen_string_literal: true

# Tests for the MyNotificationsService (generated from OpenAPI spec)

require "test_helper"

class MyNotificationsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get_bubble_ups
    bubble_ups = [
      {
        "id" => 2,
        "created_at" => "2026-07-21T00:01:43.009Z",
        "updated_at" => "2026-07-21T00:01:43.031Z",
        "section" => "bubbles",
        "unread_count" => 0,
        "read_at" => "2026-07-21T00:01:43.031Z",
        "title" => "We won Leto!",
        "type" => "Message",
        "bucket_name" => "The Leto Laptop"
      },
      {
        "id" => 3,
        "created_at" => "2026-07-21T00:02:00.000Z",
        "updated_at" => "2026-07-21T00:02:00.000Z",
        "section" => "bubbles",
        "unread_count" => 1,
        "title" => "Scheduled follow-up",
        "type" => "Todo",
        "bubble_up_at" => "2026-08-01T00:00:00Z"
      }
    ]

    stub_get("/12345/my/readings/bubble_ups.json", response_body: bubble_ups)

    result = @account.my_notifications.get_bubble_ups.to_a

    assert_kind_of Array, result
    assert_equal 2, result.length
    assert_equal 2, result[0]["id"]
    assert_equal "We won Leto!", result[0]["title"]
    assert_equal "Message", result[0]["type"]
    assert_equal "2026-08-01T00:00:00Z", result[1]["bubble_up_at"]
  end
end
