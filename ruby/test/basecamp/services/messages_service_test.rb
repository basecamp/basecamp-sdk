# frozen_string_literal: true

# Tests for the MessagesService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - No archive(), unarchive(), trash() - use recordings.archive/unarchive/trash()
# - No client-side validation (API validates)
# - Single-resource paths without .json (get, update)

require "test_helper"

class MessagesServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def sample_message(id: 789, subject: "Test Message")
    {
      "id" => id,
      "subject" => subject,
      "content" => "<p>Message content</p>",
      "status" => "active",
      "created_at" => "2024-01-01T00:00:00Z",
      "creator" => { "id" => 1, "name" => "Test User" }
    }
  end

  def test_list
    messages = [ sample_message, sample_message(id: 790, subject: "Another Message") ]
    stub_get("/12345/buckets/100/message_boards/456/messages.json", response_body: messages)

    result = @account.messages.list(project_id: 100, board_id: 456).to_a

    assert_equal 2, result.length
    assert_equal "Test Message", result[0]["subject"]
  end

  def test_get
    # Generated service: /messages/{id} without .json
    stub_get("/12345/buckets/100/messages/789", response_body: sample_message)

    result = @account.messages.get(project_id: 100, message_id: 789)

    assert_equal 789, result["id"]
    assert_equal "Test Message", result["subject"]
  end

  def test_create
    new_message = sample_message(id: 999, subject: "New Post")
    stub_post("/12345/buckets/100/message_boards/456/messages.json", response_body: new_message)

    result = @account.messages.create(
      project_id: 100,
      board_id: 456,
      subject: "New Post",
      content: "<p>Content here</p>"
    )

    assert_equal 999, result["id"]
    assert_equal "New Post", result["subject"]
  end

  def test_create_with_category
    new_message = sample_message(id: 1000, subject: "Announcement")
    stub_post("/12345/buckets/100/message_boards/456/messages.json", response_body: new_message)

    result = @account.messages.create(
      project_id: 100,
      board_id: 456,
      subject: "Announcement",
      category_id: 1
    )

    assert_equal "Announcement", result["subject"]
  end

  def test_update
    # Generated service: /messages/{id} without .json
    updated_message = sample_message(subject: "Updated Subject")
    stub_put("/12345/buckets/100/messages/789", response_body: updated_message)

    result = @account.messages.update(project_id: 100, message_id: 789, subject: "Updated Subject")

    assert_equal "Updated Subject", result["subject"]
  end

  def test_pin
    stub_post("/12345/buckets/100/recordings/789/pin.json", response_body: {})

    result = @account.messages.pin(project_id: 100, message_id: 789)

    assert_nil result
  end

  def test_unpin
    stub_delete("/12345/buckets/100/recordings/789/pin.json")

    result = @account.messages.unpin(project_id: 100, message_id: 789)

    assert_nil result
  end

  # Note: archive(), unarchive(), trash() are on RecordingsService, not MessagesService (spec-conformant)
  # Use @account.recordings.archive(project_id:, recording_id:) instead
end
